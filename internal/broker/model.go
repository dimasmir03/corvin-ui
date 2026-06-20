package broker

import (
	"encoding/json"
	"time"
)

type ComplaintReplyTask struct {
	ComplaintID uint   `json:"complaint_id"`
	TgID        int64  `json:"tg_id"`
	UserID      uint   `json:"user_id"`
	Reply       string `json:"reply"`
}

type CreateUserTask struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`

	// vless params
	UUID       string `json:"uuid,omitempty"`
	PBK        string `json:"pbk,omitempty"`
	SID        string `json:"sid,omitempty"`
	SPX        string `json:"spx,omitempty"`
	Flow       string `json:"flow,omitempty"`
	Encryption string `json:"encryption,omitempty"`

	// trojan params
	Type     string `json:"type,omitempty"`
	Security string `json:"security,omitempty"`
	Fp       string `json:"fp,omitempty"`
	Alpn     string `json:"alpn,omitempty"`
	Sni      string `json:"sni,omitempty"`
	Password string `json:"password,omitempty"`
}

type JobTask struct {
	EventType         string         `json:"event_type,omitempty"`
	JobID             uint           `json:"job_id"`
	BatchID           uint           `json:"batch_id"`
	ServerID          int            `json:"server_id"`
	Action            string         `json:"action"`
	CommandType       string         `json:"command_type,omitempty"`
	Protocol          string         `json:"protocol"`
	Profile           string         `json:"profile,omitempty"`
	TargetGroup       string         `json:"target_group,omitempty"`
	TelegramID        int64          `json:"telegram_id,omitempty"`
	UserID            uint           `json:"user_id"`
	ClientCode        string         `json:"client_code,omitempty"`
	Email             string         `json:"email,omitempty"`
	Enable            bool           `json:"enable"`
	ExpiryTime        int64          `json:"expiry_time"`
	TotalGB           int64          `json:"total_gb"`
	Credentials       VPNCredentials `json:"credentials,omitempty"`
	TechnicalClientID string         `json:"technical_client_id"`
	CreatedAt         time.Time      `json:"created_at,omitempty"`
}

type VPNCredentials struct {
	VLESS  VLESSCredentials  `json:"vless,omitempty"`
	Trojan TrojanCredentials `json:"trojan,omitempty"`
}

type VLESSCredentials struct {
	ID   string `json:"id,omitempty"`
	Flow string `json:"flow,omitempty"`
}

type TrojanCredentials struct {
	Password string `json:"password,omitempty"`
}

type JobResultEvent struct {
	EventType      string           `json:"event_type,omitempty"`
	JobID          uint             `json:"job_id"`
	BatchID        uint             `json:"batch_id"`
	ServerID       *int             `json:"server_id,omitempty"`
	NodeID         string           `json:"node_id,omitempty"`
	TargetGroup    string           `json:"target_group,omitempty"`
	Profile        string           `json:"profile,omitempty"`
	CommandType    string           `json:"command_type,omitempty"`
	Protocol       string           `json:"protocol,omitempty"`
	ClientCode     string           `json:"client_code,omitempty"`
	Email          string           `json:"email,omitempty"`
	InboundID      *int             `json:"inbound_id,omitempty"`
	Status         string           `json:"status"`
	RemoteClientID *string          `json:"remote_client_id,omitempty"`
	ConfigLink     *string          `json:"config_link,omitempty"`
	Error          *string          `json:"error,omitempty"`
	CreatedAt      *time.Time       `json:"created_at,omitempty"`
	ResultJSON     *json.RawMessage `json:"result_json,omitempty"`
}

type NodeHeartbeatEvent struct {
	EventType     string    `json:"event_type"`
	NodeID        string    `json:"node_id"`
	EndpointGroup string    `json:"endpoint_group"`
	Protocol      string    `json:"protocol"`
	AgentVersion  string    `json:"agent_version,omitempty"`
	Status        string    `json:"status"`
	LastError     string    `json:"last_error,omitempty"`
	SentAt        time.Time `json:"sent_at"`
}
