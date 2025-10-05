package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"donos-hrm/internal/auth"
	"donos-hrm/internal/handlers"
	"donos-hrm/internal/storage"
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

	store := storage.NewMemoryStore()
	authManager := auth.NewManager(clientID, clientSecret, baseURL)

	h := handlers.New(tmpl, store, authManager)

	r := mux.NewRouter()
	r.HandleFunc("/", h.RequireAuth(h.HandleForm())).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/form", h.RequireAuth(h.HandleForm())).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/complaints", h.RequireAuth(h.HandleList())).Methods(http.MethodGet)
	r.HandleFunc("/login", h.HandleLogin()).Methods(http.MethodGet)
	r.HandleFunc("/auth/google/callback", h.HandleCallback()).Methods(http.MethodGet)
	r.HandleFunc("/logout", h.HandleLogout()).Methods(http.MethodPost)

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
