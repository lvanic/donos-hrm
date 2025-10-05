package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"
	"sync"

	"donos-hrm/internal/auth"
	"donos-hrm/internal/storage"
)

type Handler struct {
	tmpl        *template.Template
	store       storage.Store
	authManager *auth.Manager
	stateMu     sync.Mutex
	states      map[string]struct{}
}

func New(tmpl *template.Template, store storage.Store, authManager *auth.Manager) *Handler {
	return &Handler{
		tmpl:        tmpl,
		store:       store,
		authManager: authManager,
		states:      make(map[string]struct{}),
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
	return func(w http.ResponseWriter, r *http.Request) {
		state := randomState()
		h.stateMu.Lock()
		h.states[state] = struct{}{}
		h.stateMu.Unlock()
		url := h.authManager.LoginURL(state)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

func (h *Handler) HandleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if _, err := h.authManager.CreateSession(w, email); err != nil {
			log.Printf("session creation failed: %v", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
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

func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
