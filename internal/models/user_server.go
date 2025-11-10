package models

type UserServer struct {
	ID       uint `gorm:"primary_key"`
	UserID   uint
	ServerID uint
	// gorm.Model
}
