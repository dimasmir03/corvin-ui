package handlers

import (
	"net/http"
	"strings"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
)

type NodesController struct {
	nodes *service.NodeService
}

func NewNodesController(nodes *service.NodeService) *NodesController {
	return &NodesController{nodes: nodes}
}

func (h *NodesController) Register(r *gin.RouterGroup) {
	r.GET("", h.ListNodes)
	r.GET("/", h.ListNodes)
	r.GET("/:node_id", h.GetNode)
}

func (h *NodesController) ListNodes(c *gin.Context) {
	nodes, err := h.nodes.ListNodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: nodes})
}

func (h *NodesController) GetNode(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Param("node_id"))
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "node_id is required"})
		return
	}
	node, err := h.nodes.GetNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: node})
}
