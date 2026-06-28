package models

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID       uint   `gorm:"primary_key;autoIncrement" json:"id" form:"id"`
	Username string `json:"username" form:"username"`
	// Password  string    `json:"-" form:"password"` // bcrypt hash
	// Email     string    `json:"email" form:"email"`
	Status    bool      `json:"status" form:"status"` // Active / Inactive
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	// gorm.Model
	Telegram  Telegram    `json:"telegram" form:"telegram"`
	Vpn       Vpn         `json:"vpn" form:"vpn"`
	Complaint []Complaint `json:"complaint" form:"complaint"`
}

const (
	ServerStatusUnknown  = "unknown"
	ServerStatusOnline   = "online"
	ServerStatusStale    = "stale"
	ServerStatusOffline  = "offline"
	ServerStatusDegraded = "degraded"
	ServerStatusDisabled = "disabled"

	ServerManagementModeAgent = "agent"

	NodeSourceKnown      = "known"
	NodeSourceDiscovered = "discovered"

	NodeRoleRU      = "ru"
	NodeRoleForeign = "foreign"
	NodeRoleDirect  = "direct"
	NodeRoleOther   = "other"
)

type Server struct {
	Id                int        `gorm:"primary_key;autoIncrement" form:"id" json:"id"`
	Name              string     `gorm:"not null" form:"name" json:"name"`
	IP                string     `gorm:"not null;unique" form:"ip" json:"ip"`
	Port              uint16     `gorm:"not null" form:"port" json:"port"`
	SecretWebPath     string     `gorm:"secretWebPath" form:"secretWebPath" json:"-"`
	ApiKey            string     `gorm:"not null" form:"apiKey" json:"-"`
	Country           string     `form:"country" json:"country"`
	Status            string     `gorm:"not null;default:unknown" form:"status" json:"status"`
	Type              string     `form:"type" json:"type"`
	NodeRole          string     `gorm:"not null;default:other;index" form:"nodeRole" json:"node_role"`
	Enabled           bool       `gorm:"not null;default:true" form:"enabled" json:"enabled"`
	LastSeenAt        *time.Time `form:"lastSeenAt" json:"last_seen_at,omitempty"`
	LastProbeAt       *time.Time `form:"lastProbeAt" json:"last_probe_at,omitempty"`
	LastStatsAt       *time.Time `form:"lastStatsAt" json:"last_stats_at,omitempty"`
	LastOnlineCount   int        `gorm:"not null;default:0" form:"lastOnlineCount" json:"last_online_count"`
	LastUploadBytes   int64      `gorm:"not null;default:0" form:"lastUploadBytes" json:"last_upload_bytes"`
	LastDownloadBytes int64      `gorm:"not null;default:0" form:"lastDownloadBytes" json:"last_download_bytes"`
	LastTotalBytes    int64      `gorm:"not null;default:0" form:"lastTotalBytes" json:"last_total_bytes"`
	LastPanelStatus   *string    `form:"lastPanelStatus" json:"last_panel_status,omitempty"`
	LastXrayStatus    *string    `form:"lastXrayStatus" json:"last_xray_status,omitempty"`
	LastError         *string    `form:"lastError" json:"last_error,omitempty"`
	PanelVersion      *string    `form:"panelVersion" json:"panel_version,omitempty"`
	XrayVersion       *string    `form:"xrayVersion" json:"xray_version,omitempty"`
	AgentVersion      *string    `form:"agentVersion" json:"agent_version,omitempty"`
	ManagementMode    string     `gorm:"not null;default:agent" form:"managementMode" json:"management_mode"`
	// Online        int         `form:"online" json:"online"`
	LastStat *ServerStat `gorm:"-" form:"lastStat" json:"lastStat"`
	// gorm.Model
}

type ServerStat struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	ServerID  int       `gorm:"index;not null" json:"server_id"`
	Online    int       `gorm:"not null" json:"online"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type NodeStatsSnapshot struct {
	ID            uint           `gorm:"primary_key;autoIncrement" json:"id"`
	ServerID      uint           `gorm:"index;not null" json:"server_id"`
	OnlineCount   int            `json:"online_count"`
	UploadBytes   int64          `json:"upload_bytes"`
	DownloadBytes int64          `json:"download_bytes"`
	TotalBytes    int64          `json:"total_bytes"`
	PanelStatus   *string        `json:"panel_status,omitempty"`
	XrayStatus    *string        `json:"xray_status,omitempty"`
	PanelVersion  *string        `json:"panel_version,omitempty"`
	XrayVersion   *string        `json:"xray_version,omitempty"`
	AgentVersion  *string        `json:"agent_version,omitempty"`
	Error         *string        `json:"error,omitempty"`
	RawJSON       datatypes.JSON `gorm:"type:jsonb" json:"raw_json,omitempty"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

