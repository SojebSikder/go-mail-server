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
go run client/smtp/smtpclient.go
```

To fetch emails:

```bash
go run client/imap/imapclient.go
```
