package middleware

import (
	"net/http"
	"vpnpanel/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

const AuthModeNone = "none"

var (
	Store    *sessions.CookieStore
	AuthMode = AuthModeNone
)

func RequireAuth(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	if AuthMode == AuthModeNone {
		logger.Info("http auth resolved", "component", "http_api", "operation", "auth", "request_id", requestID, "method", c.Request.Method, "path", c.FullPath(), "reason", "auth_disabled")
		c.Next()
		return
	}

	session, err := Store.Get(c.Request, "vpn-session")
	if err != nil {
		logger.Warn("http auth failed", "component", "http_api", "operation", "auth", "request_id", requestID, "method", c.Request.Method, "path", c.FullPath(), "reason", "session_error")
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	auth, ok := session.Values["authenticated"].(bool)
	if !ok || !auth {
		logger.Warn("http auth failed", "component", "http_api", "operation", "auth", "request_id", requestID, "method", c.Request.Method, "path", c.FullPath(), "reason", "not_authenticated")
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	logger.Info("http auth resolved", "component", "http_api", "operation", "auth", "request_id", requestID, "method", c.Request.Method, "path", c.FullPath(), "reason", "authenticated")
	c.Next()
}
