package handlers

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MobileController struct {
	service *service.MobileService
	limiter *mobileRateLimiter
	trusted []*net.IPNet
}

func NewMobileController(mobileService *service.MobileService) *MobileController {
	h := &MobileController{service: mobileService, limiter: newMobileRateLimiter()}
	if mobileService != nil {
		for _, raw := range mobileService.Config().TrustedProxies {
			if _, network, err := net.ParseCIDR(raw); err == nil {
				h.trusted = append(h.trusted, network)
			}
		}
	}
	return h
}

func (h *MobileController) Register(r *gin.RouterGroup) {
	r.GET("/connectivity-check", func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Status(http.StatusNoContent) })
	r.POST("/auth/telegram/start", h.rate("auth_start", 10, time.Minute), h.StartTelegram)
	r.GET("/auth/telegram/status", h.rate("auth_status", 60, time.Minute), h.TelegramStatus)
	r.POST("/auth/telegram/exchange", h.rate("auth_exchange", 10, time.Minute), h.ExchangeTelegram)
	r.POST("/auth/refresh", h.rate("auth_refresh", 30, time.Minute), h.Refresh)
	auth := r.Group("")
	auth.Use(h.authenticate)
	auth.POST("/auth/logout", h.Logout)
	auth.GET("/bootstrap", h.Bootstrap)
	auth.GET("/servers", h.Servers)
	auth.GET("/subscription", h.Subscription)
	auth.GET("/devices", h.Devices)
	auth.DELETE("/devices/:device_id", h.RevokeDevice)
	auth.POST("/vpn/connect", h.rate("vpn_connect", 30, time.Minute), h.Connect)
	auth.GET("/operations/:operation_id", h.Operation)
	auth.GET("/vpn/diagnostics", h.rate("diagnostics", 60, time.Minute), h.Diagnostics)
}

