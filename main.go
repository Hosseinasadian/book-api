package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var db *sqlx.DB

func main() {
	// Get port from environment (Render sets this automatically)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default for local development
	}

	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		log.Println("⚠️ DATABASE_URL environment variable is not set")
		log.Println("📌 For local development, create a .env file")
		log.Println("📌 For production, set it in Render dashboard")
	} else {
		log.Println("✅ DATABASE_URL found in environment variables")
		initDB(dbURL)
		defer db.Close()
		initTables()
	}

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // In production, replace with your frontend URL
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":  "healthy",
			"message": "API is running smoothly",
			"time":    time.Now().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("📚 Endpoints:")
	log.Printf("   GET  /health")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

func initDB(databaseURL string) {
	log.Printf("🔗 Connecting to database...")

	var err error
	db, err = sqlx.Connect("postgres", databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("✅ Connected to PostgreSQL successfully")
}

func initTables() {
	// ایجاد جدول کتاب‌ها
	booksTable := `
	CREATE TABLE IF NOT EXISTS books (
		id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid(),
		title VARCHAR(255) NOT NULL,
		author VARCHAR(255) NOT NULL,
		description TEXT,
		cover_url TEXT,
		year VARCHAR(4),
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);
	`

	// ایجاد جدول فصل‌ها
	chaptersTable := `
	CREATE TABLE IF NOT EXISTS chapters (
		id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid(),
		book_id VARCHAR(36) REFERENCES books(id) ON DELETE CASCADE,
		title VARCHAR(255) NOT NULL,
		summary TEXT,
		audio_url TEXT,
		order_num INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT NOW()
	);
	`

	// ایجاد ایندکس
	indexes := `
	CREATE INDEX IF NOT EXISTS idx_books_author ON books(author);
	CREATE INDEX IF NOT EXISTS idx_chapters_book_id ON chapters(book_id);
	`

	_, err := db.Exec(booksTable)
	if err != nil {
		log.Printf("⚠️ Could not create books table: %v", err)
	}

	_, err = db.Exec(chaptersTable)
	if err != nil {
		log.Printf("⚠️ Could not create chapters table: %v", err)
	}

	_, err = db.Exec(indexes)
	if err != nil {
		log.Printf("⚠️ Could not create indexes: %v", err)
	}

	log.Println("✅ Database tables initialized")
}
