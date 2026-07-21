package server

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

type ParsedEmail struct {
	ID         uint
	Sender     string
	Receiver   string
	Subject    string
	TextBody   string
	HTMLBody   template.HTML
	HasHTML    bool
	ReceivedAt string
}

// ParseMIME transforms a raw MIME string into a structured ParsedEmail
func ParseMIME(raw string) *ParsedEmail {
	parsed := &ParsedEmail{
		Subject: "(No Subject)",
	}

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		// Fallback for non-MIME or raw plain text messages
		parsed.TextBody = raw
		return parsed
	}

	// parse Subject with RFC 2047 header decoding
	if subj := msg.Header.Get("Subject"); subj != "" {
		dec := new(mime.WordDecoder)
		if decodedSubj, err := dec.DecodeHeader(subj); err == nil {
			parsed.Subject = decodedSubj
		} else {
			parsed.Subject = subj
		}
	}

	// parse Body (Single-part or Multi-part)
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		// single part message
		body := decodeBody(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		if strings.Contains(mediaType, "text/html") {
			parsed.HTMLBody = template.HTML(body)
			parsed.HasHTML = true
		} else {
			parsed.TextBody = body
		}
		return parsed
	}

	// handle multipart MIME
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partEncoding := part.Header.Get("Content-Transfer-Encoding")
		decodedContent := decodeBody(part, partEncoding)

		if strings.HasPrefix(partMediaType, "text/html") && !parsed.HasHTML {
			parsed.HTMLBody = template.HTML(decodedContent)
			parsed.HasHTML = true
		} else if strings.HasPrefix(partMediaType, "text/plain") && parsed.TextBody == "" {
			parsed.TextBody = decodedContent
		}
	}

	return parsed
}

func decodeBody(r io.Reader, encoding string) string {
	var reader io.Reader = r

	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		reader = quotedprintable.NewReader(r)
	case "base64":
		reader = base64.NewDecoder(base64.StdEncoding, r)
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	return buf.String()
}
