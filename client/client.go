package main

import (
	"log"
	"net/smtp"
	"strings"
)

func main() {
	from := "user1@example.com"
	to := "user1@example.com"
	subject := "Test Email"
	body := "This is the body"

	// Compose the email message
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"",
		body,
	}, "\r\n")

	// Connect to the local SMTP server on port 2525
	err := smtp.SendMail("localhost:2525", nil, from, []string{to}, []byte(msg))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Email sent successfully")
}
