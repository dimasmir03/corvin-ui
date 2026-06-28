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
	r.GET("/:server_id", h.GetNode)
	r.POST("/:server_id/refresh", h.RefreshNode)
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
	serverID := strings.TrimSpace(c.Param("server_id"))
	logger.Info("http request params parsed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "server_id", serverID)
	if serverID == "" {
		logger.Warn("http request validation failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "reason", "server_id_required")
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	logger.Info("http service call started", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "server_id", serverID)
	node, err := h.nodes.GetNode(c.Request.Context(), serverID)
	if err != nil {
		logger.Warn("http service call failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "server_id", serverID, "reason", "node_not_found", "error", err)
		c.JSON(http.StatusNotFound, Response{Success: false, Msg: err.Error()})
		return
	}
	logger.Info("http service call succeeded", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "server_id", serverID, "status", node.Status)
	c.JSON(http.StatusOK, Response{Success: true, Obj: node})
}

func (h *NodesController) RefreshNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	serverID := strings.TrimSpace(c.Param("server_id"))
	logger.Info("http request params parsed", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "server_id", serverID)
	if serverID == "" {
		logger.Warn("http request validation failed", "component", "http_api", "handler", "node_get", "operation", "get_node", "request_id", requestID, "reason", "server_id_required")
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	logger.Info("http service call started", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "server_id", serverID)
	result, err := h.nodes.RequestSnapshot(c.Request.Context(), serverID, "admin")
	if err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "server_id", serverID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	logger.Info("http service call succeeded", "component", "http_api", "handler", "node_refresh", "operation", "refresh_node", "request_id", requestID, "server_id", serverID, "command_id", result.CommandID, "reason", "queued")
	c.JSON(http.StatusOK, result)
}
