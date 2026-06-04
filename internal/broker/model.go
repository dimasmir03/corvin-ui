package broker

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
	JobID             uint   `json:"job_id"`
	BatchID           uint   `json:"batch_id"`
	ServerID          int    `json:"server_id"`
	Action            string `json:"action"`
	Protocol          string `json:"protocol"`
	UserID            uint   `json:"user_id"`
	TechnicalClientID string `json:"technical_client_id"`
}
