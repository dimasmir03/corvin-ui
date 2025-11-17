package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	ui "vpnpanel/internal"
	"vpnpanel/internal/db"
	"vpnpanel/internal/models"

	"github.com/gin-gonic/gin"
)

type PanelController struct{}

func NewPanelController(r *gin.RouterGroup) *PanelController {
	panelController := &PanelController{}
	panelController.Routes(r)
	return panelController
}

func (s PanelController) Routes(r *gin.RouterGroup) {
	r.GET("/", s.DashboardHandler)
	r.GET("/servers", s.ServersPage)             // список серверов
	r.GET("/servers/new", s.NewServerPage)       // форма добавления
	r.GET("/servers/edit/:id", s.EditServerPage) // форма редактирования

	r.GET("/users", s.UsersPage)       // список пользователей
	r.GET("/users/new", s.NewUserPage) // форма добавления
	r.GET("/users/edit/:id", s.EditUserPage)

	//complaints
	r.GET("/complaints", s.ComplaintsPage)

}

type DashboardData struct {
	Title          string
	ServerCount    int
	ActiveUsers    int
	TotalBandwidth string
}

// DashboardHandler renders the main dashboard page, displaying server count, active users, and total bandwidth.
// The rendered page includes a table with columns for the server count, active users, and total bandwidth.
// The page also includes links to add a new server and edit links for each server.
func (s PanelController) DashboardHandler(c *gin.Context) {
	log.Println("Dashboard handler started")
	data := DashboardData{
		Title:          "Dashboard",
		ServerCount:    3,
		ActiveUsers:    124,
		TotalBandwidth: "1.2 TB",
	}

	tmpl, err := template.ParseFS(ui.StaticFS,
		"templates/layout.html",
		"templates/dashboard.html",
	)
	if err != nil {
		log.Println("Template parse error:", err)
		c.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(c.Writer, "layout", data)
	if err != nil {
		log.Println("Template execute error:", err)
		c.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	log.Println("Dashboard rendered successfully")
}

// ServersPage renders the servers management page.
func (s PanelController) ServersPage(c *gin.Context) {
	tmpl, err := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/servers.html")
	if err != nil {
		log.Println("Template parse error:", err)
		c.Error(err)
		return
	}

	data := map[string]any{
		"Title": "Servers",
	}

	if err := tmpl.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		log.Println("Template execute error:", err)
		c.Error(err)
		return
	}
}

// NewServerPage renders the server add page template, allowing the user to create a new server in the database.
// The rendered page includes a form with fields for the server's name, IP, port, secret web path, and country.
// The page also includes a submit button to send the form data to the server for processing.
func (s PanelController) NewServerPage(c *gin.Context) {
	tmpl, _ := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/server_form.html")
	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{
		"Title":  "Add Server",
		"Action": "/api/servers/new",
	})
}

// EditServerPage renders the server edit page template, populating the form with the server data stored in the database.
// The ID of the server to be edited is passed as a URL parameter.
func (s PanelController) EditServerPage(c *gin.Context) {
	id := c.Param("id")
	var server models.Server
	db.DB.First(&server, id)

	tmpl, _ := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/server_form.html")
	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{
		"Title":  "Edit Server",
		"Action": fmt.Sprintf("/api/servers/edit/%s", id),
		"Server": server,
	})
}

// UsersPage renders the user list page template, displaying all users stored in the database.
// The rendered page includes a table with columns for the user's username, email, and password hash.
// The page also includes a link to add a new user and edit links for each user.
func (s PanelController) UsersPage(c *gin.Context) {
	tmpl, err := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/users.html")

	if err != nil {
		log.Println("Template parse error:", err)
		c.Error(err)
		return
	}

	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{
		"Title": "Users",
	})
}

// NewUserPage renders the user add page template, allowing the user to create a new user in the database.
// The rendered page includes a form with fields for the user's username, email, and password.
func (s PanelController) NewUserPage(c *gin.Context) {
	tmpl, _ := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/user_form.html")
	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{
		"Title":  "Add User",
		"Action": "/api/users/new",
	})
}

// EditUserPage renders the user edit page template, populating the form with the user data stored in the database.
// The ID of the user to be edited is passed as a URL parameter.
func (s PanelController) EditUserPage(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	db.DB.First(&user, id)

	tmpl, _ := template.ParseFS(ui.StaticFS, "templates/layout.html", "templates/user_form.html")
	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{
		"Title":  "Edit User",
		"Action": "/api/users/edit/" + id,
		"User":   user,
	})
}

// complaintsPage
func (s PanelController) ComplaintsPage(c *gin.Context) {
	tmpl, _ := template.ParseFS(
		ui.StaticFS,
		"templates/layout.html",
		"templates/complaints.html")
	tmpl.ExecuteTemplate(c.Writer, "layout", map[string]any{})
}
