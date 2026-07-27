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
	r.POST("/:server_id/disable", h.DisableNode)
	r.POST("/:server_id/enable", h.EnableNode)
	r.POST("/:server_id/archive", h.ArchiveNode)
	r.POST("/:server_id/restore", h.RestoreNode)
	r.POST("/archive-stale-discovered", h.ArchiveStaleDiscovered)
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

func (h *NodesController) DisableNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	serverID := strings.TrimSpace(c.Param("server_id"))
	if serverID == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	if err := h.nodes.DisableServer(c.Request.Context(), serverID); err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_disable", "operation", "disable_node", "request_id", requestID, "server_id", serverID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "node_id": serverID, "status": "disabled"}})
}

func (h *NodesController) EnableNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	serverID := strings.TrimSpace(c.Param("server_id"))
	if serverID == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	if err := h.nodes.EnableServer(c.Request.Context(), serverID); err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_enable", "operation", "enable_node", "request_id", requestID, "server_id", serverID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "node_id": serverID, "status": "enabled"}})
}

func (h *NodesController) ArchiveNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	serverID := strings.TrimSpace(c.Param("server_id"))
	if serverID == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	if err := h.nodes.ArchiveServer(c.Request.Context(), serverID, "manual"); err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_archive", "operation", "archive_node", "request_id", requestID, "server_id", serverID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "node_id": serverID, "status": "archived"}})
}

func (h *NodesController) RestoreNode(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	serverID := strings.TrimSpace(c.Param("server_id"))
	if serverID == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "server_id is required"})
		return
	}
	if err := h.nodes.RestoreServer(c.Request.Context(), serverID); err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_restore", "operation", "restore_node", "request_id", requestID, "server_id", serverID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "node_id": serverID, "status": "restored"}})
}

func (h *NodesController) ArchiveStaleDiscovered(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	count, err := h.nodes.ArchiveStaleDiscovered(c.Request.Context(), 7)
	if err != nil {
		logger.Error("http service call failed", err, "component", "http_api", "handler", "node_archive_stale", "operation", "archive_stale_discovered", "request_id", requestID)
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"archived_count": count}})
}
