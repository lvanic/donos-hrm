package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

type FileStore struct {
	mu         sync.RWMutex
	filePath   string
	complaints []Complaint
	nextID     int
}

func NewFileStore(filePath string) (*FileStore, error) {
	store := &FileStore{
		filePath:   filePath,
		complaints: []Complaint{},
		nextID:     1,
	}

	// Загружаем данные из файла если он существует
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Определяем следующий ID
	if len(store.complaints) > 0 {
		maxID := 0
		for _, c := range store.complaints {
			if c.ID > maxID {
				maxID = c.ID
			}
		}
		store.nextID = maxID + 1
	}

	return store, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.complaints)
}

func (s *FileStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.complaints, "", "  ")
	if err != nil {
		return err
	}

	// Создаем временный файл для атомарной записи
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	// Переименовываем атомарно
	return os.Rename(tmpFile, s.filePath)
}

func (s *FileStore) Add(c Complaint) (Complaint, error) {
	if c.Subject == "" || c.Description == "" {
		return Complaint{}, errors.New("subject and description required")
	}

	s.mu.Lock()
	c.ID = s.nextID
	c.CreatedAt = time.Now()
	c.Hidden = false
	s.nextID++
	s.complaints = append([]Complaint{c}, s.complaints...) // newest first
	s.mu.Unlock()

	if err := s.save(); err != nil {
		// Откатываем изменения при ошибке сохранения
		s.mu.Lock()
		s.complaints = s.complaints[1:]
		s.nextID--
		s.mu.Unlock()
		return Complaint{}, err
	}

	return c, nil
}

func (s *FileStore) List() ([]Complaint, error) {
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

func (s *FileStore) ListAll() ([]Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Complaint, len(s.complaints))
	copy(result, s.complaints)
	return result, nil
}

func (s *FileStore) Get(id int) (Complaint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.complaints {
		if c.ID == id {
			return c, nil
		}
	}
	return Complaint{}, errors.New("complaint not found")
}

func (s *FileStore) SetHidden(id int, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.complaints {
		if s.complaints[i].ID == id {
			s.complaints[i].Hidden = hidden
			if err := s.save(); err != nil {
				// Откатываем изменение
				s.complaints[i].Hidden = !hidden
				return err
			}
			return nil
		}
	}
	return errors.New("complaint not found")
}

