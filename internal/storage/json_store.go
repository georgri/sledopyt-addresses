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
	data map[int64][]string
}

func NewJSONStore(path string) (*JSONStore, error) {
	s := &JSONStore{
		path: path,
		data: map[int64][]string{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) Get(userID int64) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[userID]
	if !ok || len(v) == 0 {
		return nil, false
	}
	out := append([]string(nil), v...)
	return out, true
}

func (s *JSONStore) Set(userID int64, cityCodes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	norm := normalizeCodes(cityCodes)
	if len(norm) == 0 {
		delete(s.data, userID)
	} else {
		s.data[userID] = norm
	}
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

	// Backward-compatible parser:
	// - old format: { "123": "77000000000" }
	// - new format: { "123": ["77000000000","50000000000"] }
	var tmp map[string]json.RawMessage
	if err := json.Unmarshal(b, &tmp); err != nil {
		return fmt.Errorf("parse user state json: %w", err)
	}

	for k, raw := range tmp {
		var id int64
		if _, err := fmt.Sscan(k, &id); err != nil {
			continue
		}

		var one string
		if err := json.Unmarshal(raw, &one); err == nil {
			if one != "" {
				s.data[id] = []string{one}
			}
			continue
		}

		var many []string
		if err := json.Unmarshal(raw, &many); err == nil {
			norm := normalizeCodes(many)
			if len(norm) > 0 {
				s.data[id] = norm
			}
		}
	}

	return nil
}

func (s *JSONStore) saveLocked() error {
	tmp := make(map[string][]string, len(s.data))
	for id, cities := range s.data {
		tmp[fmt.Sprintf("%d", id)] = append([]string(nil), cities...)
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

func normalizeCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}
