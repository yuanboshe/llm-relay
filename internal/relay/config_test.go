package relay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddTokenStoresHashOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := EnsureConfig()
	if err != nil {
		t.Fatal(err)
	}
	plain := "llmr_test_plain_token"
	if err := AddToken(&cfg, plain); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(home, ".llmrelay", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), plain) {
		t.Fatalf("config must not contain plaintext relay token")
	}

	var loaded Config
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Tokens) != 1 {
		t.Fatalf("expected one token, got %d", len(loaded.Tokens))
	}
	if len(loaded.Tokens[0].Hash) != 64 {
		t.Fatalf("expected sha256 hash length 64, got %d", len(loaded.Tokens[0].Hash))
	}
}
