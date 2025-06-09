package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	server "sojebsikder/go-smtp-server/server"
	"sojebsikder/go-smtp-server/web"
	"sync"
	"syscall"
)

func showUsage() {
	fmt.Println("Usage:")
	fmt.Println("  smail start [--smtp-port PORT] [--imap-port PORT] [--web-port PORT]")
	fmt.Println("  smail help")
	fmt.Println("  smail version")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --smtp-port PORT   Specify the SMTP server port (default: 2525)")
	fmt.Println("  --imap-port PORT   Specify the IMAP server port (default: 1430)")
	fmt.Println("  --web-port PORT    Specify the web server port (default: 8080)")
}

func main() {
	if len(os.Args) < 2 {
		showUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		smtpPort := "2525"
		imapPort := "1430"
		webPort := "8080"

		fs := flag.NewFlagSet("start", flag.ExitOnError)
		fs.StringVar(&smtpPort, "smtp-port", smtpPort, "Specify the SMTP server port")
		fs.StringVar(&imapPort, "imap-port", imapPort, "Specify the IMAP server port")
		fs.StringVar(&webPort, "web-port", webPort, "Specify the web server port")
		fs.Parse(os.Args[2:])

		handleServer(&Config{
			SMTPPort: smtpPort,
			IMAPPort: imapPort,
			WebPort:  webPort,
		})
	case "help":
		showUsage()
	case "version":
		fmt.Println("Smail version 0.0.1")
	default:
		fmt.Println("Unknown command:", cmd)
		fmt.Println("Use 'smail help' to see available commands.")
		os.Exit(1)
	}
}

type Config struct {
	SMTPPort string
	IMAPPort string
	WebPort  string
}

func handleServer(config *Config) {
	var smtpPort, imapPort, webPort string
	if config == nil {
		smtpPort = "2525"
		imapPort = "1430"
		webPort = "8080"
	} else {
		smtpPort = config.SMTPPort
		imapPort = config.IMAPPort
		webPort = config.WebPort
	}

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

	// Start web server
	wg.Add(1)
	go func() {
		defer wg.Done()
		web.StartWebServer(ctx, webPort)
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	println("Shutting down servers...")

	// wait for all goroutines to finish
	wg.Wait()
	println("Servers shut down gracefully.")

}
