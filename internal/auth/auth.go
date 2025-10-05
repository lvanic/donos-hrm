package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2v2 "google.golang.org/api/oauth2/v2"
)

const sessionCookie = "session_token"

type Manager struct {
	config       *oauth2.Config
	store        SessionStore
	secureCookie bool
}

func NewManager(clientID, clientSecret, baseURL string) *Manager {
	secure := strings.HasPrefix(strings.ToLower(baseURL), "https://")
	return &Manager{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  fmt.Sprintf("%s/auth/google/callback", baseURL),
			Scopes: []string{
				oauth2v2.UserinfoEmailScope,
			},
			Endpoint: google.Endpoint,
		},
		store:        NewMemorySessionStore(),
		secureCookie: secure,
	}
}

func (m *Manager) LoginURL(state string) string {
	return m.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (m *Manager) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return m.config.Exchange(ctx, code)
}

func (m *Manager) Client(ctx context.Context, token *oauth2.Token) *http.Client {
	return m.config.Client(ctx, token)
}

func (m *Manager) GetEmail(ctx context.Context, client *http.Client) (string, error) {
	svc, err := oauth2v2.New(client)
	if err != nil {
		return "", err
	}
	info, err := svc.Userinfo.Get().Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if info == nil || info.Email == "" {
		return "", fmt.Errorf("no email found")
	}
	return info.Email, nil
}

func (m *Manager) CreateSession(w http.ResponseWriter, email string) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	m.store.Set(token, email)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	return token, nil
}

func (m *Manager) GetSession(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	email, ok := m.store.Get(c.Value)
	return email, ok
}

func (m *Manager) DeleteSession(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookie)
	if err == nil {
		m.store.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
		Secure: m.secureCookie,
	})
}

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
