package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"donos-hrm/internal/auth"
	"donos-hrm/internal/handlers"
	"donos-hrm/internal/ratelimit"
	"donos-hrm/internal/storage"
	"time"

	templ "donos-hrm/internal/templates"
)

func main() {
	_ = godotenv.Load()

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	baseURL := os.Getenv("BASE_URL")

	if clientID == "" || clientSecret == "" || baseURL == "" {
		log.Fatal("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and BASE_URL must be set")
	}

	tmpl, err := templ.Load()
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	// Используем файловое хранилище
	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "data/complaints.json"
	}

	// Создаем директорию если не существует
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	store, err := storage.NewFileStore(dataFile)
	if err != nil {
		log.Fatalf("failed to create file store: %v", err)
	}
	log.Printf("using data file: %s", dataFile)

	authManager := auth.NewManager(clientID, clientSecret, baseURL)

	// Rate limiter: 5 запросов в минуту по IP и email
	rateLimiter := ratelimit.NewLimiter(ratelimit.Config{
		MaxRequests: 5,
		Window:      1 * time.Minute,
		CleanupInt:  5 * time.Minute,
	})

	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail != "" {
		log.Printf("admin email: %s", adminEmail)
	}

	h := handlers.New(tmpl, store, authManager, rateLimiter, adminEmail)

	r := mux.NewRouter()
	r.HandleFunc("/", h.RequireAuth(h.HandleForm())).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/form", h.RequireAuth(h.HandleForm())).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/complaints", h.RequireAuth(h.HandleList())).Methods(http.MethodGet)
	r.HandleFunc("/login", h.HandleLogin()).Methods(http.MethodGet)
	r.HandleFunc("/auth/google/callback", h.HandleCallback()).Methods(http.MethodGet)
	r.HandleFunc("/logout", h.HandleLogout()).Methods(http.MethodPost)

	// Admin routes
	r.HandleFunc("/admin", h.RequireAdmin(h.HandleAdmin())).Methods(http.MethodGet)
	r.HandleFunc("/admin/toggle", h.RequireAdmin(h.HandleToggleHidden())).Methods(http.MethodPost)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := ":8045"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
