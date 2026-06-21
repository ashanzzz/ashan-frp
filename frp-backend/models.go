package main

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"time"
)

type FRPTunnel struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string         `gorm:"uniqueIndex;column:name" json:"name"`
	Proto         string         `json:"proto"` // http, https, tcp
	LocalIP       string         `json:"local_ip"`
	LocalPort     int            `json:"local_port"`
	RemotePort    int            `json:"remote_port"`
	CustomDomain  string         `json:"custom_domain"`
	Status1Panel  string         `gorm:"default:'pending'" json:"status_1panel"` 
	StatusChml    string         `gorm:"default:'pending'" json:"status_chml"` 
	StatusDNS     string         `gorm:"default:'pending'" json:"status_dns"` 
	Status        string         `gorm:"default:'stopped'" json:"status"` 
	LastError     string         `json:"last_error"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (FRPTunnel) TableName() string {
	return "frp_tunnels"
}

type FRPServerConfig struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ServerAddr   string `gorm:"column:server_addr" json:"server_addr"`
	ServerPort   int    `gorm:"column:server_port" json:"server_port"`
	AuthToken    string `gorm:"column:auth_token" json:"auth_token"`
	LogTailLines int    `gorm:"column:log_tail_lines;default:500" json:"log_tail_lines"`
}

func (FRPServerConfig) TableName() string {
	return "frp_server_config"
}

type OnePanelConfig struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	BaseURL  string `json:"base_url"` 
	Entrance string `json:"entrance"`   
	APIToken string `json:"api_token"`
}

func (OnePanelConfig) TableName() string {
	return "onepanel_config"
}

var DB *gorm.DB

func InitFRPDB(dbPath string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("GORM storage bridge error: %v", err)
	}
	DB.AutoMigrate(&FRPTunnel{}, &FRPServerConfig{}, &OnePanelConfig{})
}
