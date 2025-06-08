package server

import (
	"log"
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

func GetDB() (*gorm.DB, error) {
	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	return db, nil
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

func GetEmailsFor(receiver string) ([]Email, error) {
	var emails []Email
	err := db.Where("receiver = ?", receiver).Order("received_at desc").Find(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

// Retrieve all emails with pagination
func GetAllEmails(offset, limit int) ([]Email, error) {
	var emails []Email
	result := db.Order("received_at desc").Offset(offset).Limit(limit).Find(&emails)

	if result.Error != nil {
		return nil, result.Error
	}
	return emails, nil
}

// GetEmailById retrieves a specific email by its ID
func GetEmailById(id uint) (*Email, error) {
	var email Email
	err := db.First(&email, id).Error
	if err != nil {
		return nil, err
	}
	return &email, nil
}

func DeleteEmailById(id int) *gorm.DB {
	result := db.Delete(&Email{}, id)
	return result
}

// delete all emails
func DeleteAllEmails() *gorm.DB {
	result := db.Exec("DELETE FROM emails")
	return result
}
