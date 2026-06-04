package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

const AuthModeNone = "none"

var (
	Store    *sessions.CookieStore
	AuthMode = AuthModeNone
)

func RequireAuth(c *gin.Context) {
	if AuthMode == AuthModeNone {
		c.Next()
		return
	}

	session, err := Store.Get(c.Request, "vpn-session")
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	auth, ok := session.Values["authenticated"].(bool)
	if !ok || !auth {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	c.Next()
}
