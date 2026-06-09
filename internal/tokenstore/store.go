package tokenstore

import (
	"encoding/json"
	"os"
	"time"
)

// Record describes a locally managed relay credential entry.
type Record struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	Enabled   bool      `json:"enabled"`
}

// Store persists relay credential metadata in a local JSON file.
type Store struct {
	path string
}

// New creates a JSON-backed store.
func New(path string) *Store {
	return &Store{path: path}
}

// Load reads all records. A missing file is treated as an empty store.
func (s *Store) Load() ([]Record, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, err
	}

	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// Save writes all records to the store path.
func (s *Store) Save(records []Record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}
