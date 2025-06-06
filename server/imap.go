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
	for {
		connChan := make(chan net.Conn)
		errChan := make(chan error)

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				errChan <- err
			} else {
				connChan <- conn
			}
		}()

		select {
		case <-ctx.Done():
			ln.Close()
			return
		case conn := <-connChan:
			go HandleIMAP(conn)
		case err := <-errChan:
			fmt.Println("IMAP accept error:", err)
		}
	}
}

// HandleIMAP handles incoming IMAP connections
// It processes commands like LOGIN, FETCH, and LOGOUT.
// It retrieves emails from the database and sends them to the client.
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
			if len(args) != 2 {
				writeLine(tag + " BAD LOGIN requires username and password")
				continue
			}
			loggedInUser = args[0]
			fmt.Println("User logged in:", loggedInUser)
			writeLine(tag + " OK LOGIN completed")

		case "FETCH":
			if loggedInUser == "" {
				writeLine(tag + " NO LOGIN required")
				continue
			}
			emails, _ := GetEmailsFor(loggedInUser)
			for i, email := range emails {
				fmt.Fprintf(conn, "* %d FETCH (BODY[] {%d}\r\n%s)\r\n", i+1, len(email.Body), email.Body)
			}
			fmt.Fprintf(conn, "%s OK FETCH completed\r\n", tag)

		case "LOGOUT":
			writeLine("* BYE Logging out")
			writeLine(tag + " OK LOGOUT completed")
			return

		default:
			writeLine(tag + " BAD Unknown command")
		}
	}
}
