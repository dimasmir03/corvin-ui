package models

type User struct {
	ID       uint   `gorm:"primary_key;autoIncrement" form:"-" json:"-"`
	Username string `json:"username" form:"username"`
	Password string `json:"-" form:"password"` // bcrypt hash
	Email    string `json:"email" form:"email"`
	Status   string `json:"status" form:"status"` // Active / Inactive
	// gorm.Model
}
