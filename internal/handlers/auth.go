package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var (
	Store       = sessions.NewCookieStore([]byte("super-secret-key"))
	SessionName = "vpn-session"
	AuthKey     = "authenticated"
	DefaultUser = "admin"
	DefaultPass = "admin"
)

// --- Login Page ---
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
		"error": c.Query("error") == "1",
	})
}

// --- Handle Login ---
func LoginHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if authenticateUser(username, password) {
		session, _ := Store.Get(c.Request, SessionName)
		session.Values[AuthKey] = true
		_ = session.Save(c.Request, c.Writer)
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, "/login?error=1")
}

// --- Logout ---
func LogoutHandler(c *gin.Context) {
	session, _ := Store.Get(c.Request, SessionName)
	session.Values[AuthKey] = false
	_ = session.Save(c.Request, c.Writer)
	c.Redirect(http.StatusFound, "/login")
}

func authenticateUser(username, password string) bool {
	// TODO позже замменить на проверку в DB
	return username == DefaultUser && password == DefaultPass
}
