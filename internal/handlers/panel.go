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

func NewPanelController() *PanelController {
	return &PanelController{}
}

func (s PanelController) Register(r *gin.RouterGroup) {
	r.GET("/", s.DashboardHandler)
	r.GET("/servers", s.ServersPage)
	r.GET("/servers/new", s.NewServerPage)
	r.GET("/servers/edit/:id", s.EditServerPage)

	r.GET("/users", s.UsersPage)
	r.GET("/users/new", s.NewUserPage)
	r.GET("/users/edit/:id", s.EditUserPage)

	r.GET("/complaints", s.ComplaintsPage)
}

func renderTemplate(c *gin.Context, files []string, data any) {
	tmpl, err := template.ParseFS(ui.StaticFS, files...)
	if err != nil {
		log.Println("Template parse error:", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(c.Writer, "layout", data)
	if err != nil {
		log.Println("Template execute error:", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
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
	data := DashboardData{
		Title:          "Dashboard",
		ServerCount:    3,
		ActiveUsers:    124,
		TotalBandwidth: "1.2 TB",
	}

	renderTemplate(c, []string{"templates/layout.html", "templates/dashboard.html"}, data)
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
	if err := db.DB.First(&server, id).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	renderTemplate(c, []string{"template/layout.html", "templates/server_form.html"}, map[string]any{
		"Title":  "Edit Server",
		"Action": fmt.Sprintf("/api/servers/edit/%s", id),
		"Server": server,
	})
}

// UsersPage renders the user list page template, displaying all users stored in the database.
// The rendered page includes a table with columns for the user's username, email, and password hash.
// The page also includes a link to add a new user and edit links for each user.
func (s PanelController) UsersPage(c *gin.Context) {
	renderTemplate(c, []string{"templates/layout.html", "templates/users.html"}, map[string]any{
		"Title": "Users",
	})
}

// NewUserPage renders the user add page template, allowing the user to create a new user in the database.
// The rendered page includes a form with fields for the user's username, email, and password.
func (s PanelController) NewUserPage(c *gin.Context) {
	renderTemplate(c, []string{"templates/layout.html", "templates/user_form.html"}, map[string]any{
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

	if user.ID == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	renderTemplate(c, []string{
		"templates/layout.html",
		"templates/user_form.html",
	},
		map[string]any{
			"Title":  "Edit User",
			"Action": "/api/users/edit/" + id,
			"User":   user,
		})
}

// complaintsPage
func (s PanelController) ComplaintsPage(c *gin.Context) {
	renderTemplate(c,
		[]string{
			"templates/layout.html",
			"templates/complaints.html",
		},
		map[string]any{
			"Title": "Complaints",
		},
	)
}
