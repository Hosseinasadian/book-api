package main

import (
	"database/sql"
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

	r.Route("/api", func(r chi.Router) {
		r.Get("/books", getBooks)
		r.Get("/books/{id}", getBookByID)
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

type Book struct {
	ID          string    `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Author      string    `json:"author" db:"author"`
	CoverURL    string    `json:"coverUrl" db:"cover_url"`
	Description string    `json:"description" db:"description"`
	Year        string    `json:"year" db:"year"`
	Chapters    []Chapter `json:"chapters" db:"-"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

type Chapter struct {
	ID        string    `json:"id" db:"id"`
	BookID    string    `json:"bookId" db:"book_id"`
	Title     string    `json:"title" db:"title"`
	Summary   string    `json:"summary" db:"summary"`
	AudioURL  string    `json:"audioUrl" db:"audio_url"`
	OrderNum  int       `json:"orderNum" db:"order_num"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

func getBooks(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Database connection is not available", http.StatusServiceUnavailable)
		return
	}

	var books []Book
	err := db.Select(&books, `
		SELECT id, title, author, description, cover_url, year, created_at, updated_at 
		FROM books 
		ORDER BY created_at DESC
	`)

	if err != nil {
		log.Printf("❌ Error fetching books from database: %v", err)
		http.Error(w, "Failed to fetch books", http.StatusInternalServerError)
		return
	}

	// اگر کتابی پیدا نشد
	if len(books) == 0 {
		// می‌توانیم داده‌های نمونه برگردانیم
		books = []Book{}
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(books); err != nil {
		log.Printf("❌ Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getBookByID(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "Database connection is not available", http.StatusServiceUnavailable)
		return
	}

	bookID := chi.URLParam(r, "id")

	// کوئری با JOIN برای دریافت همه چیز در یک درخواست
	type BookWithChapters struct {
		Book
		ChapterID      sql.NullString `json:"-" db:"chapter_id"`
		ChapterTitle   sql.NullString `json:"-" db:"chapter_title"`
		ChapterSummary sql.NullString `json:"-" db:"chapter_summary"`
		AudioURL       sql.NullString `json:"-" db:"audio_url"`
		OrderNum       sql.NullInt32  `json:"-" db:"order_num"`
	}

	var rows []BookWithChapters
	err := db.Select(&rows, `
		SELECT 
			b.id, b.title, b.author, b.description, b.cover_url, b.year, 
			b.created_at, b.updated_at,
			c.id as chapter_id, c.title as chapter_title, 
			c.summary as chapter_summary, c.audio_url, c.order_num
		FROM books b
		LEFT JOIN chapters c ON b.id = c.book_id
		WHERE b.id = $1
		ORDER BY c.order_num ASC
	`, bookID)

	if err != nil {
		log.Printf("❌ Error fetching book with chapters: %v", err)
		http.Error(w, "Failed to fetch book", http.StatusInternalServerError)
		return
	}

	if len(rows) == 0 {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	var chapters []Chapter

	book := rows[0].Book

	for _, row := range rows {
		if row.ChapterID.Valid {
			chapters = append(chapters, Chapter{
				ID:        row.ChapterID.String,
				BookID:    bookID,
				Title:     row.ChapterTitle.String,
				Summary:   row.ChapterSummary.String,
				AudioURL:  row.AudioURL.String,
				OrderNum:  int(row.OrderNum.Int32),
				CreatedAt: time.Now(), // اینجا نیاز به اصلاح دارید
			})
		}
	}

	book.Chapters = chapters

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}
