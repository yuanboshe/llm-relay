package config

import (
	"path/filepath"
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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Fatalf("ListenAddr = %q, want default local address", cfg.ListenAddr)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("Providers length = %d, want 0", len(cfg.Providers))
	}
}
