package storage

import (
	"errors"
	"sync"
	"time"
)

type Complaint struct {
	ID          int
	Reporter    string
	Subject     string
	Description string
	CreatedAt   time.Time
}

type Store interface {
	Add(c Complaint) (Complaint, error)
	List() ([]Complaint, error)
}

type MemoryStore struct {
	mu         sync.RWMutex
	complaints []Complaint
	nextID     int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1}
}

func (s *MemoryStore) Add(c Complaint) (Complaint, error) {
	if c.Subject == "" || c.Description == "" {
		return Complaint{}, errors.New("subject and description required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = s.nextID
	c.CreatedAt = time.Now()
	s.nextID++
	s.complaints = append([]Complaint{c}, s.complaints...) // newest first
	return c, nil
}

func (s *MemoryStore) List() ([]Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Complaint, len(s.complaints))
	copy(result, s.complaints)
	return result, nil
}
