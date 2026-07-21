package web

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"crypto/rand"
	"encoding/base64"

	server "sojebsikder/go-smtp-server/internal/server"
)

//go:embed templates/*.html static/*.css
var contentFS embed.FS

// Memory session engine
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

	// Authentication Middleware
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
			// Put the session context on the request context scope
			ctx := context.WithValue(r.Context(), "username", username)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}

	// sign up handler
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

	// login handler
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

	// mailbox view
	mux.HandleFunc("/emails", authRequired(func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value("username").(string)
		emailID := r.URL.Query().Get("id")

		if emailID != "" {
			id, _ := strconv.Atoi(emailID)
			email, err := server.GetEmailByIdAndUser(id, username)
			if err != nil {
				http.Error(w, "Email not found", http.StatusNotFound)
				return
			}
			tmpl.ExecuteTemplate(w, "email.html", map[string]interface{}{"Email": email, "User": username})
		} else {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			limit := 20
			offset := (page - 1) * limit

			emails, _ := server.GetEmailsFor(username, offset, limit)
			tmpl.ExecuteTemplate(w, "inbox.html", map[string]interface{}{
				"Emails":   emails,
				"User":     username,
				"NextPage": page + 1,
			})
		}
	}))

	// Delete Item
	mux.HandleFunc("/delete", authRequired(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := r.Context().Value("username").(string)
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))

		server.DeleteEmailByIdAndUser(id, username)
		http.Redirect(w, r, "/emails", http.StatusSeeOther)
	}))

	// Log Out Handler
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			sessionMu.Lock()
			delete(sessions, cookie.Value)
			sessionMu.Unlock()
		}

		// expire the browser cookie by setting MaxAge to -1
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})

		// redirect to login screen
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
