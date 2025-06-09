package imapclient

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func ExecuteIMAPClient() {
	conn, err := net.Dial("tcp", "localhost:1430")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	readResponse(reader)

	// Send LOGIN command
	sendCommand(writer, "a1 LOGIN bob@example.com anypassword")
	readResponse(reader)

	// Send FETCH command
	sendCommand(writer, "a2 FETCH 1 BODY[]")
	readMultiResponse(reader)

	// Send LOGOUT command
	sendCommand(writer, "a3 LOGOUT")
	readMultiResponse(reader)
}

func sendCommand(w *bufio.Writer, cmd string) {
	fmt.Println(">>>", cmd)
	w.WriteString(cmd + "\r\n")
	w.Flush()
}

func readResponse(r *bufio.Reader) {
	line, _ := r.ReadString('\n')
	fmt.Print("<<<", line)
}

func readMultiResponse(r *bufio.Reader) {
	for {
		line, _ := r.ReadString('\n')
		fmt.Print("<<<", line)
		if strings.HasPrefix(line, "a") || strings.HasPrefix(line, "* BYE") {
			break
		}
	}
}
