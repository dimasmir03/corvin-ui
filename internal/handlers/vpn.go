package handlers

import (
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type VpnController struct {
	repo *repository.VpnRepo
}

func NewVpnController(repo *repository.VpnRepo) *VpnController {
	return &VpnController{repo: repo}
}

func (s VpnController) Register(r *gin.RouterGroup) {
	r.GET("/:user_id", s.GetVpn)
	r.POST("/:user_id/create", s.CreateVpn)
	r.POST("/:user_id/delete", s.DeleteVpn)
	r.POST("/:user_id/edit", s.UpdateVpn)
	r.POST("/:user_id/edit/status", s.UpdateVpnStatus)
	r.POST("/:user_id/regenerate", s.RegenerateVpn)
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
