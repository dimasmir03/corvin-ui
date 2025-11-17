package handlers

import (
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

type ServersController struct {
	Repo *repository.ServerRepo
	Tmpl *template.Template
}

func NewServersController(r *gin.RouterGroup) *ServersController {
	serverController := &ServersController{}
	serverController.Repo = repository.NewServerRepo(db.DB)
	serverController.Routes(r)
	return serverController
}

func (s ServersController) Routes(r *gin.RouterGroup) {
	r.GET("/list", s.AllServers)
	r.POST("/create", s.CreateServer)
	r.POST("/:id/status", s.GetServerStatus)
	r.GET("/:id", s.GetServer)
	r.POST("/:id/edit", s.UpdateServer)
	r.POST("/:id/delete", s.DeleteServer)
	r.GET("/onlines", s.OnlineUsersServers)
	r.GET("/online_history", s.OnlineHistory)
}

// AllServers retrieves all servers from the database and returns them as a JSON object.
// If the retrieval fails, it returns a JSON object with an internal server error.
func (s ServersController) AllServers(c *gin.Context) {
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
func (s ServersController) CreateServer(c *gin.Context) {
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

// GetServerStatus
func (s ServersController) GetServerStatus(c *gin.Context) {
	// id, err := strconv.Atoi(c.Param("id"))
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
	// 	return
	// }
	// status, err := s.Repo.GetStatus(id)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server status"})
	// 	return
	// }
	// c.JSON(http.StatusOK, status)

	
}

//GetServer
func (s ServersController) GetServer(c *gin.Context) {
	// id, err := strconv.Atoi(c.Param("id"))
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
	// 	return
	// }
	// server, err := s.Repo.Get(id)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server"})
	// 	return
	// }
	// c.JSON(http.StatusOK, server)
}


// UpdateServer updates a server by its ID.
// It returns a JSON object with a success message if the server is updated successfully,
// or an error message if the server ID is invalid or the update fails.
func (s ServersController) UpdateServer(ctx *gin.Context) {
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
func (s ServersController) DeleteServer(ctx *gin.Context) {
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

func (s ServersController) OnlineUsersServers(c *gin.Context) {
	// servers, err := s.Repo.GetAll()
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error get servers": err.Error()})
	// 	return
	// }

	servers, total, err := s.Repo.GetAllWithLastStat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// logs response
	fmt.Printf("[INFO] Total online: %d\n", total)
	for _, s := range servers {
		fmt.Printf("[INFO] online servers:  %s, %d\n", s.Name, int(s.LastStat.Online))
	}

	// if err != nil {
	// 	http.Error(w, "failed to load servers", 500)
	// 	return
	// }
	// json.NewEncoder(w).Encode(servers)

	// c.JSON(http.StatusOK, gin.H{
	// 	"total_online": total,
	// 	"servers":      servers,
	// })

	type OnlineResponse struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			TotalOnline int             `json:"total_online"`
			Servers     []models.Server `json:"servers"`
		} `json:"obj"`
	}

	onlineResponse := OnlineResponse{
		Success: true,
		Msg:     "",
		Obj: struct {
			TotalOnline int             `json:"total_online"`
			Servers     []models.Server `json:"servers"`
		}{TotalOnline: total, Servers: servers},
	}

	c.JSON(http.StatusOK, onlineResponse)

}

// OnlineHistory
func (s ServersController) OnlineHistory(c *gin.Context) {
	history, err := s.Repo.GetOnlineHistory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, history)
}
