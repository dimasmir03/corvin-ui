package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var Store = sessions.NewCookieStore([]byte("super-secret-key"))

// --- Login Page ---
func LoginPage(c *gin.Context) {
	tmpl, err := template.New("login.html").ParseFiles(
		filepath.Join("internal", "templates", "login.html"),
	)
	if err != nil {
		c.Error(err)
		return
	}

	err = tmpl.Execute(c.Writer, nil)
	if err != nil {
		c.Error(err)
		return
	}
}

// --- Handle Login ---
func LoginHandler(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// Простая проверка — позже заменим на БД
	if username == "admin" && password == "admin" {
		session, _ := Store.Get(c.Request, "vpn-session")
		session.Values["authenticated"] = true
		session.Save(c.Request, c.Writer)
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, "/login?error=1")
}

// --- Logout ---
func LogoutHandler(c *gin.Context) {
	session, _ := Store.Get(c.Request, "vpn-session")
	session.Values["authenticated"] = false
	session.Save(c.Request, c.Writer)
	c.Redirect(http.StatusFound, "/login")
}
