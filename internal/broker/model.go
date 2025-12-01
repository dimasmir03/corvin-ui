package broker

type ComplaintReplyTask struct {
	ComplaintID uint   `json:"complaint_id"`
	TgID        int64  `json:"tg_id"`
	UserID      uint   `json:"user_id"`
	Reply       string `json:"reply"`
}

type CreateUserTask struct {
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	UUID       string `json:"uuid"`
	PBK        string `json:"pbk"`
	SID        string `json:"sid"`
	SPX        string `json:"spx"`
	Flow       string `json:"flow"`
	Encryption string `json:"encryption"`
}
