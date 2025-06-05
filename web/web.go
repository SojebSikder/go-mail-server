package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Email struct {
	ID         uint `gorm:"primaryKey"`
	Sender     string
	Receiver   string
	Body       string
	ReceivedAt time.Time
}

func main() {
	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open("emails.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Start the web server
	startWebServer(db)
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

	log.Println("Web server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
