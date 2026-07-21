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

	// clean leading blank lines or spaces so header parsing starts at line 1
	cleanRaw := strings.TrimLeft(raw, "\r\n\t ")

	msg, err := mail.ReadMessage(strings.NewReader(cleanRaw))
	if err != nil {
		parsed.TextBody = cleanRaw
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

	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		body := decodeBody(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		if strings.Contains(mediaType, "text/html") {
			parsed.HTMLBody = template.HTML(body)
			parsed.HasHTML = true
		} else {
			parsed.TextBody = body
		}
		return parsed
	}

	boundary, ok := params["boundary"]
	if !ok {
		body, _ := io.ReadAll(msg.Body)
		parsed.TextBody = string(body)
		return parsed
	}

	parseMultipart(msg.Body, boundary, parsed)
	return parsed
}

func parseMultipart(r io.Reader, boundary string, parsed *ParsedEmail) {
	mr := multipart.NewReader(r, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF || err != nil {
			break
		}

		partContentType := part.Header.Get("Content-Type")
		if partContentType == "" {
			partContentType = "text/plain"
		}

		partMediaType, params, _ := mime.ParseMediaType(partContentType)

		// handle nested multipart (e.g. multipart/related inside multipart/alternative)
		if strings.HasPrefix(partMediaType, "multipart/") {
			if subBoundary, ok := params["boundary"]; ok {
				parseMultipart(part, subBoundary, parsed)
			}
			continue
		}

		partEncoding := part.Header.Get("Content-Transfer-Encoding")
		decodedContent := decodeBody(part, partEncoding)

		if strings.HasPrefix(partMediaType, "text/html") {
			if !parsed.HasHTML {
				parsed.HTMLBody = template.HTML(decodedContent)
				parsed.HasHTML = true
			}
		} else if strings.HasPrefix(partMediaType, "text/plain") {
			if parsed.TextBody == "" {
				parsed.TextBody = decodedContent
			}
		}
	}
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
	return strings.TrimSpace(buf.String())
}
