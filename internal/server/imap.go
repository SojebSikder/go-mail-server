package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
)

func CreateIMAPConnection(ctx context.Context, imapPort string) {
	ln, err := net.Listen("tcp", ":"+imapPort)
	if err != nil {
		panic("failed to start IMAP server: " + err.Error())
	}
	fmt.Println("IMAP server listening on port", imapPort)

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
				fmt.Println("IMAP accept error:", err)
				continue
			}
		}
		go HandleIMAP(conn)
	}
}

func HandleIMAP(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writeLine := func(line string) {
		writer.WriteString(line + "\r\n")
		writer.Flush()
	}

	writeLine("* OK Simple IMAP Server Ready")
	var loggedInUser string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			writeLine("* BAD Invalid command format")
			continue
		}

		tag := parts[0]
		cmd := strings.ToUpper(parts[1])
		args := parts[2:]

		switch cmd {
		case "LOGIN":
			if len(args) < 2 {
				writeLine(tag + " BAD LOGIN requires username and password")
				continue
			}
			user := strings.Trim(args[0], "\"")
			pass := strings.Trim(strings.Join(args[1:], " "), "\"")

			// authentication
			ok, err := AuthenticateUser(user, pass)
			if err != nil || !ok {
				writeLine(tag + " NO Authentication failed")
				continue
			}

			loggedInUser = user
			writeLine(tag + " OK LOGIN completed")

		case "FETCH":
			if loggedInUser == "" {
				writeLine(tag + " NO LOGIN required")
				continue
			}
			// fetch first 100 entries
			emails, err := GetEmailsFor(loggedInUser, 0, 100)
			if err != nil {
				writeLine(tag + " NO Error fetching emails")
				continue
			}

			for i, email := range emails {
				fmt.Fprintf(writer, "* %d FETCH (BODY[] {%d}\r\n%s)\r\n", i+1, len(email.Body), email.Body)
			}
			writer.Flush()
			writeLine(tag + " OK FETCH completed")

		case "LOGOUT":
			writeLine("* BYE Logging out")
			writeLine(tag + " OK LOGOUT completed")
			return

		default:
			writeLine(tag + " BAD Unknown command")
		}
	}
}
