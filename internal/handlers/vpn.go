package handlers

import (
	"vpnpanel/internal/db"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type VpnController struct {
	repo *repository.VpnRepo
}

func NewVpnController(r *gin.RouterGroup) *VpnController {
	apiController := &VpnController{repo: repository.NewVpnRepo(db.DB)}
	apiController.Routes(r)
	return apiController
}

func (s VpnController) Routes(r *gin.RouterGroup) {
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

// CreateVpn
func (s VpnController) CreateVpn(c *gin.Context) {}

// DeleteVpn
func (s VpnController) DeleteVpn(c *gin.Context) {}

// UpdateVpn
func (s VpnController) UpdateVpn(c *gin.Context) {}

// UpdateVpnStatus
func (s VpnController) UpdateVpnStatus(c *gin.Context) {}

// UpdateVpnStatus
func (s VpnController) RegenerateVpn(c *gin.Context) {}
