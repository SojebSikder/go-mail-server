package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
)

func CreateSMTPConnection(ctx context.Context, smtpPort string) {
	ln, err := net.Listen("tcp", ":"+smtpPort)
	if err != nil {
		panic("failed to start SMTP server: " + err.Error())
	}
	fmt.Println("SMTP server listening on port", smtpPort)

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
			go HandleSMTP(conn)
		case err := <-errChan:
			fmt.Println("SMTP accept error:", err)
		}
	}
}

// HandleSMTP handles incoming SMTP connections
// It processes commands like HELO, MAIL FROM, RCPT TO, DATA, and QUIT.
// It saves received emails to a database and prints them to the console.
func HandleSMTP(conn net.Conn) {
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
	var data strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)

		if dataMode {
			if line == "." {
				// Email received
				// Save to database
				SaveEmailToDB(from, to, data.String())
				// Print to console
				fmt.Println("=== New Email ===")
				fmt.Println("From:", from)
				fmt.Println("To:", to)
				// fmt.Println("Body:\n", data.String())
				writeLine("250 OK: Message accepted")
				dataMode = false
				data.Reset()
			} else {
				data.WriteString(line + "\n")
			}
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			writeLine("500 Empty command")
			continue
		}

		cmd := strings.ToUpper(parts[0])
		arg := strings.Join(parts[1:], " ")

		switch cmd {
		case "HELO", "EHLO":
			writeLine("250 Hello")
		case "MAIL":
			if strings.HasPrefix(strings.ToUpper(arg), "FROM:") {
				from = strings.TrimPrefix(arg, "FROM:")
				from = strings.Trim(from, "<>")
				writeLine("250 OK")
			} else {
				writeLine("500 Syntax error in MAIL FROM")
			}
		case "RCPT":
			if strings.HasPrefix(strings.ToUpper(arg), "TO:") {
				to = strings.TrimPrefix(arg, "TO:")
				to = strings.Trim(to, "<>")
				writeLine("250 OK")
			} else {
				writeLine("500 Syntax error in RCPT TO")
			}
		case "DATA":
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
