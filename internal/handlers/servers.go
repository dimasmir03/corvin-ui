package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type ServersController struct {
	Repo *repository.ServerRepo
}

func NewServersController(repo *repository.ServerRepo) *ServersController {
	return &ServersController{Repo: repo}
}

type ServerRequest struct {
	ID            int    `json:"id" form:"id"`
	Name          string `json:"name" form:"name"`
	IP            string `json:"ip" form:"ip"`
	Port          uint16 `json:"port" form:"port"`
	SecretWebPath string `json:"secretWebPath" form:"secretWebPath"`
	ApiKey        string `json:"apiKey" form:"apiKey"`
	APIKey        string `json:"APIKey" form:"APIKey"`
	Country       string `json:"country" form:"country"`
	Status        string `json:"status" form:"status"`
	Type          string `json:"type" form:"type"`
}

type ServerResponse struct {
	ID        int                `json:"id"`
	Name      string             `json:"name"`
	IP        string             `json:"ip"`
	Port      uint16             `json:"port"`
	Country   string             `json:"country"`
	Status    string             `json:"status"`
	Type      string             `json:"type"`
	APIKeySet bool               `json:"api_key_set"`
	LastStat  *models.ServerStat `json:"lastStat,omitempty"`
}

func (r ServerRequest) toModel() models.Server {
	apiKey := r.ApiKey
	if apiKey == "" {
		apiKey = r.APIKey
	}

	return models.Server{
		Id:            r.ID,
		Name:          r.Name,
		IP:            r.IP,
		Port:          r.Port,
		SecretWebPath: r.SecretWebPath,
		ApiKey:        apiKey,
		Country:       r.Country,
		Status:        r.Status,
		Type:          r.Type,
	}
}

func newServerResponse(server models.Server) ServerResponse {
	return ServerResponse{
		ID:        server.Id,
		Name:      server.Name,
		IP:        server.IP,
		Port:      server.Port,
		Country:   server.Country,
		Status:    server.Status,
		Type:      server.Type,
		APIKeySet: server.ApiKey != "",
		LastStat:  server.LastStat,
	}
}

func newServerResponses(servers []models.Server) []ServerResponse {
	out := make([]ServerResponse, 0, len(servers))
	for _, server := range servers {
		out = append(out, newServerResponse(server))
	}
	return out
}

func (s ServersController) Register(r *gin.RouterGroup) {
	r.GET("/list", s.AllServers)
	r.POST("/create", s.CreateServer)
	r.GET("/onlines", s.OnlineUsersServers)
	r.GET("/online_history", s.OnlineHistory)
	r.GET("/:id", s.GetServer)
	r.POST("/:id/edit", s.UpdateServer)
	r.POST("/:id/delete", s.DeleteServer)
	r.POST("/:id/status", s.GetServerStatus) // TODO: реализовать
}

// #region CRUD

func (s ServersController) AllServers(c *gin.Context) {
	servers, err := s.Repo.GetAll()
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get servers"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: newServerResponses(servers)})
}

func (s ServersController) CreateServer(c *gin.Context) {
	var req ServerRequest

	if err := c.ShouldBind(&req); err != nil {
		log.Printf("Failed to bind data: %v\n", err)
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
	}

	server := req.toModel()

	if err := s.Repo.Create(&server); err != nil {
		log.Printf("CreateServer db error: %v\n", err)
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to create server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"redirect": "/panel/servers",
	})
}

func (s ServersController) GetServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}

	server, err := s.Repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Server not found"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Obj: newServerResponse(*server)})
}

func (s ServersController) UpdateServer(ctx *gin.Context) {
	var req ServerRequest

	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
	}

	server := req.toModel()
	if server.Id == 0 {
		id, err := strconv.Atoi(ctx.Param("id"))
		if err != nil {
			ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
			return
		}
		server.Id = id
	}

	existing, err := s.Repo.GetByID(server.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Server not found"})
		return
	}
	if server.SecretWebPath == "" {
		server.SecretWebPath = existing.SecretWebPath
	}
	if server.ApiKey == "" {
		server.ApiKey = existing.ApiKey
	}

	if err := s.Repo.Update(&server); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to update server"})
		return
	}

	ctx.JSON(http.StatusOK, Response{Success: true})
}

func (s ServersController) DeleteServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}

	if err = s.Repo.Delete(id); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to delete server"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Server deleted successfully",
	})
}

// #endregion

// #region Misc

func (s ServersController) GetServerStatus(c *gin.Context) {
	// id, err := strconv.Atoi(c.Param("id"))
	// if err != nil {
	// 	c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
	// 	return
	// }

	// server, err := s.Repo.Get(id)
	// if err != nil {
	// 	c.JSON(http.StatusOK, Response{Success: false, Msg: "Server not found"})
	// 	return
	// }

	// c.JSON(http.StatusOK, Response{Success: true, Obj: server})

	// TODO: дописать когда будет API
	c.JSON(http.StatusNotImplemented, Response{
		Success: false,
		Msg:     "Not implemented",
	})
}

func (s ServersController) OnlineUsersServers(c *gin.Context) {
	servers, total, err := s.Repo.GetAllWithLastStat()
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get servers"})
		return
	}

	fmt.Printf("[INFO] Total online: %d\n", total)
	for _, server := range servers {
		online := 0
		if server.LastStat != nil {
			online = server.LastStat.Online
		}
		fmt.Printf("[INFO] online servers:  %s, %d\n", server.Name, online)
	}

	type OnlineResponse struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			TotalOnline int              `json:"total_online"`
			Servers     []ServerResponse `json:"servers"`
		} `json:"obj"`
	}

	onlineResponse := OnlineResponse{
		Success: true,
		Obj: struct {
			TotalOnline int              `json:"total_online"`
			Servers     []ServerResponse `json:"servers"`
		}{TotalOnline: total, Servers: newServerResponses(servers)},
	}

	c.JSON(http.StatusOK, onlineResponse)

}

func (s ServersController) OnlineHistory(c *gin.Context) {
	history, err := s.Repo.GetOnlineHistory()
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get online history"})
		return
	}
	c.JSON(http.StatusOK, history)
}

// #endregion