type NodeState struct {
	ID               uint       `gorm:"primary_key;autoIncrement" json:"id"`
	ServerID         string     `gorm:"index" json:"server_id"`
	NodeID           string     `gorm:"index" json:"node_id,omitempty"`
	DisplayName      string     `json:"display_name"`
	EndpointGroup    string     `gorm:"not null;index" json:"endpoint_group"`
	ExpectedProtocol string     `gorm:"not null;default:unknown;index" json:"expected_protocol"`
	ReportedProtocol string     `gorm:"not null;default:unknown;index" json:"reported_protocol"`
	Protocol         string     `gorm:"not null;index" json:"protocol"`
	AgentVersion     string     `json:"agent_version"`
	AgentAlive       bool       `gorm:"not null;default:true" json:"agent_alive"`
	Status           string     `gorm:"not null;index" json:"status"`
	Source           string     `gorm:"not null;default:discovered;index" json:"source"`
	LastSeenAt       time.Time  `gorm:"not null;index" json:"last_seen"`
	LastSnapshotAt   *time.Time `gorm:"index" json:"last_snapshot_at,omitempty"`
	XUIAvailable     *bool      `json:"xui_available,omitempty"`
	InboundID        *int       `json:"inbound_id,omitempty"`
	InboundRemark    string     `json:"inbound_remark"`
	ClientsCount     int        `gorm:"not null;default:0" json:"clients_count"`
	OnlineCount      int        `gorm:"not null;default:0" json:"online_count"`
	TrafficUp        int64      `gorm:"not null;default:0" json:"traffic_up"`
	TrafficDown      int64      `gorm:"not null;default:0" json:"traffic_down"`
	LastError        string     `json:"last_error"`
	Enabled          bool       `gorm:"not null;default:true" json:"enabled"`
	SentAt           *time.Time `json:"sent_at,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type ServerRegistry struct {
	ID               uint       `gorm:"primary_key;autoIncrement" json:"id"`
	ServerID         string     `gorm:"uniqueIndex;not null" json:"server_id"`
	DisplayName      string     `gorm:"not null" json:"display_name"`
	EndpointGroup    string     `gorm:"not null;default:unknown;index" json:"endpoint_group"`
	ExpectedProtocol string     `gorm:"not null;default:unknown;index" json:"expected_protocol"`
	Source           string     `gorm:"not null;default:discovered;index" json:"source"`
	Enabled          bool       `gorm:"not null;default:true;index" json:"enabled"`
	ArchivedAt       *time.Time `gorm:"index" json:"archived_at,omitempty"`
	ArchivedReason   string     `json:"archived_reason,omitempty"`
	FirstSeenAt      *time.Time `gorm:"index" json:"first_seen_at,omitempty"`
	LastSeenAt       *time.Time `gorm:"index" json:"last_seen_at,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ServerRegistry) TableName() string { return "server_registry" }

type NodeStateSnapshot struct {
	ID               uint           `gorm:"primary_key;autoIncrement" json:"id"`
	ServerID         string         `gorm:"not null;index" json:"server_id"`
	DisplayName      string         `json:"display_name"`
	EndpointGroup    string         `gorm:"not null;index" json:"endpoint_group"`
	ExpectedProtocol string         `gorm:"not null;index" json:"expected_protocol"`
	ReportedProtocol string         `gorm:"not null;index" json:"reported_protocol"`
	AgentVersion     string         `json:"agent_version"`
	AgentAlive       bool           `gorm:"not null;default:true" json:"agent_alive"`
	XUIAvailable     *bool          `gorm:"index" json:"xui_available,omitempty"`
	InboundID        *int           `json:"inbound_id,omitempty"`
	InboundRemark    string         `json:"inbound_remark"`
	ClientsCount     int            `gorm:"not null;default:0" json:"clients_count"`
	OnlineCount      int            `gorm:"not null;default:0" json:"online_count"`
	TrafficUp        int64          `gorm:"not null;default:0" json:"traffic_up"`
	TrafficDown      int64          `gorm:"not null;default:0" json:"traffic_down"`
	LastError        string         `json:"last_error"`
	RawJSON          datatypes.JSON `gorm:"type:jsonb" json:"raw_json,omitempty"`
	SentAt           time.Time      `gorm:"index" json:"sent_at"`
	ReceivedAt       time.Time      `gorm:"not null;index" json:"received_at"`
	CreatedAt        time.Time      `gorm:"autoCreateTime;index" json:"created_at"`
}

type EndpointGroup struct {
	ID         uint      `gorm:"primary_key;autoIncrement" json:"id"`
	Code       string    `gorm:"uniqueIndex;not null" json:"code"`
	Name       string    `gorm:"not null" json:"name"`
	Protocol   string    `gorm:"not null;index" json:"protocol"`
	PublicHost string    `json:"public_host"`
	PublicPort int       `gorm:"not null;default:443" json:"public_port"`
	Security   string    `json:"security"`
	Network    string    `json:"network"`
	SNI        string    `json:"sni"`
	Path       string    `json:"path"`
	Flow       string    `json:"flow"`
	Enabled    bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type VPNClient struct {
	ID             uint      `gorm:"primary_key;autoIncrement" json:"id"`
	UserID         uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	TelegramID     int64     `gorm:"index;not null" json:"telegram_id"`
	ClientCode     string    `gorm:"uniqueIndex;not null" json:"client_code"`
	Email          string    `gorm:"uniqueIndex;not null" json:"email"`
	VlessUUID      string    `gorm:"not null" json:"-"`
	TrojanPassword string    `gorm:"not null" json:"-"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

const (
	VPNProfileStatusPending = "pending"
	VPNProfileStatusActive  = "active"
	VPNProfileStatusPartial = "partial"
	VPNProfileStatusFailed  = "failed"

	VPNProfileNodeStatusPending = "pending"
	VPNProfileNodeStatusSuccess = "success"
	VPNProfileNodeStatusFailed  = "failed"
)

type VPNProfile struct {
	ID            uint             `gorm:"primary_key;autoIncrement" json:"id"`
	VPNClientID   uint             `gorm:"index;not null;uniqueIndex:idx_vpn_profile_unique" json:"vpn_client_id"`
	VPNClient     VPNClient        `gorm:"foreignKey:VPNClientID" json:"vpn_client,omitempty"`
	Profile       string           `gorm:"not null;index;uniqueIndex:idx_vpn_profile_unique" json:"profile"`
	EndpointGroup string           `gorm:"not null;index;uniqueIndex:idx_vpn_profile_unique" json:"endpoint_group"`
	Protocol      string           `gorm:"not null;index" json:"protocol"`
	Status        string           `gorm:"not null;index" json:"status"`
	FinalLink     string           `json:"final_link"`
	LastError     string           `json:"last_error"`
	NotifiedAt    *time.Time       `json:"notified_at,omitempty"`
	CreatedAt     time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
	Nodes         []VPNProfileNode `gorm:"foreignKey:VPNProfileID" json:"nodes,omitempty"`
}

type VPNProfileNode struct {
	ID           uint       `gorm:"primary_key;autoIncrement" json:"id"`
	VPNProfileID uint       `gorm:"index;not null;uniqueIndex:idx_vpn_profile_server_unique" json:"vpn_profile_id"`
	ServerID     string     `gorm:"index;uniqueIndex:idx_vpn_profile_server_unique" json:"server_id"`
	NodeID       string     `gorm:"index" json:"node_id,omitempty"`
	Protocol     string     `gorm:"not null;index" json:"protocol"`
	Status       string     `gorm:"not null;index" json:"status"`
	InboundID    *int       `json:"inbound_id,omitempty"`
	LastError    string     `json:"last_error"`
	AppliedAt    *time.Time `json:"applied_at,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type Telegram struct {
	ID        int    `gorm:"primaryKey" json:"id" form:"-"`
	TgID      int64  `gorm:"unique" json:"telegram_id" form:"telegram_id"`
	Username  string `gorm:"not null" json:"username" form:"username"`
	Firstname string `gorm:"not null" json:"first_name" form:"first_name"`
	Lastname  string `gorm:"not null" json:"last_name" form:"last_name"`
	UserID    uint   `gorm:"index;not null" json:"user_id" form:"user_id"`
}

type Vpn struct {
	ID     int    `gorm:"primaryKey" json:"id" form:"-"`
	UUID   string `gorm:"unique;not null" json:"uuid" form:"uuid"`
	Status string `gorm:"not null" json:"status" form:"status"`

	// VpnUser    string    `gorm:"unique;not null" json:"vpn_user" form:"vpn_user"`
	// VpnPass    string    `gorm:"not null" json:"vpn_pass" form:"vpn_pass"`

	Link string `gorm:"not null" json:"link" form:"link"`

	VlessLink  string `json:"vless_link" form:"vless_link"`
	TrojanLink string `json:"trojan_link" form:"trojan_link"`

	Created_at time.Time `gorm:"autoCreateTime" json:"created_at"`
	Expires_at time.Time `gorm:"autoCreateTime" json:"expires_at"`
	UserID     uint      `gorm:"index;not null" json:"user_id" form:"user_id"`
}

type Complaint struct {
	ID             uint      `gorm:"primary_key;autoIncrement" json:"id" `
	TgID           int64     `json:"telegram_id" `
	Username       string    `json:"username" `
	Text           string    `json:"text" `
	Reply          string    `json:"reply" `
	Status         string    `json:"status" `
	CreatedAt      time.Time `json:"created_at"`
	UserID         uint      `gorm:"index;not null" json:"user_id" form:"user_id"`
	Photo          bool      `json:"photo" `
	PhotoURL       string    `json:"photo_url" `
	PhotoObjectKey string    `json:"photo_object_key" `
	PhotoFileID    string    `json:"photo_file_id" `
}

type Settings struct {
	ID    int    `gorm:"primary_key;autoIncrement" json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

const (
	JobBatchStatusPending        = "pending"
	JobBatchStatusProcessing     = "processing"
	JobBatchStatusSuccess        = "success"
	JobBatchStatusPartialSuccess = "partial_success"
	JobBatchStatusFailed         = "failed"

	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusSuccess    = "success"
	JobStatusFailed     = "failed"
	JobStatusRetrying   = "retrying"
)

type JobBatch struct {
	ID        uint      `gorm:"primary_key;autoIncrement" json:"id"`
	Type      string    `gorm:"not null;index" json:"type"`
	UserID    *uint     `gorm:"index" json:"user_id,omitempty"`
	Status    string    `gorm:"not null;index" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Jobs      []Job     `gorm:"foreignKey:BatchID" json:"jobs,omitempty"`
}

type Job struct {
	ID             uint            `gorm:"primary_key;autoIncrement" json:"id"`
	BatchID        uint            `gorm:"not null;index" json:"batch_id"`
	ServerID       *int            `gorm:"index" json:"server_id,omitempty"`
	TargetServerID string          `gorm:"index" json:"target_server_id,omitempty"`
	Protocol       string          `gorm:"not null;index" json:"protocol"`
	Action         string          `gorm:"not null;index" json:"action"`
	Status         string          `gorm:"not null;index" json:"status"`
	PayloadJSON    datatypes.JSON  `gorm:"type:jsonb" json:"payload_json"`
	ResultJSON     *datatypes.JSON `gorm:"type:jsonb" json:"result_json,omitempty"`
	Error          *string         `json:"error,omitempty"`
	Attempts       int             `gorm:"not null;default:0" json:"attempts"`
	IdempotencyKey string          `gorm:"uniqueIndex;not null" json:"idempotency_key"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type AuditLog struct {
	ID           uint      `gorm:"primary_key;autoIncrement" json:"id"`
	ActorType    string    `gorm:"not null;index" json:"actor_type"`
	ActorID      *string   `gorm:"index" json:"actor_id,omitempty"`
	Action       string    `gorm:"not null;index" json:"action"`
	EntityType   string    `gorm:"not null;index" json:"entity_type"`
	EntityID     *string   `gorm:"index" json:"entity_id,omitempty"`
	Status       string    `gorm:"not null;index" json:"status"`
	Message      string    `json:"message"`
	OldValueJSON *string   `gorm:"type:jsonb" json:"old_value_json,omitempty"`
	NewValueJSON *string   `gorm:"type:jsonb" json:"new_value_json,omitempty"`
	MetadataJSON *string   `gorm:"type:jsonb" json:"metadata_json,omitempty"`
	IP           string    `json:"ip,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}
