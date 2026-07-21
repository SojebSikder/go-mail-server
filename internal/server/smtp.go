package server

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
)

// StartSMTPListeners spins up both Port 25 (Inbound) and Port 587 (Submission)
func StartSMTPListeners(ctx context.Context) {
	// Port 25: Public Inbound (No mandatory auth upfront, open relay protected)
	go CreateSMTPConnection(ctx, "25", false)

	// Port 587: Client Submission (Strict authentication required)
	go CreateSMTPConnection(ctx, "587", true)
}

func CreateSMTPConnection(ctx context.Context, smtpPort string, requireAuth bool) {
	ln, err := net.Listen("tcp", ":"+smtpPort)
	if err != nil {
		fmt.Printf("Failed to start SMTP server on port %s: %v\n", smtpPort, err)
		return
	}
	fmt.Printf("SMTP server listening on port %s (Auth Required: %v)\n", smtpPort, requireAuth)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Println("SMTP accept error:", err)
				continue
			}
		}
		go HandleSMTP(conn, requireAuth)
	}
}

func HandleSMTP(conn net.Conn, requireAuth bool) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writeLine := func(line string) {
		writer.WriteString(line + "\r\n")
		writer.Flush()
	}

	writeLine("220 Simple SMTP Server Ready")

	var from, to string
	var dataMode bool
	var isAuthenticated bool
	var loggedInUser string
	var data strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		if dataMode {
			// strip trailing CRLF/LF without destroying leading spaces or empty lines
			cleanLine := strings.TrimRight(line, "\r\n")

			if cleanLine == "." {
				// save email to DB
				if err := SaveEmailToDB(from, to, data.String()); err != nil {
					fmt.Println("Failed to save email to DB:", err)
					writeLine("451 Requested action aborted: local error in processing")
				} else {
					fmt.Printf("=== New Email Saved (Port Mode - AuthReq: %v) ===\nFrom: %s\nTo: %s\n", requireAuth, from, to)
					writeLine("250 OK: Message accepted")
				}
				dataMode = false
				data.Reset()
			} else {
				// handle SMTP dot-stuffing (RFC 5321: lines starting with '..' lose one '.')
				if strings.HasPrefix(cleanLine, "..") {
					cleanLine = cleanLine[1:]
				}
				data.WriteString(cleanLine + "\n")
			}
			continue
		}

		// only trim command lines outside DATA mode
		line = strings.TrimSpace(line)

		parts := strings.Fields(line)
		if len(parts) == 0 {
			writeLine("500 Empty command")
			continue
		}

		cmd := strings.ToUpper(parts[0])
		arg := strings.Join(parts[1:], " ")

		switch cmd {
		case "EHLO":
			writeLine("250-Hello")
			writeLine("250 AUTH LOGIN")
		case "HELO":
			writeLine("250 Hello")
		case "AUTH":
			if isAuthenticated {
				writeLine("503 Already authenticated")
				break
			}

			authType := strings.ToUpper(arg)
			if authType == "LOGIN" {
				// prompt for Username (Base64 for "Username:")
				writeLine("334 VXNlcm5hbWU6")
				userBase64, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				userBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(userBase64))
				if err != nil {
					writeLine("501 Invalid Base64 encoding")
					break
				}

				// prompt for Password (Base64 for "Password:")
				writeLine("334 UGFzc3dvcmQ6")
				passBase64, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				passBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(passBase64))
				if err != nil {
					writeLine("501 Invalid Base64 encoding")
					break
				}

				username := strings.TrimSpace(string(userBytes))
				password := strings.TrimSpace(string(passBytes))

				ok, err := AuthenticateUser(username, password)
				if err == nil && ok {
					isAuthenticated = true
					loggedInUser = username
					writeLine("235 2.7.0 Authentication successful")
				} else {
					writeLine("535 5.7.8 Authentication credentials invalid")
				}
			} else {
				writeLine("504 Unrecognized authentication type")
			}

		case "MAIL":
			if requireAuth && !isAuthenticated {
				writeLine("530 5.7.0 Authentication required")
				break
			}

			if strings.HasPrefix(strings.ToUpper(arg), "FROM:") {
				parsedFrom := strings.TrimSpace(strings.TrimPrefix(arg, "FROM:"))
				parsedFrom = strings.Trim(parsedFrom, "<>")
				if parsedFrom == "" && isAuthenticated {
					from = loggedInUser
				} else {
					from = parsedFrom
				}
				writeLine("250 OK")
			} else {
				writeLine("500 Syntax error in MAIL FROM")
			}

		case "RCPT":
			if requireAuth && !isAuthenticated {
				writeLine("530 5.7.0 Authentication required")
				break
			}

			if strings.HasPrefix(strings.ToUpper(arg), "TO:") {
				parsedTo := strings.TrimSpace(strings.TrimPrefix(arg, "TO:"))
				to = strings.Trim(parsedTo, "<>")

				if !requireAuth && !isLocalDomain(to) {
					writeLine("550 5.7.1 Relaying denied")
					break
				}

				writeLine("250 OK")
			} else {
				writeLine("500 Syntax error in RCPT TO")
			}

		case "DATA":
			if requireAuth && !isAuthenticated {
				writeLine("530 5.7.0 Authentication required")
				break
			}
			if to == "" {
				writeLine("503 Bad sequence of commands (missing RCPT TO)")
				break
			}
			writeLine("354 End Data with <CR><LF>.<CR><LF>")
			dataMode = true

		case "QUIT":
			writeLine("221 Bye")
			return

		default:
			writeLine("502 Command not implemented")
		}
	}
}

// Helper function to prevent your server from acting as an open spam relay on port 25
func isLocalDomain(recipient string) bool {
	parts := strings.Split(recipient, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])

	return domain == "jabokivabe.com"
}
