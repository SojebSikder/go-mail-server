package main

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Email struct {
	ID         uint `gorm:"primarykey"`
	From       string
	To         string
	Subject    string
	HTML       string
	Text       string
	Raw        string
	ReceivedAt time.Time
}

type Backend struct {
	DB *gorm.DB
}

type Session struct {
	Backend *Backend
	From    string
	To      []string
	RawData strings.Builder
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.From = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.To = append(s.To, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.RawData.Write(body)

	env, err := enmime.ReadEnvelope(strings.NewReader(s.RawData.String()))
	if err != nil {
		return err
	}

	email := Email{
		From:       s.From,
		To:         strings.Join(s.To, ", "),
		Subject:    env.GetHeader("Subject"),
		HTML:       env.HTML,
		Text:       env.Text,
		Raw:        s.RawData.String(),
		ReceivedAt: time.Now(),
	}

	log.Printf("Received email from %s to %s: %s", email.From, email.To, email.Subject)
	return s.Backend.DB.Create(&email).Error
}

func (s *Session) Reset()        {}
func (s *Session) Logout() error { return nil }

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{Backend: b}, nil
}

func startSMTPServer(db *gorm.DB) {
	be := &Backend{DB: db}
	s := smtp.NewServer(be)
	s.Addr = ":2525"
	s.Domain = "localhost"
	s.AllowInsecureAuth = true

	log.Println("SMTP server listening on", s.Addr)
	go func() {
		log.Fatal(s.ListenAndServe())
	}()
}

func startWebServer(db *gorm.DB) {
	http.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageParam)
		if page < 1 {
			page = 1
		}
		limit := 20
		offset := (page - 1) * limit

		var emails []Email
		result := db.Order("received_at desc").Offset(offset).Limit(limit).Find(&emails)
		if result.Error != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><h1>Inbox</h1><ul>")
		for _, e := range emails {
			content := e.Text
			if content == "" && e.HTML != "" {
				content = "[HTML content not shown]"
			}
			fmt.Fprintf(w,
				`<li>
					<b>%s</b><br>
					From: %s<br>
					To: %s<br>
					Received: %s<br>
					<pre>%s</pre>
				</li><hr>`,
				html.EscapeString(e.Subject),
				html.EscapeString(e.From),
				html.EscapeString(e.To),
				e.ReceivedAt.Format(time.RFC1123),
				html.EscapeString(content),
			)
		}
		fmt.Fprintf(w, "</ul><a href='/emails?page=%d'>Next Page</a></body></html>", page+1)
	})

	log.Println("Web server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	_ = godotenv.Load()

	db, err := gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}
	if err := db.AutoMigrate(&Email{}); err != nil {
		log.Fatal("failed to migrate database:", err)
	}

	startSMTPServer(db)
	startWebServer(db)
}
