package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	imapclient "sojebsikder/go-smtp-server/internal/client/imap"
	smtpclient "sojebsikder/go-smtp-server/internal/client/smtp"
	server "sojebsikder/go-smtp-server/internal/server"
	"sojebsikder/go-smtp-server/internal/web"
	"sync"
	"syscall"
)

var version = "0.1.0"

func showUsage() {
	fmt.Println("Usage:")
	fmt.Println("  smail start [--smtp-port PORT] [--submission-port PORT] [--imap-port PORT] [--web-port PORT]")
	fmt.Println("  smail testsmtp")
	fmt.Println("  smail testimap")
	fmt.Println()
	fmt.Println("  smail help")
	fmt.Println("  smail version")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --smtp-port PORT        Specify inbound SMTP port (default: 25)")
	fmt.Println("  --submission-port PORT  Specify client submission SMTP port with Auth (default: 587)")
	fmt.Println("  --imap-port PORT        Specify the IMAP server port (default: 1430)")
	fmt.Println("  --web-port PORT         Specify the web server port (default: 8080)")
}

func main() {
	if len(os.Args) < 2 {
		showUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		smtpPort := "25"
		submissionPort := "587"
		imapPort := "1430"
		webPort := "8080"

		fs := flag.NewFlagSet("start", flag.ExitOnError)
		fs.StringVar(&smtpPort, "smtp-port", smtpPort, "Specify the inbound SMTP server port")
		fs.StringVar(&submissionPort, "submission-port", submissionPort, "Specify the authenticated submission SMTP server port")
		fs.StringVar(&imapPort, "imap-port", imapPort, "Specify the IMAP server port")
		fs.StringVar(&webPort, "web-port", webPort, "Specify the web server port")
		fs.Parse(os.Args[2:])

		handleServer(&Config{
			SMTPPort:       smtpPort,
			SubmissionPort: submissionPort,
			IMAPPort:       imapPort,
			WebPort:        webPort,
		})
	case "testsmtp":
		withAttachment := false
		if len(os.Args) > 2 && os.Args[2] == "--with-attachment" {
			withAttachment = true
		}
		smtpclient.ExecuteSMTPClient(withAttachment)
		os.Exit(0)
	case "testimap":
		imapclient.ExecuteIMAPClient()
		os.Exit(0)
	case "help":
		showUsage()
	case "version":
		fmt.Println("Smail version " + version)
	default:
		fmt.Println("Unknown command:", cmd)
		fmt.Println("Use 'smail help' to see available commands.")
		os.Exit(1)
	}
}

type Config struct {
	SMTPPort       string
	SubmissionPort string
	IMAPPort       string
	WebPort        string
}

func handleServer(config *Config) {
	var smtpPort, submissionPort, imapPort, webPort string
	if config == nil {
		smtpPort = "25"
		submissionPort = "587"
		imapPort = "1430"
		webPort = "8080"
	} else {
		smtpPort = config.SMTPPort
		submissionPort = config.SubmissionPort
		imapPort = config.IMAPPort
		webPort = config.WebPort
	}

	if err := server.InitDB(); err != nil {
		panic("failed to connect to database: " + err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	// start Inbound SMTP Server (Port 25 - Auth optional/disabled, relay restricted)
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.CreateSMTPConnection(ctx, smtpPort, false)
	}()

	// start Submission SMTP Server (Port 587 - Auth required)
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.CreateSMTPConnection(ctx, submissionPort, true)
	}()

	// start IMAP server
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

	// Wait for all goroutines to finish
	wg.Wait()
	println("Servers shut down gracefully.")
}
