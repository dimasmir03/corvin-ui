package handlers

import "github.com/gin-gonic/gin"

func NewAPIController(r *gin.RouterGroup) *APIController {
	apiController := &APIController{}
	apiController.Routes(r)
	return apiController
}

type APIController struct {
}

func (s APIController) Routes(r *gin.RouterGroup) {
	r.GET("/:user_id", s.GetVpn)
	r.POST("/:user_id/create", s.CreateVpn)
	r.POST("/:user_id/delete", s.DeleteVpn)
	r.POST("/:user_id/edit", s.UpdateVpn)
	r.POST("/:user_id/edit/status", s.UpdateVpnStatus)
	r.POST("/:user_id/regenerate", s.RegenerateVpn)
}
