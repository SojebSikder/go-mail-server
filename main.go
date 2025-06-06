package main

import (
	"context"
	"os/signal"
	server "sojebsikder/go-smtp-server/server"
	"sync"
	"syscall"
)

func main() {
	smtpPort := "2525"
	imapPort := "1430"

	if err := server.InitDB(); err != nil {
		panic("failed to connect to database: " + err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	// Start SMTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.CreateSMTPConnection(ctx, smtpPort)
	}()

	// Start IMAP server on a different port
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.CreateIMAPConnection(ctx, imapPort)
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	println("Shutting down servers...")

	// wait for all goroutines to finish
	wg.Wait()
	println("Servers shut down gracefully.")

}
