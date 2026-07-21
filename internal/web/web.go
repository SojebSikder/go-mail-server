package web

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"sojebsikder/go-smtp-server/internal/config"
	server "sojebsikder/go-smtp-server/internal/server"
)

type contextKey string

const userContextKey contextKey = "username"

//go:embed templates/*.html static/*.css
var contentFS embed.FS

var (
	sessions  = make(map[string]string)
	sessionMu sync.RWMutex
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func StartWebServer(ctx context.Context, port string) {
	tmpl := template.Must(template.ParseFS(contentFS, "templates/*.html"))
	mux := http.NewServeMux()

	mux.Handle("/static/", http.FileServer(http.FS(contentFS)))

	authRequired := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			sessionMu.RLock()
			username, exists := sessions[cookie.Value]
			sessionMu.RUnlock()

			if !exists {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			reqCtx := context.WithValue(r.Context(), userContextKey, username)
			next.ServeHTTP(w, r.WithContext(reqCtx))
		}
	}

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tmpl.ExecuteTemplate(w, "register.html", nil)
			return
		}
		u := r.FormValue("username")
		p := r.FormValue("password")
		if u == "" || p == "" {
			tmpl.ExecuteTemplate(w, "register.html", map[string]string{"Error": "Fields cannot be blank"})
			return
		}
		if err := server.RegisterUser(u, p); err != nil {
			tmpl.ExecuteTemplate(w, "register.html", map[string]string{"Error": "Username taken or error processing request"})
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tmpl.ExecuteTemplate(w, "login.html", nil)
			return
		}
		u := r.FormValue("username")
		p := r.FormValue("password")

		ok, err := server.AuthenticateUser(u, p)
		if err != nil || !ok {
			tmpl.ExecuteTemplate(w, "login.html", map[string]string{"Error": "Invalid username or password"})
			return
		}

		token := generateToken()
		sessionMu.Lock()
		sessions[token] = u
		sessionMu.Unlock()

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/emails", http.StatusSeeOther)
	})

	mux.HandleFunc("/compose", authRequired(func(w http.ResponseWriter, r *http.Request) {
		username, _ := r.Context().Value(userContextKey).(string)
		tmpl.ExecuteTemplate(w, "compose.html", map[string]interface{}{
			"User": username,
		})
	}))

	mux.HandleFunc("/send", authRequired(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username, _ := r.Context().Value(userContextKey).(string)
		to := strings.TrimSpace(r.FormValue("to"))
		subject := r.FormValue("subject")
		body := r.FormValue("body")

		if to == "" || subject == "" || body == "" {
			tmpl.ExecuteTemplate(w, "compose.html", map[string]interface{}{
				"User":    username,
				"Error":   "All fields are required.",
				"To":      to,
				"Subject": subject,
				"Body":    body,
			})
			return
		}

		// Ensure local domain formatting for sender
		fromAddr := username
		if !strings.Contains(fromAddr, "@") {
			fromAddr = username + "@" + config.DOMAIN
		}

		// format RFC 822 compliant raw MIME body
		rawEmail := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			fromAddr, to, subject, body)

		// save a copy of sent mail locally
		if err := server.SaveOutboundEmail(fromAddr, to, rawEmail); err != nil {
			log.Printf("Warning: failed to save outbound copy to DB: %v", err)
		}

		// determine destination domain
		recipientParts := strings.Split(to, "@")
		if len(recipientParts) != 2 {
			tmpl.ExecuteTemplate(w, "compose.html", map[string]interface{}{
				"User":    username,
				"Error":   "Invalid email address provided.",
				"To":      to,
				"Subject": subject,
				"Body":    body,
			})
			return
		}

		recipientDomain := strings.ToLower(recipientParts[1])

		// route: Internal domain vs External internet delivery
		if recipientDomain == config.DOMAIN {
			// Local recipient -> deliver directly to local engine
			err := sendDirectViaSMTP("127.0.0.1:25", fromAddr, to, rawEmail)
			if err != nil {
				tmpl.ExecuteTemplate(w, "compose.html", map[string]interface{}{
					"User":    username,
					"Error":   "Failed to deliver local email: " + err.Error(),
					"To":      to,
					"Subject": subject,
					"Body":    body,
				})
				return
			}
		} else {
			// External recipient (e.g. gmail.com) -> Dispatch via MX Lookup asynchronously
			go func(from, recipient, payload string) {
				if err := server.SendToExternalMX(from, recipient, payload); err != nil {
					log.Printf("[MTA ERROR] Outbound delivery to %s failed: %v", recipient, err)
				} else {
					log.Printf("[MTA SUCCESS] Successfully delivered outbound email to %s", recipient)
				}
			}(fromAddr, to, rawEmail)
		}

		http.Redirect(w, r, "/emails", http.StatusSeeOther)
	}))

	mux.HandleFunc("/emails", authRequired(func(w http.ResponseWriter, r *http.Request) {
		username, _ := r.Context().Value(userContextKey).(string)
		emailID := r.URL.Query().Get("id")

		if emailID != "" {
			id, _ := strconv.Atoi(emailID)
			email, err := server.GetEmailByIdAndUser(id, username)
			if err != nil {
				http.Error(w, "Email not found", http.StatusNotFound)
				return
			}

			parsed := server.ParseMIME(email.Body)
			parsed.ID = email.ID
			parsed.Sender = email.Sender
			parsed.Receiver = email.Receiver
			parsed.ReceivedAt = email.ReceivedAt.Format("Jan 02, 2006 15:04")

			tmpl.ExecuteTemplate(w, "email.html", map[string]interface{}{
				"Email": parsed,
				"User":  username,
			})
		} else {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			limit := 20
			offset := (page - 1) * limit

			rawEmails, _ := server.GetEmailsFor(username, offset, limit)

			var parsedEmails []*server.ParsedEmail
			for _, email := range rawEmails {
				p := server.ParseMIME(email.Body)
				p.ID = email.ID
				p.Sender = email.Sender
				p.Receiver = email.Receiver
				p.ReceivedAt = email.ReceivedAt.Format("Jan 02, 15:04")
				parsedEmails = append(parsedEmails, p)
			}

			tmpl.ExecuteTemplate(w, "inbox.html", map[string]interface{}{
				"Emails":   parsedEmails,
				"User":     username,
				"NextPage": page + 1,
			})
		}
	}))

	mux.HandleFunc("/delete", authRequired(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username, _ := r.Context().Value(userContextKey).(string)
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))

		server.DeleteEmailByIdAndUser(id, username)
		http.Redirect(w, r, "/emails", http.StatusSeeOther)
	}))

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			sessionMu.Lock()
			delete(sessions, cookie.Value)
			sessionMu.Unlock()
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		log.Println("Secure web UI portal live at http://localhost:" + port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP serving error: %v", err)
		}
	}()

	<-ctx.Done()
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctxShutdown)
}

// sendDirectViaSMTP connects directly to the local SMTP engine over TCP
func sendDirectViaSMTP(addr, from, to, message string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// read greeting banner
	if _, err := reader.ReadString('\n'); err != nil {
		return err
	}

	sendCommand := func(cmd string) error {
		fmt.Fprintf(conn, "%s\r\n", cmd)
		resp, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if len(resp) < 3 || (resp[0] != '2' && resp[0] != '3') {
			return fmt.Errorf("SMTP error: %s", strings.TrimSpace(resp))
		}
		return nil
	}

	if err := sendCommand("EHLO localhost"); err != nil {
		return err
	}
	if err := sendCommand(fmt.Sprintf("MAIL FROM:<%s>", from)); err != nil {
		return err
	}
	if err := sendCommand(fmt.Sprintf("RCPT TO:<%s>", to)); err != nil {
		return err
	}
	if err := sendCommand("DATA"); err != nil {
		return err
	}

	// handle dot-stuffing for message lines
	lines := strings.Split(message, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ".") {
			line = "." + line
		}
		fmt.Fprintf(conn, "%s\r\n", line)
	}

	// end DATA payload
	if err := sendCommand("."); err != nil {
		return err
	}

	return sendCommand("QUIT")
}
