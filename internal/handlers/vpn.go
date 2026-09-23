package handlers

import (
	"net/http"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
)

type VpnController struct {
	repo    *repository.VpnRepo
	service *service.VPNService
}

func NewVpnController(repo *repository.VpnRepo, vpnService *service.VPNService) *VpnController {
	return &VpnController{repo: repo, service: vpnService}
}

func (s VpnController) Register(r *gin.RouterGroup) {
	r.GET("/auto-routing", s.GetAutoRouting)
	r.PUT("/auto-routing", s.UpdateAutoRouting)
	r.GET("/:user_id", s.GetVpn)
	r.POST("/:user_id/create", s.CreateVpn)
	r.POST("/:user_id/delete", s.DeleteVpn)
	r.POST("/:user_id/edit", s.UpdateVpn)
	r.POST("/:user_id/edit/status", s.UpdateVpnStatus)
	r.POST("/:user_id/regenerate", s.RegenerateVpn)
}

type updateAutoRoutingRequest struct {
	AutoMode     string `json:"auto_mode" binding:"required"`
	AutoServerID string `json:"auto_server_id"`
}

func (s VpnController) GetAutoRouting(c *gin.Context) {
	settings, err := s.service.GetAutoRoutingSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: settings})
}

func (s VpnController) UpdateAutoRouting(c *gin.Context) {
	var request updateAutoRoutingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: err.Error()})
		return
	}
	settings, err := s.service.UpdateAutoRoutingSettings(request.AutoMode, request.AutoServerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: settings})
}

func (s VpnController) GetVpn(c *gin.Context) {
	// user_id, err := strconv.ParseInt(c.Param("user_id"))
	// if err != nil {
	// 	c.JSON(400, gin.H{"error": err.Error()})
	// 	return
	// }

	// s.repo.GetVpn(user_id)

}

func (s VpnController) CreateVpn(c *gin.Context) {}

func (s VpnController) DeleteVpn(c *gin.Context) {}

func (s VpnController) UpdateVpn(c *gin.Context) {}

func (s VpnController) UpdateVpnStatus(c *gin.Context) {}

func (s VpnController) RegenerateVpn(c *gin.Context) {}
