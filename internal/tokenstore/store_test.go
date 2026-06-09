package tokenstore

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFile(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "tokens.json"))

	records, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records length = %d, want 0", len(records))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := New(path)
	now := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	want := []Record{
		{
			KeyID:     "example-id",
			TokenHash: "sha256:example",
			CreatedAt: now.Format(time.RFC3339),
			Enabled:   true,
		},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("records length = %d, want 1", len(got))
	}
	if got[0].KeyID != want[0].KeyID || got[0].TokenHash != want[0].TokenHash || !got[0].Enabled {
		t.Fatalf("record = %+v, want %+v", got[0], want[0])
	}
}

func TestGenerateTokenAndHashToken(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if len(token) <= len("llmr_") || token[:len("llmr_")] != "llmr_" {
		t.Fatalf("token = %q, want llmr_ prefix", token)
	}

	hash := HashToken(token)
	if len(hash) != len("sha256:")+64 {
		t.Fatalf("hash = %q, want sha256 hex", hash)
	}
	if hash == token {
		t.Fatal("hash equals plaintext token")
	}
}
