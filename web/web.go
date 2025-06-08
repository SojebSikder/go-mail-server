package web

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	server "sojebsikder/go-smtp-server/server"
)

// StartWebServer initializes the web server to display emails
func StartWebServer(ctx context.Context, port string) {
	// Initialize the database connection
	_, err := server.GetDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Load templates
	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))

	mux := http.NewServeMux()

	// Display paginated inbox
	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageParam)
		if page < 1 {
			page = 1
		}
		limit := 20
		offset := (page - 1) * limit

		emails, dbErr := server.GetAllEmails(offset, limit)
		if dbErr != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Emails":   emails,
			"NextPage": page + 1,
		}
		err := tmpl.ExecuteTemplate(w, "inbox.html", data)
		if err != nil {
			log.Println("Template error:", err)
			http.Error(w, "Template rendering error", http.StatusInternalServerError)
		}
	})

	// Delete a specific email
	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		emailID := r.URL.Query().Get("id")
		if emailID == "" {
			http.Error(w, "Email ID is required", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(emailID)
		if err != nil {
			http.Error(w, "Invalid email ID", http.StatusBadRequest)
			return
		}
		result := server.DeleteEmailById(id)
		if result.Error != nil {
			http.Error(w, "Failed to delete email", http.StatusInternalServerError)
			return
		}

		tmpl.ExecuteTemplate(w, "message_deleted.html", map[string]int{"ID": id})
	})

	// Delete all emails
	mux.HandleFunc("/delete_all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result := server.DeleteAllEmails()
		if result.Error != nil {
			http.Error(w, "Failed to delete all emails", http.StatusInternalServerError)
			return
		}
		tmpl.ExecuteTemplate(w, "all_deleted.html", nil)
	})

	// HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Run the server
	go func() {
		log.Println("Web server listening on :" + port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	// Wait for shutdown
	<-ctx.Done()

	log.Println("Shutting down web server...")
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Web server shutdown failed: %v", err)
	}
}
