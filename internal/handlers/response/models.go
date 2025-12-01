package response

// здесь все ответы и приветы по сети распишу

//common response
type Response struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg,omitempty"`
	Obj     any    `json:"obj,omitempty"`
}

type CreateUserDTO struct {
	TgID      int64  `json:"tg_id" binding:"required"`
	Username  string `json:"username,omitempty"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
}

type ClientDTO struct {
	ID        uint   `json:"id"`
	TgID      int64  `json:"tg_id"`
	Username  string `json:"username,omitempty"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
}

type CreateVpnDTO struct {
	TgID int64 `json:"tg_id" binding:"required"`
}

type VpnResult struct {
	TgID int64  `json:"tg_id"`
	Link string `json:"link"`
}

type VpnDTO struct {
	TgID     int64  `json:"tg_id,omitempty"`
	Username string `json:"username,omitempty"`
	Link     string `json:"link,omitempty"`
}

type CreateComplaintDTO struct {
	TgID     int64  `json:"tg_id" binding:"required" form:"tg_id"`
	Username string `json:"username,omitempty" form:"username"`
	Text     string `json:"text" binding:"required" form:"text"`
}

type UpdateComplaintDTO struct {
	ComplaintID uint   `json:"complaint_id" binding:"required"`
	AdminReply  string `json:"admin_reply,omitempty"`
	Status      string `json:"status,omitempty"`
}

type ComplaintDTO struct {
	ID         uint   `json:"id"`
	TgID       int64  `json:"tg_id"`
	Text       string `json:"text"`
	AdminReply string `json:"admin_reply,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Status     string `json:"status,omitempty"`
}
