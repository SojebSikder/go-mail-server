package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

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

			// parse raw MIME email body
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

			// process email previews and subjects for Inbox view
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
