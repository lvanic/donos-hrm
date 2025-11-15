package storage

import (
	"errors"
	"sync"
	"time"
)

type Complaint struct {
	ID          int       `json:"id"`
	Reporter    string    `json:"reporter"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Hidden      bool      `json:"hidden"`
}

type Store interface {
	Add(c Complaint) (Complaint, error)
	List() ([]Complaint, error)
	ListAll() ([]Complaint, error) // Для админа - все отзывы включая скрытые
	SetHidden(id int, hidden bool) error
	Get(id int) (Complaint, error)
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
	c.Hidden = false
	s.nextID++
	s.complaints = append([]Complaint{c}, s.complaints...) // newest first
	return c, nil
}

func (s *MemoryStore) List() ([]Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var visible []Complaint
	for _, c := range s.complaints {
		if !c.Hidden {
			visible = append(visible, c)
		}
	}
	return visible, nil
}

func (s *MemoryStore) ListAll() ([]Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Complaint, len(s.complaints))
	copy(result, s.complaints)
	return result, nil
}

func (s *MemoryStore) Get(id int) (Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.complaints {
		if c.ID == id {
			return c, nil
		}
	}
	return Complaint{}, errors.New("complaint not found")
}

func (s *MemoryStore) SetHidden(id int, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.complaints {
		if s.complaints[i].ID == id {
			s.complaints[i].Hidden = hidden
			return nil
		}
	}
	return errors.New("complaint not found")
}
