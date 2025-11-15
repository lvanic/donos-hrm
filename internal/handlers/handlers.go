package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"

	"donos-hrm/internal/auth"
	"donos-hrm/internal/ratelimit"
	"donos-hrm/internal/storage"
)

type Handler struct {
	tmpl        *template.Template
	store       storage.Store
	authManager *auth.Manager
	rateLimiter *ratelimit.Limiter
	adminEmail  string
	stateMu     sync.Mutex
	states      map[string]struct{}
}

func New(tmpl *template.Template, store storage.Store, authManager *auth.Manager, rateLimiter *ratelimit.Limiter, adminEmail string) *Handler {
	return &Handler{
		tmpl:        tmpl,
		store:       store,
		authManager: authManager,
		rateLimiter: rateLimiter,
		adminEmail:  adminEmail,
		states:      make(map[string]struct{}),
	}
}

func (h *Handler) IsAdmin(email string) bool {
	return h.adminEmail != "" && strings.EqualFold(email, h.adminEmail)
}

func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, ok := h.authManager.GetSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !h.IsAdmin(email) {
			http.Error(w, "Access denied. Admin privileges required.", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (h *Handler) HandleForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _ := h.authManager.GetSession(r)

		switch r.Method {
		case http.MethodGet:
			h.renderTemplate(w, "layout", h.viewData(email, "Submit Complaint", "form"))
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			subject := r.FormValue("subject")
			description := r.FormValue("description")

			_, err := h.store.Add(storage.Complaint{
				Reporter:    email,
				Subject:     subject,
				Description: description,
			})
			if err != nil {
				h.renderTemplate(w, "layout", h.viewData(email, "Submit Complaint", "form", map[string]any{"Error": err.Error()}))
				return
			}

			http.Redirect(w, r, "/complaints", http.StatusSeeOther)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (h *Handler) HandleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _ := h.authManager.GetSession(r)
		complaints, err := h.store.List()
		if err != nil {
			http.Error(w, "failed to list complaints", http.StatusInternalServerError)
			return
		}
		h.renderTemplate(w, "layout", h.viewData(email, "Complaints", "list", map[string]any{"Complaints": complaints}))
	}
}

func (h *Handler) HandleLogin() http.HandlerFunc {
	return h.rateLimiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
		state := randomState()
		h.stateMu.Lock()
		h.states[state] = struct{}{}
		h.stateMu.Unlock()
		url := h.authManager.LoginURL(state)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})
}

func (h *Handler) HandleCallback() http.HandlerFunc {
	return h.rateLimiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if !h.consumeState(state) {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		token, err := h.authManager.Exchange(ctx, code)
		if err != nil {
			log.Printf("token exchange failed: %v", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}

		client := h.authManager.Client(ctx, token)
		email, err := h.authManager.GetEmail(ctx, client)
		if err != nil {
			log.Printf("email fetch failed: %v", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}

		// Проверка на наличие "pynest" в email
		if !strings.Contains(strings.ToLower(email), "pynest") {
			log.Printf("access denied: email %s does not contain 'pynest'", email)
			http.Error(w, "Access denied. Only emails containing 'pynest' are allowed.", http.StatusForbidden)
			return
		}

		// Rate limiting по email
		if !h.rateLimiter.CheckEmail(email) {
			log.Printf("rate limit exceeded for email: %s", email)
			http.Error(w, "Too many requests from this email. Please try again later.", http.StatusTooManyRequests)
			return
		}

		if _, err := h.authManager.CreateSession(w, email); err != nil {
			log.Printf("session creation failed: %v", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}

func (h *Handler) HandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.authManager.DeleteSession(w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.authManager.GetSession(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data map[string]any) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	} else {
		log.Printf("rendered %s", name)
	}
}

func (h *Handler) viewData(email, title, bodyTemplate string, extras ...map[string]any) map[string]any {
	data := map[string]any{
		"Title":           title,
		"Email":           email,
		"ContentTemplate": bodyTemplate,
		"IsAdmin":         h.IsAdmin(email),
	}
	for _, extra := range extras {
		for k, v := range extra {
			data[k] = v
		}
	}
	return data
}

func (h *Handler) consumeState(state string) bool {
	if state == "" {
		return false
	}
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if _, ok := h.states[state]; !ok {
		return false
	}
	delete(h.states, state)
	return true
}

func (h *Handler) HandleAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _ := h.authManager.GetSession(r)
		complaints, err := h.store.ListAll()
		if err != nil {
			http.Error(w, "failed to list complaints", http.StatusInternalServerError)
			return
		}
		h.renderTemplate(w, "layout", h.viewData(email, "Admin Panel", "admin", map[string]any{
			"Complaints": complaints,
			"IsAdmin":    true,
		}))
	}
}

func (h *Handler) HandleToggleHidden() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		idStr := r.FormValue("id")
		if idStr == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}

		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		hidden := r.FormValue("hidden") == "true"

		if err := h.store.SetHidden(id, hidden); err != nil {
			log.Printf("failed to toggle hidden: %v", err)
			http.Error(w, "failed to update", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
