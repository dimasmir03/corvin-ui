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

func (s ServersController) Register(r *gin.RouterGroup) {
	r.GET("/list", s.AllServers)
	r.POST("/create", s.CreateServer)
	r.GET("/:id", s.GetServer)
	r.POST("/:id/edit", s.UpdateServer)
	r.POST("/:id/delete", s.DeleteServer)

	r.POST("/:id/status", s.GetServerStatus) // TODO: реализовать
	r.GET("/onlines", s.OnlineUsersServers)
	r.GET("/online_history", s.OnlineHistory)
}

// #region CRUD

func (s ServersController) AllServers(c *gin.Context) {
	servers, err := s.Repo.GetAll()
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get servers"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: servers})
}

func (s ServersController) CreateServer(c *gin.Context) {
	var server models.Server

	if err := c.ShouldBind(&server); err != nil {
		log.Printf("Failed to bind data: %v\n", err)
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
	}

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

	c.JSON(http.StatusOK, Response{Success: true, Obj: server})
}

func (s ServersController) UpdateServer(ctx *gin.Context) {
	var server models.Server

	if err := ctx.ShouldBind(&server); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
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
	for _, s := range servers {
		fmt.Printf("[INFO] online servers:  %s, %d\n", s.Name, int(s.LastStat.Online))
	}

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
		Obj: struct {
			TotalOnline int             `json:"total_online"`
			Servers     []models.Server `json:"servers"`
		}{TotalOnline: total, Servers: servers},
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
