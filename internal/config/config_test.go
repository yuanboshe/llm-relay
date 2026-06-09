package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPaths(t *testing.T) {
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths returned error: %v", err)
	}

	if filepath.Base(paths.Dir) != DefaultDirName {
		t.Fatalf("Dir = %q, want suffix %q", paths.Dir, DefaultDirName)
	}
	if filepath.Base(paths.ConfigFile) != DefaultConfigName {
		t.Fatalf("ConfigFile = %q, want suffix %q", paths.ConfigFile, DefaultConfigName)
	}
	if filepath.Base(paths.TokenFile) != DefaultStoreName {
		t.Fatalf("TokenFile = %q, want suffix %q", paths.TokenFile, DefaultStoreName)
	}
}

func TestDefaultPathsHonorsLLMRelayHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths returned error: %v", err)
	}

	if paths.Dir != home {
		t.Fatalf("Dir = %q, want env home %q", paths.Dir, home)
	}
	if paths.ConfigFile != filepath.Join(home, DefaultConfigName) {
		t.Fatalf("ConfigFile = %q, want config under env home", paths.ConfigFile)
	}
	if paths.TokenFile != filepath.Join(home, DefaultStoreName) {
		t.Fatalf("TokenFile = %q, want token store under env home", paths.TokenFile)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != "127.0.0.1:18080" {
		t.Fatalf("ListenAddr = %q, want default local address", cfg.ListenAddr)
	}
	if cfg.Upstream.BaseURL != "" {
		t.Fatalf("Upstream.BaseURL = %q, want empty", cfg.Upstream.BaseURL)
	}
}

func TestInitCreatesConfigAndTokenFiles(t *testing.T) {
	paths := Paths{
		Dir:        t.TempDir(),
		ConfigFile: filepath.Join(t.TempDir(), DefaultConfigName),
		TokenFile:  filepath.Join(t.TempDir(), DefaultStoreName),
	}
	paths.ConfigFile = filepath.Join(paths.Dir, DefaultConfigName)
	paths.TokenFile = filepath.Join(paths.Dir, DefaultStoreName)

	if err := Init(paths, false); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	configData, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, `listen_addr = "127.0.0.1:18080"`) {
		t.Fatalf("config = %q, want default listen addr", configText)
	}
	if !strings.Contains(configText, "[upstream]") {
		t.Fatalf("config = %q, want upstream section", configText)
	}

	tokenData, err := os.ReadFile(paths.TokenFile)
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	if strings.TrimSpace(string(tokenData)) != "[]" {
		t.Fatalf("tokens = %q, want empty json array", string(tokenData))
	}
}

func TestInitRefusesExistingConfigWithoutForce(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		Dir:        dir,
		ConfigFile: filepath.Join(dir, DefaultConfigName),
		TokenFile:  filepath.Join(dir, DefaultStoreName),
	}
	if err := os.WriteFile(paths.ConfigFile, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	if err := Init(paths, false); err == nil {
		t.Fatal("Init returned nil error, want existing config error")
	}
}
