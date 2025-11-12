package models

import "time"

type ServerStat struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  int       `gorm:"index;not null" json:"server_id"`
	Online    int       `gorm:"not null" json:"online"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
