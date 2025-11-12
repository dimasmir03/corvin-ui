package models

type Server struct {
	Id            int    `gorm:"primary_key;autoIncrement" form:"id" json:"id"`
	Name          string `gorm:"not null" form:"name" json:"name"`
	IP            string `gorm:"not null;unique" form:"ip" json:"ip"`
	Port          uint16 `gorm:"not null" form:"port" json:"port"`
	SecretWebPath string `gorm:"secretWebPath" form:"secretWebPath" json:"secretWebPath"`
	APIKey        string `gorm:"not null" form:"apiKey" json:"apiKey"`
	Country       string `form:"country" json:"country"`
	Status        string `form:"status" json:"status"`
	// Online        int         `form:"online" json:"online"`
	LastStat *ServerStat `gorm:"-"  form:"lastStat" json:"lastStat"`
	// gorm.Model
}
