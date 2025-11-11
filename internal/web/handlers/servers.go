package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"vpnpanel/internal/db"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type ServerController struct {
	Repo *repository.ServerRepo
	Tmpl *template.Template
}

func NewServerController(r *gin.RouterGroup) *ServerController {
	serverController := &ServerController{}
	serverController.Repo = repository.NewServerRepo(db.DB)
	serverController.Routes(r)
	return serverController
}

func (s ServerController) Routes(r *gin.RouterGroup) {
	r.GET("/list", s.AllServers)
	r.POST("/new", s.CreateServer)
	r.POST("/edit/:id", s.UpdateServer)
	r.POST("/delete/:id", s.DeleteServer)
	r.GET("/onlines", s.OnlineUsersServers)
}

// AllServers retrieves all servers from the database and returns them as a JSON object.
// If the retrieval fails, it returns a JSON object with an internal server error.
func (s ServerController) AllServers(c *gin.Context) {
	servers, err := s.Repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, servers)
}

// CreateServer creates a new server in the database.
// It binds the server data from the request body and creates a new server in the database.
// If the binding fails, it returns a JSON object with a bad request error.
// If the creation fails, it returns a JSON object with an internal server error.
// Otherwise, it redirects the client to the server list page.
func (s ServerController) CreateServer(c *gin.Context) {
	var server models.Server
	if err := c.Bind(&server); err != nil {
		log.Printf("Failed to bind server data: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.Repo.Create(&server); err != nil {
		log.Printf("Failed to create server: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"redirect": "/panel/servers",
	})
}

// UpdateServer updates a server by its ID.
// It returns a JSON object with a success message if the server is updated successfully,
// or an error message if the server ID is invalid or the update fails.
func (s ServerController) UpdateServer(ctx *gin.Context) {
	var server models.Server
	if err := ctx.Bind(&server); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := s.Repo.Update(&server); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Server updated successfully"})
}

// DeleteServer deletes a server by its ID.
// It returns a JSON object with a success message if the server is deleted successfully,
// or an error message if the server ID is invalid or the deletion fails.
func (s ServerController) DeleteServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	err = s.Repo.Delete(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete server"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Server deleted successfully",
	})
}

func (s ServerController) OnlineUsersServers(c *gin.Context) {
	servers, err := s.Repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error get servers": err.Error()})
		return
	}

	onlineCounts := make(map[int]int, len(servers))

	client := &http.Client{}
	for _, server := range servers {
		url := fmt.Sprintf("http://%s:%d%spanel/api/inbounds/onlines", server.IP, server.Port, server.SecretWebPath)
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error new req": err.Error()})
			return
		}
		req.Header.Add("X-API-KEY", server.APIKey)
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error do req": err.Error()})
			return
		}
		defer resp.Body.Close()
		// body, err := io.ReadAll(resp.Body)
		// if err != nil {
		// 	log.Printf("Failed to read response body: %v\n", err)
		// 	c.JSON(http.StatusInternalServerError, gin.H{"Failed to read response body": err.Error()})
		// 	return
		// }
		// // req url
		// log.Println("Request URL:", req.URL.String())

		// // req header X-API-KEY
		// log.Println("Request Header X-API-KEY:", req.Header.Get("X-API-KEY"))

		// log.Println("Response status code:", resp.StatusCode)
		// response body as string
		// log.Printf("Response body: %s\n", string(body))

		var onlineResponse struct {
			Success bool     `json:"success"`
			Msg     string   `json:"msg"`
			Obj     []string `json:"obj"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&onlineResponse); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"Failed to decode response body": err.Error()})
			return
		}
		onlineCounts[server.Id] = len(onlineResponse.Obj)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"msg":     "",
		"obj":     onlineCounts,
	})
}
