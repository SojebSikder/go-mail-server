# Description

SMTP and IMAP mail server created using go.

## Usage

Run the SMTP, IMAP server:

```bash
go run .
```

Run SMTP client to send email:

```bash
go run client/smtp/smtpclient.go
```

To fetch emails:

```bash
go run client/imap/imapclient.go
```

To view emails run the following command:

```bash
go run web/web.go
```
