package middleware

import (
	"net/http"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var Store *sessions.CookieStore

func RequireAuth(c *gin.Context) {
	session, _ := Store.Get(c.Request, "vpn-session")

	auth, ok := session.Values["authenticated"].(bool)
	if !ok || !auth {
		logger.Infof("Redirecting to login: %s", c.Request.URL.Path)
		c.Redirect(http.StatusFound, "/login")
		return
	}
	logger.Infof("Authenticated request: %s", c.Request.URL.Path)
	c.Next()
}
