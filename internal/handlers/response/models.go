package response

// здесь все ответы и приветы по сети распишу

//common response
type Response struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     any    `json:"obj"`
}

type CreateUserDTO struct {
	TgID      int64  `json:"tg_id" binding:"required"`
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

type ClientDTO struct {
	ID        uint   `json:"id"`
	TgID      int64  `json:"tg_id"`
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

type CreateVpnDTO struct {
	TgID int64 `json:"tg_id" binding:"required"`
}

type VpnResult struct {
	TgID int64  `json:"tg_id"`
	Link string `json:"link"`
}

type CreateComplaintDTO struct {
	TgID     int64  `json:"tg_id" binding:"required"`
	Username string `json:"username"`
	Text     string `json:"text" binding:"required"`
}

type UpdateComplaintDTO struct {
	ComplaintID uint   `json:"complaint_id" binding:"required"`
	AdminReply  string `json:"admin_reply"`
	Status      string `json:"status"`
}

type UserRequest struct {
	ID        uint   `json:"id,omitempty"`
	TgID      int64  `json:"tg_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Firstname string `json:"firstname,omitempty"`
	Lastname  string `json:"lastname,omitempty"`
}

type Vpn struct {
	TgID     int64  `json:"tg_id,omitempty"`
	Username string `json:"username,omitempty"`
	Link     string `json:"link,omitempty"`
}

type Complaint struct {
	ID         uint   `json:"id,omitempty"`
	TgID       int64  `json:"tg_id,omitempty"`
	Text       string `json:"text,omitempty"`
	AdminReply string `json:"admin_reply,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Status     string `json:"status,omitempty"`
}
