package server

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

type Email struct {
	ID         uint `gorm:"primaryKey"`
	Sender     string
	Receiver   string `gorm:"index"`
	Body       string
	ReceivedAt time.Time
}

var DB *gorm.DB

func InitDB() error {
	var err error
	DB, err = gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.Exec("PRAGMA journal_mode = WAL;")
	}

	return DB.AutoMigrate(&User{}, &Email{})
}

// user actions
func RegisterUser(username, password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := User{Username: username, PasswordHash: string(hashed)}
	return DB.Create(&user).Error
}

func AuthenticateUser(username, password string) (bool, error) {
	var user User
	err := DB.Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil, nil
}

// email actions
func SaveEmailToDB(from, to, body string) error {
	email := Email{
		Sender:     from,
		Receiver:   to,
		Body:       body,
		ReceivedAt: time.Now(),
	}
	return DB.Create(&email).Error
}

func GetEmailsFor(receiver string, offset, limit int) ([]Email, error) {
	var emails []Email
	err := DB.Where("receiver = ?", receiver).Order("received_at desc").
		Offset(offset).Limit(limit).Find(&emails).Error
	return emails, err
}

func GetEmailByIdAndUser(id int, user string) (*Email, error) {
	var email Email
	err := DB.Where("id = ? AND receiver = ?", id, user).First(&email).Error
	if err != nil {
		return nil, err
	}
	return &email, nil
}

func DeleteEmailByIdAndUser(id int, user string) error {
	return DB.Where("id = ? AND receiver = ?", id, user).Delete(&Email{}).Error
}
