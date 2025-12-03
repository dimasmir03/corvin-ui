package app

import (
	"io/fs"
	"log"
	"net/http"
	ui "vpnpanel/internal"
	"vpnpanel/internal/handlers"
	"vpnpanel/internal/middleware"

	nice "github.com/ekyoung/gin-nice-recovery"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

func (s *Server) Routes() *gin.Engine {
	r := gin.New()

	r.Use(gin.LoggerWithWriter(log.Writer()))
	r.Use(nice.Recovery(recoveryHandler))
	r.Use(cors.Default())

	initSessionStore()

	if err := mountStatic(r); err != nil {
		log.Printf("WARN: failed to mount static files: %v", err)
	}

	// public routes
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.LoginHandler)

	// protected routes
	auth := r.Group("/")
	auth.Use(middleware.RequireAuth)

	auth.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/panel")
	})

	// ==== Panel routes ====
	panel := auth.Group("/panel")
	s.PanelController.Register(panel)

	api := auth.Group("/api")

	s.ServersController.Register(api.Group("/servers"))
	s.UserController.Register(api.Group("/users"))
	s.VpnController.Register(api.Group("/vpn"))
	s.TelegramController.Register(api.Group("/telegram"))
	s.ComplaintsController.Register(api.Group("/complaints"))
	s.MediaController.Register(api.Group("/media"))

	return r
}

func mountStatic(r *gin.Engine) error {
	staticFS, err := fs.Sub(ui.StaticFS, "static")
	if err != nil {
		return err
	}

	r.StaticFS("/static", http.FS(staticFS))
	return nil
}

func initSessionStore() {
	store := sessions.NewCookieStore([]byte("super-secret-key"))
	middleware.Store = store
	handlers.Store = store
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.HTML(500, "error.tmpl", gin.H{
		"title": "Error",
		"err":   err,
	})
}

func defaultCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"http://*", "https://*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
