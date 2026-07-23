package server

import (
	"fmt"
	"net"
	"net/smtp"
	"sojebsikder/go-smtp-server/internal/config"
	"strings"
	"time"
)

// sendToExternalMX resolves recipient MX records and performs remote MTA delivery
func SendToExternalMX(from, to, rawMessage string) error {
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid recipient address: %s", to)
	}
	domain := parts[1]

	// resolve MX records for the external target domain
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return fmt.Errorf("failed DNS lookup for domain %s: %w", domain, err)
	}

	// target the highest priority MX host
	targetMX := mxRecords[0].Host

	// dial remote SMTP engine on Port 25
	smtpAddr := net.JoinHostPort(targetMX, "25")
	conn, err := net.DialTimeout("tcp", smtpAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed TCP connect to %s: %w", smtpAddr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, targetMX)
	if err != nil {
		return fmt.Errorf("SMTP handshake failed with %s: %w", targetMX, err)
	}
	defer client.Quit()

	// complete SMTP handshake sequence
	if err := client.Hello(config.GetAllowedSenderDomain()); err != nil {
		return fmt.Errorf("EHLO failed: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	// stream payload stream
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command rejected: %w", err)
	}

	if _, err = wc.Write([]byte(rawMessage)); err != nil {
		return fmt.Errorf("failed payload stream: %w", err)
	}

	return wc.Close()
}
