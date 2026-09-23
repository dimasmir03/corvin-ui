package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/config"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type integrationPublisher struct{ messages []broker.JobTask }

func (p *integrationPublisher) PublishJob(message broker.JobTask) error {
	p.messages = append(p.messages, message)
	return nil
}

type mobileIntegrationEnv struct {
	router    *gin.Engine
	mobile    *service.MobileService
	telegram  models.Telegram
	publisher *integrationPublisher
}

func newMobileIntegrationEnv(t *testing.T) mobileIntegrationEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Telegram{}, &models.NodeState{}, &models.ServerRegistry{}, &models.EndpointGroup{}, &models.UserSubscription{}, &models.VPNRoutingSettings{}, &models.MobileDevice{}, &models.MobileLoginSession{}, &models.MobileSession{}, &models.MobileUsedRefreshToken{}, &models.MobileOperation{}, &models.MobileIdempotencyRecord{}, &models.UserServerAccess{}, &models.VPNClient{}, &models.VPNProfile{}, &models.VPNProfileNode{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "mobile-user", Status: true}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	telegram := models.Telegram{TgID: 987654, Username: "mobile", Firstname: "Mobile", Lastname: "User", UserID: user.ID}
	if err = db.Create(&telegram).Error; err != nil {
		t.Fatal(err)
	}
	registry := models.ServerRegistry{ServerID: "server-nl-1", DisplayName: "NL 1", EndpointGroup: "direct", ExpectedProtocol: "vless", CountryCode: "NL", CountryName: "Netherlands", City: "Amsterdam", PublicHost: "nl.example.com", PublicPort: 443, Security: "reality", Network: "tcp", SNI: "example.com", Enabled: true, Source: "registered"}
	if err = db.Create(&registry).Error; err != nil {
		t.Fatal(err)
	}
	available := true
	state := models.NodeState{ServerID: registry.ServerID, NodeID: registry.ServerID, DisplayName: registry.DisplayName, EndpointGroup: "direct", ExpectedProtocol: "vless", ReportedProtocol: "vless", Protocol: "vless", AgentAlive: true, Status: models.ServerStatusOnline, Source: "registered", LastSeenAt: time.Now(), LastSnapshotAt: ptrTime(time.Now()), XUIAvailable: &available, Enabled: true}
	if err = db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	publisher := &integrationPublisher{}
	vpnRepo := repository.NewVpnRepo(db)
	vpn := service.NewVPNService(vpnRepo, repository.NewTelegramRepo(db), publisher)
	cfg := config.MobileConfig{Enabled: true, JWTSecret: strings.Repeat("j", 64), AccessTTL: 900, RefreshTTLHours: 24, LoginTTLMinutes: 10, TelegramBotUsername: "test_bot", PublicBaseURL: "https://panel.example.com", MinAppVersion: "1.0.0", TrustedProxies: []string{"10.0.0.0/8"}}
	mobile := service.NewMobileService(repository.NewMobileRepo(db), vpnRepo, vpn, cfg)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("request_id", "integration-request"); c.Next() })
	NewMobileController(mobile).Register(router.Group("/api/mobile/v1"))
	return mobileIntegrationEnv{router: router, mobile: mobile, telegram: telegram, publisher: publisher}
}

