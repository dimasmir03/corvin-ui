package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
)

type ServersController struct {
	Repo  *repository.ServerRepo
	jobs  *jobsvc.Service
	audit *audit.Logger
	nodes *service.NodeService
}

func NewServersController(repo *repository.ServerRepo, jobs *jobsvc.Service, auditLogger *audit.Logger, nodes ...*service.NodeService) *ServersController {
	controller := &ServersController{Repo: repo, jobs: jobs, audit: auditLogger}
	if len(nodes) > 0 {
		controller.nodes = nodes[0]
	}
	return controller
}

type ServerRequest struct {
	ID             int    `json:"id" form:"id"`
	Name           string `json:"name" form:"name"`
	IP             string `json:"ip" form:"ip"`
	Port           uint16 `json:"port" form:"port"`
	SecretWebPath  string `json:"secretWebPath" form:"secretWebPath"`
	ApiKey         string `json:"apiKey" form:"apiKey"`
	APIKey         string `json:"APIKey" form:"APIKey"`
	Country        string `json:"country" form:"country"`
	Status         string `json:"status" form:"status"`
	Type           string `json:"type" form:"type"`
	Enabled        *bool  `json:"enabled" form:"enabled"`
	ManagementMode string `json:"management_mode" form:"managementMode"`
	NodeRole       string `json:"node_role" form:"nodeRole"`
}

type ServerResponse struct {
	ID                int                `json:"id"`
	Name              string             `json:"name"`
	IP                string             `json:"ip"`
	Port              uint16             `json:"port"`
	Country           string             `json:"country"`
	Status            string             `json:"status"`
	Type              string             `json:"type"`
	Enabled           bool               `json:"enabled"`
	NodeRole          string             `json:"node_role"`
	LastSeenAt        *time.Time         `json:"last_seen_at,omitempty"`
	LastProbeAt       *time.Time         `json:"last_probe_at,omitempty"`
	LastStatsAt       *time.Time         `json:"last_stats_at,omitempty"`
	LastOnlineCount   int                `json:"last_online_count"`
	LastUploadBytes   int64              `json:"last_upload_bytes"`
	LastDownloadBytes int64              `json:"last_download_bytes"`
	LastTotalBytes    int64              `json:"last_total_bytes"`
	LastPanelStatus   *string            `json:"last_panel_status,omitempty"`
	LastXrayStatus    *string            `json:"last_xray_status,omitempty"`
	LastError         *string            `json:"last_error,omitempty"`
	PanelVersion      *string            `json:"panel_version,omitempty"`
	XrayVersion       *string            `json:"xray_version,omitempty"`
	AgentVersion      *string            `json:"agent_version,omitempty"`
	ManagementMode    string             `json:"management_mode"`
	APIKeySet         bool               `json:"api_key_set"`
	LastStat          *models.ServerStat `json:"lastStat,omitempty"`
}

func (r ServerRequest) toModel() models.Server {
	apiKey := r.ApiKey
	if apiKey == "" {
		apiKey = r.APIKey
	}

	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	status := r.Status
	if status == "" {
		status = models.ServerStatusUnknown
	}
	managementMode := r.ManagementMode
	if managementMode == "" {
		managementMode = models.ServerManagementModeAgent
	}
	nodeRole := r.NodeRole
	if nodeRole == "" {
		nodeRole = models.NodeRoleOther
	}

	return models.Server{
		Id:             r.ID,
		Name:           r.Name,
		IP:             r.IP,
		Port:           r.Port,
		SecretWebPath:  r.SecretWebPath,
		ApiKey:         apiKey,
		Country:        r.Country,
		Status:         status,
		Type:           r.Type,
		NodeRole:       nodeRole,
		Enabled:        enabled,
		ManagementMode: managementMode,
	}
}

