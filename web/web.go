package web

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"time"

	server "sojebsikder/go-smtp-server/server"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// StartWebServer initializes the web server to display emails
func StartWebServer(ctx context.Context, port string) {
	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageParam)
		if page < 1 {
			page = 1
		}
		limit := 20
		offset := (page - 1) * limit

		var emails []server.Email
		result := db.Order("received_at desc").Offset(offset).Limit(limit).Find(&emails)
		if result.Error != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><h1>Inbox</h1><ul>")
		for _, e := range emails {
			content := e.Body
			fmt.Fprintf(w,
				`<li>
					From: %s<br>
					To: %s<br>
					Received: %s<br>
					<pre>%s</pre>
				</li><hr>`,
				html.EscapeString(e.Sender),
				html.EscapeString(e.Receiver),
				e.ReceivedAt.Format(time.RFC1123),
				html.EscapeString(content),
			)
		}
		fmt.Fprintf(w, "</ul><a href='/emails?page=%d'>Next Page</a></body></html>", page+1)
	})

	// Create an HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Run the server in a goroutine
	go func() {
		log.Println("Web server listening on :" + port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Gracefully shut down the server with timeout
	log.Println("Shutting down web server...")
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Web server shutdown failed: %v", err)
	}
}
