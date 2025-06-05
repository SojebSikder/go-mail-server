package main

import (
	"log"
	"net/smtp"
	"strings"
)

func main() {
	from := "sojebsikder10@gmail.com"
	to := "sojebsikder@gmail.com"
	subject := "Test Email"
	body := "Hello from smtp server"

	// Compose the email message
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"",
		body,
	}, "\r\n")

	// Connect to the local SMTP server on port 2525 without authentication
	err := smtp.SendMail("localhost:2525", nil, from, []string{to}, []byte(msg))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Email sent successfully")
}