func newServerResponse(server models.Server) ServerResponse {
	return ServerResponse{
		ID:                server.Id,
		Name:              server.Name,
		IP:                server.IP,
		Port:              server.Port,
		Country:           server.Country,
		Status:            server.Status,
		Type:              server.Type,
		Enabled:           server.Enabled,
		NodeRole:          server.NodeRole,
		LastSeenAt:        server.LastSeenAt,
		LastProbeAt:       server.LastProbeAt,
		LastStatsAt:       server.LastStatsAt,
		LastOnlineCount:   server.LastOnlineCount,
		LastUploadBytes:   server.LastUploadBytes,
		LastDownloadBytes: server.LastDownloadBytes,
		LastTotalBytes:    server.LastTotalBytes,
		LastPanelStatus:   server.LastPanelStatus,
		LastXrayStatus:    server.LastXrayStatus,
		LastError:         server.LastError,
		PanelVersion:      server.PanelVersion,
		XrayVersion:       server.XrayVersion,
		AgentVersion:      server.AgentVersion,
		ManagementMode:    server.ManagementMode,
		APIKeySet:         server.ApiKey != "",
		LastStat:          server.LastStat,
	}
}

func newServerResponses(servers []models.Server) []ServerResponse {
	out := make([]ServerResponse, 0, len(servers))
	for _, server := range servers {
		out = append(out, newServerResponse(server))
	}
	return out
}

func serverManagementDisabled(c *gin.Context) {
	c.JSON(http.StatusGone, Response{Success: false, Msg: "Server management from UI is temporarily disabled; nodes are discovered from agent snapshots"})
}

func (s ServersController) Register(r *gin.RouterGroup) {
	r.GET("", s.AllServers)
	r.GET("/", s.AllServers)
	r.GET("/list", s.AllServers)
	r.POST("/create", s.CreateServer)
	r.GET("/onlines", s.OnlineUsersServers)
	r.GET("/online_history", s.OnlineHistory)
	r.POST("/archive-stale-discovered", s.ArchiveStaleDiscoveredServers)
	r.POST("/:id/disable", s.DisableServerRegistry)
	r.POST("/:id/enable", s.EnableServerRegistry)
	r.POST("/:id/archive", s.ArchiveServerRegistry)
	r.POST("/:id/restore", s.RestoreServerRegistry)
	r.GET("/:id", s.GetServer)
	r.POST("/:id/edit", s.UpdateServer)
	r.POST("/:id/delete", s.DeleteServer)
	r.POST("/:id/probe", s.ProbeServer)
	r.POST("/:id/collect_stats", s.CollectNodeStats)
	r.GET("/:id/stats/latest", s.LatestNodeStats)
	r.GET("/:id/stats/history", s.NodeStatsHistory)
	r.POST("/:id/status", s.GetServerStatus) // TODO: реализовать
}

// #region CRUD

func (s ServersController) AllServers(c *gin.Context) {
	if s.nodes != nil && c.Query("role") == "" {
		nodes, err := s.nodes.ListNodes(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
			return
		}
		c.JSON(http.StatusOK, Response{Success: true, Obj: nodes})
		return
	}
	servers, err := s.Repo.GetAllFiltered(c.Query("role"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: newServerResponses(servers)})
}

func (s ServersController) CreateServer(c *gin.Context) {
	logger.Info("server management disabled", "component", "http_api", "handler", "servers_createserver", "operation", "server_management", "reason", "agent_snapshot_monitoring")
	serverManagementDisabled(c)
}

func (s ServersController) DisableServerRegistry(c *gin.Context) {
	if s.nodes == nil {
		serverManagementDisabled(c)
		return
	}
	serverID := c.Param("id")
	if err := s.nodes.DisableServer(c.Request.Context(), serverID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "status": "disabled"}})
}

func (s ServersController) EnableServerRegistry(c *gin.Context) {
	if s.nodes == nil {
		serverManagementDisabled(c)
		return
	}
	serverID := c.Param("id")
	if err := s.nodes.EnableServer(c.Request.Context(), serverID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "status": "enabled"}})
}

func (s ServersController) ArchiveServerRegistry(c *gin.Context) {
	if s.nodes == nil {
		serverManagementDisabled(c)
		return
	}
	serverID := c.Param("id")
	if err := s.nodes.ArchiveServer(c.Request.Context(), serverID, "manual"); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "status": "archived"}})
}

