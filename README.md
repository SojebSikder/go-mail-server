# Description

SMTP and IMAP mail server created using go.

## Usage

Run the SMTP, IMAP server:

```bash
go run .
```

To view emails go to following url:

```bash
http://localhost:8080/emails
```

Run SMTP client to send email:

```bash
go run . testsmtp
```

To fetch emails:

```bash
go run . testimap
```

## Supported commands

```bash
Usage:
  smail start [--smtp-port PORT] [--imap-port PORT] [--web-port PORT]
  smail testsmtp
  smail testimap

  smail help
  smail version

Options:
  --smtp-port PORT   Specify the SMTP server port (default: 2525)
  --imap-port PORT   Specify the IMAP server port (default: 1430)
  --web-port PORT    Specify the web server port (default: 8080)
```
