package app

import (
	"io/fs"
	"net/http"
	ui "vpnpanel/internal"
	"vpnpanel/internal/handlers"
	"vpnpanel/internal/middleware"

	nice "github.com/ekyoung/gin-nice-recovery"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

func Routes() *gin.Engine {

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(nice.Recovery(recoveryHandler))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://*", "http://*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 🔹 Подключаем статику из embed
	staticFS, _ := fs.Sub(ui.StaticFS, "static")
	r.StaticFS("/static", http.FS(staticFS))

	// r.StaticFS("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// init session store
	store := sessions.NewCookieStore([]byte("super-secret-key"))
	middleware.Store = store
	handlers.Store = store

	// protected routes
	g := r.Group("/")
	g.Use(middleware.RequireAuth)

	// public routes
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.LoginHandler)
	r.GET("/logout", handlers.LogoutHandler)

	g.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/panel")
	})

	// ==== Panel routes ====
	panelRoutes := g.Group("/panel")
	handlers.NewPanelController(panelRoutes)

	// подключаем контроллеры
	apiRoutes := g.Group("/api")

	// Servers routes
	serversRoutes := apiRoutes.Group("/servers")
	handlers.NewServersController(serversRoutes)

	// Users routes
	usersRoutes := apiRoutes.Group("/users")
	handlers.NewUserController(usersRoutes)

	// Auth routes
	// authRoutes := apiRoutes.Group("/auth")
	// handlers.NewAuthController(authRoutes)

	// Server routes
	// serverRoutes := apiRoutes.Group("/server")
	// handlers.NewServerController(serverRoutes)

	// VPN routes
	vpnRoutes := apiRoutes.Group("/vpn")
	handlers.NewVpnController(vpnRoutes)

	// Telegram routes
	telegramRoutes := apiRoutes.Group("/telegram")
	handlers.NewTelegramController(telegramRoutes)

	//Complaints
	complaintsRoutes := apiRoutes.Group("/complaints")
	handlers.NewComplaintsController(complaintsRoutes)

	return r
}

func recoveryHandler(c *gin.Context, err interface{}) {
	c.HTML(500, "error.tmpl", gin.H{
		"title": "Error",
		"err":   err,
	})
}
