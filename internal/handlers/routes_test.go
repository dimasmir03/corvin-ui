package handlers

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNodeAndServerRouteRegistrationDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	NewNodesController(nil).Register(api.Group("/nodes"))
	NewServersController(nil, nil, nil, nil).Register(api.Group("/servers"))
}
