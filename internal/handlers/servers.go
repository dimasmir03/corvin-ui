package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type ServersController struct {
	Repo  *repository.ServerRepo
	jobs  *jobsvc.Service
	audit *audit.Logger
}

func NewServersController(repo *repository.ServerRepo, jobs *jobsvc.Service, auditLogger *audit.Logger) *ServersController {
	return &ServersController{Repo: repo, jobs: jobs, audit: auditLogger}
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

func (s ServersController) Register(r *gin.RouterGroup) {
	r.GET("", s.AllServers)
	r.GET("/", s.AllServers)
	r.GET("/list", s.AllServers)
	r.POST("/create", s.CreateServer)
	r.GET("/onlines", s.OnlineUsersServers)
	r.GET("/online_history", s.OnlineHistory)
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
	servers, err := s.Repo.GetAllFiltered(c.Query("role"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Obj: newServerResponses(servers)})
}

func (s ServersController) CreateServer(c *gin.Context) {
	var req ServerRequest

	if err := c.ShouldBind(&req); err != nil {
		log.Printf("Failed to bind data: %v\n", err)
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
	}

	server := req.toModel()

	if err := s.Repo.Create(&server); err != nil {
		log.Printf("CreateServer db error: %v\n", err)
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to create server"})
		return
	}

	_ = s.audit.Log(audit.Event{
		ActorType:  audit.ActorAdmin,
		Action:     "server.created",
		EntityType: "server",
		EntityID:   audit.StringID(server.Id),
		Status:     audit.StatusSuccess,
		Message:    "server created",
		NewValue: map[string]any{
			"name": server.Name,
			"ip":   server.IP,
			"port": server.Port,
		},
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"redirect": "/panel/servers",
	})
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
	var req ServerRequest

	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to bind server data"})
		return
	}

	server := req.toModel()
	if server.Id == 0 {
		id, err := strconv.Atoi(ctx.Param("id"))
		if err != nil {
			ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
			return
		}
		server.Id = id
	}

	existing, err := s.Repo.GetByID(server.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Server not found"})
		return
	}
	if server.SecretWebPath == "" {
		server.SecretWebPath = existing.SecretWebPath
	}
	if server.ApiKey == "" {
		server.ApiKey = existing.ApiKey
	}
	if req.Enabled == nil {
		server.Enabled = existing.Enabled
	}
	if req.Status == "" {
		server.Status = existing.Status
	}
	if req.ManagementMode == "" {
		server.ManagementMode = existing.ManagementMode
	}
	if req.NodeRole == "" {
		server.NodeRole = existing.NodeRole
	}
	server.LastSeenAt = existing.LastSeenAt
	server.LastProbeAt = existing.LastProbeAt
	server.LastStatsAt = existing.LastStatsAt
	server.LastOnlineCount = existing.LastOnlineCount
	server.LastUploadBytes = existing.LastUploadBytes
	server.LastDownloadBytes = existing.LastDownloadBytes
	server.LastTotalBytes = existing.LastTotalBytes
	server.LastPanelStatus = existing.LastPanelStatus
	server.LastXrayStatus = existing.LastXrayStatus
	server.LastError = existing.LastError
	server.PanelVersion = existing.PanelVersion
	server.XrayVersion = existing.XrayVersion
	server.AgentVersion = existing.AgentVersion

	oldStatus := existing.Status
	oldEnabled := existing.Enabled
	if err := s.Repo.Update(&server); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to update server"})
		return
	}

	if oldEnabled != server.Enabled {
		action := "server.enabled"
		message := "server enabled"
		if !server.Enabled {
			action = "server.disabled"
			message = "server disabled"
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorAdmin,
			Action:     action,
			EntityType: "server",
			EntityID:   audit.StringID(server.Id),
			Status:     audit.StatusSuccess,
			Message:    message,
			OldValue:   map[string]any{"enabled": oldEnabled},
			NewValue:   map[string]any{"enabled": server.Enabled},
			IP:         ctx.ClientIP(),
			UserAgent:  ctx.Request.UserAgent(),
		})
	}

	if oldStatus != server.Status && server.Status == models.ServerStatusDisabled {
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorAdmin,
			Action:     "server.disabled",
			EntityType: "server",
			EntityID:   audit.StringID(server.Id),
			Status:     audit.StatusSuccess,
			Message:    "server status disabled",
			OldValue:   map[string]any{"status": oldStatus},
			NewValue:   map[string]any{"status": server.Status},
			IP:         ctx.ClientIP(),
			UserAgent:  ctx.Request.UserAgent(),
		})
	}

	ctx.JSON(http.StatusOK, Response{Success: true})
}

func (s ServersController) DeleteServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}

	if err = s.Repo.Delete(id); err != nil {
		ctx.JSON(http.StatusOK, Response{Success: false, Msg: "Failed to delete server"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Server deleted successfully",
	})
}

// #endregion

// #region Misc

func (s ServersController) ProbeServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}
	if s.jobs == nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Jobs service is not configured"})
		return
	}

	batch, job, err := s.jobs.ProbeServer(id)
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{
		"batch_id": batch.ID,
		"job_id":   job.ID,
		"status":   batch.Status,
	}})
}

func (s ServersController) CollectNodeStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Invalid ID"})
		return
	}
	if s.jobs == nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: "Jobs service is not configured"})
		return
	}

	batch, job, err := s.jobs.CollectNodeStats(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, Response{Success: false, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{
		"batch_id": batch.ID,
		"job_id":   job.ID,
		"status":   batch.Status,
	}})
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
