package handlers

import (
	"net/http"
	"vpnpanel/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var (
	Store       *sessions.CookieStore
	SessionName = "vpn-session"
	AuthKey     = "authenticated"
	DefaultUser = "admin"
	DefaultPass = "admin"
)

// --- Login Page ---
func LoginPage(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	logger.Info("login page requested", "component", "http_api", "handler", "login_page", "operation", "login", "request_id", requestID)
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
		"error": c.Query("error") == "1",
	})
}

// --- Handle Login ---
func LoginHandler(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	username := c.PostForm("username")
	password := c.PostForm("password")
	logger.Info("login requested", "component", "http_api", "handler", "login", "operation", "login", "request_id", requestID, "username", username)

	if authenticateUser(username, password) {
		session, _ := Store.Get(c.Request, SessionName)
		session.Values[AuthKey] = true
		_ = session.Save(c.Request, c.Writer)
		logger.Info("login succeeded", "component", "http_api", "handler", "login", "operation", "login", "request_id", requestID, "username", username, "reason", "credentials_valid")
		c.Redirect(http.StatusFound, "/")
		return
	}

	logger.Warn("login failed", "component", "http_api", "handler", "login", "operation", "login", "request_id", requestID, "username", username, "reason", "credentials_invalid")
	c.Redirect(http.StatusFound, "/login?error=1")
}

// --- Logout ---
func LogoutHandler(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	logger.Info("logout requested", "component", "http_api", "handler", "logout", "operation", "logout", "request_id", requestID)
	session, _ := Store.Get(c.Request, SessionName)
	session.Values[AuthKey] = false
	_ = session.Save(c.Request, c.Writer)
	c.Redirect(http.StatusFound, "/login")
}

func authenticateUser(username, password string) bool {
	// TODO позже замменить на проверку в DB
	return username == DefaultUser && password == DefaultPass
}