func (h *MobileController) StartTelegram(c *gin.Context) {
	var req struct {
		InstallID           string `json:"install_id" binding:"required"`
		Platform            string `json:"platform" binding:"required"`
		AppVersion          string `json:"app_version" binding:"required"`
		DeviceName          string `json:"device_name" binding:"required"`
		CodeChallenge       string `json:"code_challenge" binding:"required"`
		CodeChallengeMethod string `json:"code_challenge_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
		return
	}
	result, err := h.service.StartTelegramLogin(service.MobileStartInput{InstallID: req.InstallID, Platform: req.Platform, AppVersion: req.AppVersion, DeviceName: req.DeviceName, CodeChallenge: req.CodeChallenge, CodeChallengeMethod: req.CodeChallengeMethod})
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
func (h *MobileController) TelegramStatus(c *gin.Context) {
	status, err := h.service.TelegramLoginStatus(c.Query("login_id"))
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}
func (h *MobileController) ExchangeTelegram(c *gin.Context) {
	var req struct {
		LoginID      string `json:"login_id" binding:"required"`
		CodeVerifier string `json:"code_verifier" binding:"required"`
		InstallID    string `json:"install_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, 400, "INVALID_REQUEST", err.Error(), nil)
		return
	}
	tokens, err := h.service.ExchangeTelegramLogin(req.LoginID, req.CodeVerifier, req.InstallID)
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, tokens)
}
func (h *MobileController) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
		InstallID    string `json:"install_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, 400, "INVALID_REQUEST", err.Error(), nil)
		return
	}
	tokens, err := h.service.Refresh(req.RefreshToken, req.InstallID)
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, tokens)
}
func (h *MobileController) Logout(c *gin.Context) {
	identity := mobileIdentity(c)
	var req struct {
		SessionID    string `json:"session_id" binding:"required"`
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, 400, "INVALID_REQUEST", err.Error(), nil)
		return
	}
	if req.SessionID != identity.SessionID {
		h.fail(c, 401, "UNAUTHORIZED", "session does not match access token", nil)
		return
	}
	if err := h.service.Logout(identity, req.RefreshToken); err != nil {
		h.handle(c, err)
		return
	}
	c.Status(204)
}
func (h *MobileController) Bootstrap(c *gin.Context) {
	result, err := h.service.Bootstrap(mobileIdentity(c))
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *MobileController) Servers(c *gin.Context) {
	version, servers, err := h.service.Servers(mobileIdentity(c).UserID)
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, gin.H{"catalog_version": version, "servers": servers})
}
func (h *MobileController) Subscription(c *gin.Context) {
	result, err := h.service.Subscription(mobileIdentity(c).UserID)
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *MobileController) Devices(c *gin.Context) {
	result, err := h.service.Devices(mobileIdentity(c))
	if err != nil {
		h.handle(c, err)
		return
	}
	c.JSON(200, gin.H{"devices": result})
}
func (h *MobileController) RevokeDevice(c *gin.Context) {
	if err := h.service.RevokeDevice(mobileIdentity(c), c.Param("device_id")); err != nil {
		h.handle(c, err)
		return
	}
	c.Status(204)
}
func (h *MobileController) Connect(c *gin.Context) {
	var req service.MobileConnectInput
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, 400, "INVALID_REQUEST", err.Error(), nil)
		return
	}
	view, status, err := h.service.Connect(mobileIdentity(c), c.GetHeader("Idempotency-Key"), req)
	if err != nil {
		h.handle(c, err)
		return
	}
	if status == 200 && view.Result != nil {
		c.Header("Cache-Control", "no-store")
		c.JSON(200, view.Result)
		return
	}
	c.JSON(202, gin.H{"status": "provisioning", "operation_id": view.OperationID, "retry_after_seconds": view.RetryAfterSeconds})
}
func (h *MobileController) Operation(c *gin.Context) {
	view, err := h.service.Operation(mobileIdentity(c), c.Param("operation_id"))
	if err != nil {
		h.handle(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, view)
}
func (h *MobileController) Diagnostics(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"observed_ip": h.observedIP(c), "country_code": nil, "country_name": nil, "city": nil})
}

func (h *MobileController) authenticate(c *gin.Context) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		h.fail(c, 401, "UNAUTHORIZED", "bearer token is required", nil)
		c.Abort()
		return
	}
	identity, err := h.service.Authenticate(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		h.handle(c, err)
		c.Abort()
		return
	}
	c.Set("mobile_identity", identity)
	c.Next()
}
func mobileIdentity(c *gin.Context) service.MobileIdentity {
	value, _ := c.Get("mobile_identity")
	identity, _ := value.(service.MobileIdentity)
	return identity
}
func (h *MobileController) handle(c *gin.Context, err error) {
	var mobile *service.MobileError
	if errors.As(err, &mobile) {
		status := http.StatusBadRequest
		switch mobile.Code {
		case "UNAUTHORIZED", "TOKEN_REUSED":
			status = 401
		case "USER_BLOCKED", "SUBSCRIPTION_INACTIVE", "DEVICE_LIMIT_REACHED", "DEVICE_REVOKED":
			status = 403
		case "ROUTE_UNAVAILABLE", "PROTOCOL_UNAVAILABLE":
			status = 409
		case "RATE_LIMITED":
			status = 429
		}
		h.fail(c, status, mobile.Code, mobile.Message, mobile.Details)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.fail(c, 404, "NOT_FOUND", "resource not found", nil)
		return
	}
	h.fail(c, 500, "INTERNAL", "internal server error", nil)
}
func (h *MobileController) fail(c *gin.Context, status int, code, message string, details map[string]any) {
	requestID, _ := c.Get("request_id")
	body := gin.H{"code": code, "message": message}
	if details != nil {
		body["details"] = details
	}
	c.JSON(status, gin.H{"error": body, "request_id": requestID})
}

func (h *MobileController) observedIP(c *gin.Context) string {
	remote, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		remote = c.Request.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(remote))
	trusted := false
	for _, network := range h.trusted {
		if peer != nil && network.Contains(peer) {
			trusted = true
			break
		}
	}
	if trusted {
		for _, raw := range strings.Split(c.GetHeader("X-Forwarded-For"), ",") {
			if ip := net.ParseIP(strings.TrimSpace(raw)); ip != nil {
				return ip.String()
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(c.GetHeader("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	if peer != nil {
		return peer.String()
	}
	return remote
}

type mobileRateLimiter struct {
	mu      sync.Mutex
	entries map[string]mobileRateEntry
}
type mobileRateEntry struct {
	started time.Time
	count   int
}

func newMobileRateLimiter() *mobileRateLimiter {
	return &mobileRateLimiter{entries: map[string]mobileRateEntry{}}
}
func (h *MobileController) rate(scope string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := scope + ":" + h.observedIP(c)
		now := time.Now()
		h.limiter.mu.Lock()
		entry := h.limiter.entries[key]
		if entry.started.IsZero() || now.Sub(entry.started) >= window {
			entry = mobileRateEntry{started: now}
		}
		entry.count++
		h.limiter.entries[key] = entry
		blocked := entry.count > limit
		h.limiter.mu.Unlock()
		if blocked {
			h.fail(c, 429, "RATE_LIMITED", "too many requests", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
