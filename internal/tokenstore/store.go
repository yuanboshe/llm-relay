package tokenstore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record describes a locally managed relay credential entry.
type Record struct {
	KeyID     string `json:"key_id"`
	TokenHash string `json:"token_hash"`
	CreatedAt string `json:"created_at"`
	RotatedAt string `json:"rotated_at"`
	Enabled   bool   `json:"enabled"`
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
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}

// GenerateToken creates a new plaintext relay token. Callers must display it once only.
func GenerateToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "llmr_" + base64.RawURLEncoding.EncodeToString(random), nil
}

// HashToken returns the persisted SHA-256 token hash.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// NewRecord creates an enabled token record for a plaintext token.
func NewRecord(keyID string, token string, now time.Time) Record {
	return Record{
		KeyID:     keyID,
		TokenHash: HashToken(token),
		CreatedAt: now.UTC().Format(time.RFC3339),
		Enabled:   true,
	}
}

// Find returns the index and record for keyID.
func Find(records []Record, keyID string) (int, Record, error) {
	for i, record := range records {
		if record.KeyID == keyID {
			return i, record, nil
		}
	}
	return -1, Record{}, fmt.Errorf("token not found: %s", keyID)
}