func (s ServersController) RestoreServerRegistry(c *gin.Context) {
	if s.nodes == nil {
		serverManagementDisabled(c)
		return
	}
	serverID := c.Param("id")
	if err := s.nodes.RestoreServer(c.Request.Context(), serverID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"server_id": serverID, "status": "restored"}})
}

func (s ServersController) ArchiveStaleDiscoveredServers(c *gin.Context) {
	if s.nodes == nil {
		serverManagementDisabled(c)
		return
	}
	count, err := s.nodes.ArchiveStaleDiscovered(c.Request.Context(), 7)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{"archived_count": count}})
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

	c.JSON(http.StatusOK, Response{Success: true, Obj: newServerResponse(*server)})
}

func (s ServersController) UpdateServer(ctx *gin.Context) {
	logger.Info("server management disabled", "component", "http_api", "handler", "servers_updateserver", "operation", "server_management", "reason", "agent_snapshot_monitoring")
	serverManagementDisabled(ctx)
}

func (s ServersController) DeleteServer(ctx *gin.Context) {
	logger.Info("server management disabled", "component", "http_api", "handler", "servers_deleteserver", "operation", "server_management", "reason", "agent_snapshot_monitoring")
	serverManagementDisabled(ctx)
}

func (s ServersController) ProbeServer(c *gin.Context) {
	logger.Info("server management disabled", "component", "http_api", "handler", "servers_probeserver", "operation", "server_management", "reason", "agent_snapshot_monitoring")
	serverManagementDisabled(c)
}

func (s ServersController) CollectNodeStats(c *gin.Context) {
	logger.Info("server management disabled", "component", "http_api", "handler", "servers_collectnodestats", "operation", "server_management", "reason", "agent_snapshot_monitoring")
	serverManagementDisabled(c)
}

func (s ServersController) LatestNodeStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}

	snapshot, err := s.Repo.LatestNodeStats(id)
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Stats not found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: snapshot})
}

func (s ServersController) NodeStatsHistory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	history, err := s.Repo.NodeStatsHistory(id, limit)
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get stats history"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: history})
}

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
	for _, server := range servers {
		online := 0
		if server.LastStat != nil {
			online = server.LastStat.Online
		}
		fmt.Printf("[INFO] online servers:  %s, %d\n", server.Name, online)
	}

	type OnlineResponse struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			TotalOnline int              `json:"total_online"`
			Servers     []ServerResponse `json:"servers"`
		} `json:"obj"`
	}

	onlineResponse := OnlineResponse{
		Success: true,
		Obj: struct {
			TotalOnline int              `json:"total_online"`
			Servers     []ServerResponse `json:"servers"`
		}{TotalOnline: total, Servers: newServerResponses(servers)},
	}

	c.JSON(http.StatusOK, onlineResponse)

}

type onlineHistoryPoint struct {
	Time   time.Time `json:"time"`
	Online int       `json:"online"`
}

func (s ServersController) OnlineHistory(c *gin.Context) {
	rangeName := c.DefaultQuery("range", "24h")
	duration, ok := onlineHistoryRange(rangeName)
	if !ok {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "Invalid range"})
		return
	}

	serverID, err := strconv.Atoi(c.DefaultQuery("server_id", "0"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: "Invalid server_id"})
		return
	}

	stats, err := s.Repo.OnlineHistory(serverID, time.Now().Add(-duration), 500)
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to get online history"})
		return
	}

	points := make([]onlineHistoryPoint, 0, len(stats))
	for _, stat := range stats {
		points = append(points, onlineHistoryPoint{Time: stat.CreatedAt, Online: stat.Online})
	}

	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{
		"server_id": serverID,
		"range":     rangeName,
		"points":    points,
	}})
}

func onlineHistoryRange(value string) (time.Duration, bool) {
	switch value {
	case "1h":
		return time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "24h", "":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

// #endregion
