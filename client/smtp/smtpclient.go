package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	err := sendMailWithAttachment(
		"user1@example.com",
		"user1@example.com",
		"Test Email with Attachment",
		"This is the email body.",
		"./emails.db", // path to the file to attach
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Email sent successfully with attachment")
}

func sendMailWithAttachment(from, to, subject, body, attachmentPath string) error {
	// Read the file to attach
	fileData, err := os.ReadFile(attachmentPath)
	if err != nil {
		return fmt.Errorf("reading attachment: %w", err)
	}

	// Encode the file data to base64
	encoded := base64.StdEncoding.EncodeToString(fileData)

	// Get the filename
	filename := filepath.Base(attachmentPath)

	// Email headers
	boundary := "my-boundary-779"
	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "multipart/mixed; boundary=" + boundary

	// Compose the message
	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	msg.WriteString(body + "\r\n")

	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: application/octet-stream\r\n")
	msg.WriteString("Content-Transfer-Encoding: base64\r\n")
	msg.WriteString("Content-Disposition: attachment; filename=\"" + filename + "\"\r\n\r\n")

	// Write base64 data in lines of max 76 characters
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msg.WriteString(encoded[i:end] + "\r\n")
	}

	msg.WriteString("--" + boundary + "--")

	// Send the email
	return smtp.SendMail("localhost:2525", nil, from, []string{to}, []byte(msg.String()))
}
