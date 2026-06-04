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
	ServerStatusOffline  = "offline"
	ServerStatusDegraded = "degraded"
	ServerStatusDisabled = "disabled"

	ServerManagementModeAgent = "agent"
)

type Server struct {
	Id             int        `gorm:"primary_key;autoIncrement" form:"id" json:"id"`
	Name           string     `gorm:"not null" form:"name" json:"name"`
	IP             string     `gorm:"not null;unique" form:"ip" json:"ip"`
	Port           uint16     `gorm:"not null" form:"port" json:"port"`
	SecretWebPath  string     `gorm:"secretWebPath" form:"secretWebPath" json:"-"`
	ApiKey         string     `gorm:"not null" form:"apiKey" json:"-"`
	Country        string     `form:"country" json:"country"`
	Status         string     `gorm:"not null;default:unknown" form:"status" json:"status"`
	Type           string     `form:"type" json:"type"`
	Enabled        bool       `gorm:"not null;default:true" form:"enabled" json:"enabled"`
	LastSeenAt     *time.Time `form:"lastSeenAt" json:"last_seen_at,omitempty"`
	LastProbeAt    *time.Time `form:"lastProbeAt" json:"last_probe_at,omitempty"`
	LastError      *string    `form:"lastError" json:"last_error,omitempty"`
	PanelVersion   *string    `form:"panelVersion" json:"panel_version,omitempty"`
	XrayVersion    *string    `form:"xrayVersion" json:"xray_version,omitempty"`
	AgentVersion   *string    `form:"agentVersion" json:"agent_version,omitempty"`
	ManagementMode string     `gorm:"not null;default:agent" form:"managementMode" json:"management_mode"`
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
	ID        uint      `gorm:"primary_key;autoIncrement" json:"id" `
	TgID      int64     `json:"telegram_id" `
	Username  string    `json:"username" `
	Text      string    `json:"text" `
	Reply     string    `json:"reply" `
	Status    string    `json:"status" `
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index;not null" json:"user_id" form:"user_id"`
	Photo     bool      `json:"photo" `
	PhotoURL  string    `json:"photo_url" `
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
