package handlers

import (
	"net/http"
	"strings"
	"vpnpanel/internal/logger"
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
	r.POST("/:node_id/refresh", h.RefreshNode)
}

func (h *NodesController) ListNodes(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	logger.Info("http service call started", "component", "http_api", "handler", "nodes_list", "operation", "list_nodes", "request_id", requestID)
	nodes, err := h.nodes.ListNodes(c.Request.Context())
	if err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "nodes_list", "operation", "list_nodes", "request_id", requestID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	logger.Info("http service call succeeded", "component", "http_api", "handler", "nodes_list", "operation", "list_nodes", "request_id", requestID, "nodes_count", len(nodes))
	c.JSON(http.StatusOK, Response{Success: true, Obj: nodes})
}

func (h *NodesController) GetNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	nodeID := strings.TrimSpace(c.Param("node_id"))
	logger.Info("http request params parsed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "node_id", nodeID)
	if nodeID == "" {
		logger.Warn("http request validation failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "reason", "node_id_required")
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "node_id is required"})
		return
	}
	logger.Info("http service call started", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "node_id", nodeID)
	node, err := h.nodes.GetNode(c.Request.Context(), nodeID)
	if err != nil {
		logger.Warn("http service call failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "node_id", nodeID, "reason", "node_not_found", "error", err)
		c.JSON(http.StatusNotFound, Response{Success: false, Msg: err.Error()})
		return
	}
	logger.Info("http service call succeeded", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "node_id", nodeID, "status", node.Status)
	c.JSON(http.StatusOK, Response{Success: true, Obj: node})
}

func (h *NodesController) RefreshNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	nodeID := strings.TrimSpace(c.Param("node_id"))
	logger.Info("http request params parsed", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "node_id", nodeID)
	if nodeID == "" {
		logger.Warn("http request validation failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "reason", "node_id_required")
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "node_id is required"})
		return
	}
	logger.Info("http service call started", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "node_id", nodeID)
	result, err := h.nodes.RequestSnapshot(c.Request.Context(), nodeID, "admin")
	if err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "node_id", nodeID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	logger.Info("http service call succeeded", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "node_id", nodeID, "command_id", result.CommandID, "reason", "queued")
	c.JSON(http.StatusOK, result)
}
