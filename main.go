package main

import (
	"fmt"
	"net"
	server "sojebsikder/go-smtp-server/server"
)

func main() {
	port := "2525"
	fmt.Printf("Starting SMTP server on port %s...\n", port)
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	fmt.Println("SMTP server listening on port", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Failed to accept:", err)
			continue
		}
		go server.HandleSMTP(conn)
	}
}
