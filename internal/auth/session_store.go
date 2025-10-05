package auth

import "sync"

type SessionStore interface {
	Set(token, email string)
	Get(token string) (string, bool)
	Delete(token string)
}

type MemorySessionStore struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{store: make(map[string]string)}
}

func (s *MemorySessionStore) Set(token, email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[token] = email
}

func (s *MemorySessionStore) Get(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email, ok := s.store[token]
	return email, ok
}

func (s *MemorySessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, token)
}
