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
			ID:        "example-id",
			Label:     "local development",
			CreatedAt: now,
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
	if got[0].ID != want[0].ID || got[0].Label != want[0].Label || !got[0].Enabled {
		t.Fatalf("record = %+v, want %+v", got[0], want[0])
	}
}
