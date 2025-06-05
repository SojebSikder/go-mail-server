package server

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Email struct {
	ID         uint `gorm:"primaryKey"`
	Sender     string
	Receiver   string
	Body       string
	ReceivedAt time.Time
}

var db *gorm.DB

func InitDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, _ := db.DB()
	sqlDB.Exec("PRAGMA journal_mode = WAL;")
	return db.AutoMigrate(&Email{})
}

func SaveEmailToDB(from, to, body string) error {
	email := Email{
		Sender:     from,
		Receiver:   to,
		Body:       body,
		ReceivedAt: time.Now(),
	}
	return db.Create(&email).Error
}
