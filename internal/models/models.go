package models

import (
	"time"
)

type User struct {
	ID       int    `gorm:"primary_key;autoIncrement" json:"id" form:"id"`
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

type Server struct {
	Id            int    `gorm:"primary_key;autoIncrement" form:"id" json:"id"`
	Name          string `gorm:"not null" form:"name" json:"name"`
	IP            string `gorm:"not null;unique" form:"ip" json:"ip"`
	Port          uint16 `gorm:"not null" form:"port" json:"port"`
	SecretWebPath string `gorm:"secretWebPath" form:"secretWebPath" json:"secretWebPath"`
	ApiKey        string `gorm:"not null" form:"apiKey" json:"apiKey"`
	Country       string `form:"country" json:"country"`
	Status        string `form:"status" json:"status"`
	Type          string `form:"type" json:"type"`
	// Online        int         `form:"online" json:"online"`
	LastStat *ServerStat `gorm:"-"  form:"lastStat" json:"lastStat"`
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
	UserID    int    `gorm:"index;not null" json:"user_id" form:"user_id"`
}

type Vpn struct {
	ID     int    `gorm:"primaryKey" json:"id" form:"-"`
	UUID   string `gorm:"unique;not null" json:"uuid" form:"uuid"`
	Status string `gorm:"not null" json:"status" form:"status"`
	// VpnUser    string    `gorm:"unique;not null" json:"vpn_user" form:"vpn_user"`
	// VpnPass    string    `gorm:"not null" json:"vpn_pass" form:"vpn_pass"`
	Link       string    `gorm:"not null" json:"link" form:"link"`
	Created_at time.Time `gorm:"autoCreateTime" json:"created_at"`
	Expires_at time.Time `gorm:"autoCreateTime" json:"expires_at"`
	UserID     uint      `gorm:"index;not null" json:"user_id" form:"user_id"`
}

type Complaint struct {
	ID        uint      `gorm:"primary_key;autoIncrement" json:"id"`
	TgID      int64     `json:"telegram_id"`
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	Reply     string    `json:"reply"`
	Status    string    `json:"status"` // pending, resolved, closed, processing
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index;not null" json:"user_id" form:"user_id"`
}

type Settings struct {
	ID    int    `gorm:"primary_key;autoIncrement" json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}
