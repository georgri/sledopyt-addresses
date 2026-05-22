package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type JSONStore struct {
	path string
	mu   sync.RWMutex
	data map[int64]string
}

func NewJSONStore(path string) (*JSONStore, error) {
	s := &JSONStore{
		path: path,
		data: map[int64]string{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) Get(userID int64) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[userID]
	return v, ok
}

func (s *JSONStore) Set(userID int64, cityCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[userID] = cityCode
	return s.saveLocked()
}

func (s *JSONStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read user state: %w", err)
	}

	var tmp map[string]string
	if err := json.Unmarshal(b, &tmp); err != nil {
		return fmt.Errorf("parse user state json: %w", err)
	}

	for k, v := range tmp {
		var id int64
		if _, err := fmt.Sscan(k, &id); err == nil {
			s.data[id] = v
		}
	}

	return nil
}

func (s *JSONStore) saveLocked() error {
	tmp := make(map[string]string, len(s.data))
	for id, city := range s.data {
		tmp[fmt.Sprintf("%d", id)] = city
	}

	b, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		return fmt.Errorf("encode user state: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0o644); err != nil {
		return fmt.Errorf("write user state: %w", err)
	}
	return nil
}