func TestMobileAPIIntegrationLoginBootstrapCatalogConnectAndDiagnostics(t *testing.T) {
	env := newMobileIntegrationEnv(t)
	installID := uuid.NewString()
	verifier := strings.Repeat("p", 64)
	startBody := map[string]any{"install_id": installID, "platform": "android", "app_version": "1.0.0", "device_name": "Integration phone", "code_challenge": pkceChallenge(verifier), "code_challenge_method": "S256"}
	start := mobileRequest(t, env.router, "POST", "/api/mobile/v1/auth/telegram/start", startBody, "")
	if start.Code != 201 {
		t.Fatalf("start=%d %s", start.Code, start.Body.String())
	}
	var started struct {
		LoginID          string `json:"login_id"`
		TelegramDeeplink string `json:"telegram_deeplink"`
	}
	decodeResponse(t, start, &started)
	raw := started.TelegramDeeplink[strings.LastIndex(started.TelegramDeeplink, "login_")+6:]
	if err := env.mobile.ApproveTelegramLogin(raw, env.telegram.TgID); err != nil {
		t.Fatal(err)
	}
	status := mobileRequest(t, env.router, "GET", "/api/mobile/v1/auth/telegram/status?login_id="+started.LoginID, nil, "")
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"approved"`) {
		t.Fatalf("status=%d %s", status.Code, status.Body.String())
	}
	exchange := mobileRequest(t, env.router, "POST", "/api/mobile/v1/auth/telegram/exchange", map[string]any{"login_id": started.LoginID, "code_verifier": verifier, "install_id": installID}, "")
	if exchange.Code != 200 {
		t.Fatalf("exchange=%d %s", exchange.Code, exchange.Body.String())
	}
	var tokens service.MobileTokens
	decodeResponse(t, exchange, &tokens)
	auth := "Bearer " + tokens.AccessToken
	bootstrap := mobileRequest(t, env.router, "GET", "/api/mobile/v1/bootstrap", nil, auth)
	if bootstrap.Code != 200 || !strings.Contains(bootstrap.Body.String(), "connectivity_check_url") {
		t.Fatalf("bootstrap=%d %s", bootstrap.Code, bootstrap.Body.String())
	}
	subscription := mobileRequest(t, env.router, "GET", "/api/mobile/v1/subscription", nil, auth)
	devices := mobileRequest(t, env.router, "GET", "/api/mobile/v1/devices", nil, auth)
	if subscription.Code != 200 || devices.Code != 200 || !strings.Contains(devices.Body.String(), "Integration phone") {
		t.Fatalf("subscription/devices: %d %d %s", subscription.Code, devices.Code, devices.Body.String())
	}
	catalog := mobileRequest(t, env.router, "GET", "/api/mobile/v1/servers", nil, auth)
	if catalog.Code != 200 || strings.Contains(catalog.Body.String(), "nl.example.com") || !strings.Contains(catalog.Body.String(), "server-nl-1") {
		t.Fatalf("catalog=%d %s", catalog.Code, catalog.Body.String())
	}
	identity, err := env.mobile.Authenticate(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	connectBody := map[string]any{"device_id": identity.DeviceID, "server_id": "server-nl-1", "protocol": "vless", "mode": "manual", "config_revision": "0"}
	connect := mobileRequestWithHeaders(t, env.router, "POST", "/api/mobile/v1/vpn/connect", connectBody, map[string]string{"Authorization": auth, "Idempotency-Key": key})
	if connect.Code != 202 {
		t.Fatalf("connect=%d %s", connect.Code, connect.Body.String())
	}
	repeat := mobileRequestWithHeaders(t, env.router, "POST", "/api/mobile/v1/vpn/connect", connectBody, map[string]string{"Authorization": auth, "Idempotency-Key": key})
	if repeat.Code != 202 || len(env.publisher.messages) != 1 {
		t.Fatalf("repeat=%d messages=%d %s", repeat.Code, len(env.publisher.messages), repeat.Body.String())
	}
	diagnostics := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/mobile/v1/vpn/diagnostics", nil)
	request.Header.Set("Authorization", auth)
	request.Header.Set("X-Forwarded-For", "198.51.100.25")
	request.RemoteAddr = "203.0.113.10:32100"
	env.router.ServeHTTP(diagnostics, request)
	if diagnostics.Code != 200 || !strings.Contains(diagnostics.Body.String(), "203.0.113.10") || diagnostics.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("diagnostics=%d headers=%v %s", diagnostics.Code, diagnostics.Header(), diagnostics.Body.String())
	}
	logout := mobileRequest(t, env.router, "POST", "/api/mobile/v1/auth/logout", map[string]any{"session_id": tokens.SessionID, "refresh_token": tokens.RefreshToken}, auth)
	if logout.Code != 204 {
		t.Fatalf("logout=%d %s", logout.Code, logout.Body.String())
	}
	afterLogout := mobileRequest(t, env.router, "GET", "/api/mobile/v1/bootstrap", nil, auth)
	if afterLogout.Code != 401 {
		t.Fatalf("access after logout=%d %s", afterLogout.Code, afterLogout.Body.String())
	}
}

func TestMobileAPIErrorEnvelope(t *testing.T) {
	env := newMobileIntegrationEnv(t)
	response := mobileRequest(t, env.router, "GET", "/api/mobile/v1/bootstrap", nil, "")
	if response.Code != 401 || !strings.Contains(response.Body.String(), `"code":"UNAUTHORIZED"`) || !strings.Contains(response.Body.String(), `"request_id":"integration-request"`) {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}

func mobileRequest(t *testing.T, router http.Handler, method, path string, body any, authorization string) *httptest.ResponseRecorder {
	headers := map[string]string{}
	if authorization != "" {
		headers["Authorization"] = authorization
	}
	return mobileRequestWithHeaders(t, router, method, path, body, headers)
}
func mobileRequestWithHeaders(t *testing.T, router http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	request := httptest.NewRequest(method, path, payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %s: %v", response.Body.String(), err)
	}
}
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func ptrTime(value time.Time) *time.Time { return &value }
