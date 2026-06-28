package broker

import (
	"encoding/json"
	"strconv"
	"strings"
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
	ProfileID         uint           `json:"profile_id,omitempty"`
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
	ProfileID      uint             `json:"profile_id,omitempty"`
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

func (e *JobResultEvent) UnmarshalJSON(data []byte) error {
	type alias JobResultEvent
	var aux struct {
		*alias
		JobID     json.RawMessage `json:"job_id"`
		ProfileID json.RawMessage `json:"profile_id"`
	}
	aux.alias = (*alias)(e)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.JobID = flexibleUint(aux.JobID)
	e.ProfileID = flexibleUint(aux.ProfileID)
	return nil
}

func flexibleUint(raw json.RawMessage) uint {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err == nil {
		return uint(value)
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

type NodeSnapshotEvent struct {
	EventType     string    `json:"event_type"`
	NodeID        string    `json:"node_id"`
	ServerID      string    `json:"server_id,omitempty"`
	EndpointGroup string    `json:"endpoint_group"`
	Protocol      string    `json:"protocol"`
	AgentVersion  string    `json:"agent_version,omitempty"`
	AgentAlive    bool      `json:"agent_alive"`
	InboundID     *int      `json:"inbound_id,omitempty"`
	InboundRemark string    `json:"inbound_remark,omitempty"`
	XUIAvailable  bool      `json:"xui_available"`
	ClientsCount  int       `json:"clients_count"`
	OnlineCount   int       `json:"online_count"`
	TrafficUp     int64     `json:"traffic_up"`
	TrafficDown   int64     `json:"traffic_down"`
	LastError     string    `json:"last_error,omitempty"`
	SentAt        time.Time `json:"sent_at"`
}

func (e *NodeSnapshotEvent) UnmarshalJSON(data []byte) error {
	var aux struct {
		EventType     string          `json:"event_type"`
		NodeID        json.RawMessage `json:"node_id"`
		ServerID      json.RawMessage `json:"server_id"`
		EndpointGroup string          `json:"endpoint_group"`
		Protocol      string          `json:"protocol"`
		AgentVersion  string          `json:"agent_version,omitempty"`
		AgentAlive    bool            `json:"agent_alive"`
		InboundID     *int            `json:"inbound_id,omitempty"`
		InboundRemark string          `json:"inbound_remark,omitempty"`
		XUIAvailable  bool            `json:"xui_available"`
		ClientsCount  int             `json:"clients_count"`
		OnlineCount   int             `json:"online_count"`
		TrafficUp     int64           `json:"traffic_up"`
		TrafficDown   int64           `json:"traffic_down"`
		LastError     string          `json:"last_error,omitempty"`
		SentAt        time.Time       `json:"sent_at"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.EventType = aux.EventType
	e.NodeID = flexibleString(aux.NodeID)
	e.ServerID = flexibleString(aux.ServerID)
	if strings.TrimSpace(e.NodeID) == "" {
		e.NodeID = e.ServerID
	}
	e.EndpointGroup = aux.EndpointGroup
	e.Protocol = aux.Protocol
	e.AgentVersion = aux.AgentVersion
	e.AgentAlive = aux.AgentAlive
	e.InboundID = aux.InboundID
	e.InboundRemark = aux.InboundRemark
	e.XUIAvailable = aux.XUIAvailable
	e.ClientsCount = aux.ClientsCount
	e.OnlineCount = aux.OnlineCount
	e.TrafficUp = aux.TrafficUp
	e.TrafficDown = aux.TrafficDown
	e.LastError = aux.LastError
	e.SentAt = aux.SentAt
	return nil
}

func flexibleString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var value json.Number
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value.String())
	}
	return ""
}

type CollectSnapshotCommand struct {
	EventType    string    `json:"event_type"`
	CommandID    string    `json:"command_id"`
	TargetNodeID string    `json:"target_node_id,omitempty"`
	TargetGroup  string    `json:"target_group,omitempty"`
	RequestedBy  string    `json:"requested_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
