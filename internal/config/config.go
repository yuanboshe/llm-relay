package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultDirName    = ".llmrelay"
	DefaultConfigName = "config.toml"
	DefaultStoreName  = "tokens.json"
)

// Paths contains the default local files used by llm-relay.
type Paths struct {
	Dir        string
	ConfigFile string
	TokenFile  string
}

// Provider describes an upstream LLM API provider.
type Provider struct {
	Name    string
	Kind    string
	BaseURL string
	APIKey  string
}

// Config contains relay runtime settings.
type Config struct {
	ListenAddr string
	Providers  []Provider
}

// DefaultPaths returns the standard configuration paths under the user home directory.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	dir := filepath.Join(home, DefaultDirName)
	return Paths{
		Dir:        dir,
		ConfigFile: filepath.Join(dir, DefaultConfigName),
		TokenFile:  filepath.Join(dir, DefaultStoreName),
	}, nil
}

// DefaultConfig returns a conservative local configuration skeleton.
func DefaultConfig() Config {
	return Config{
		ListenAddr: "127.0.0.1:8080",
		Providers:  []Provider{},
	}
}
