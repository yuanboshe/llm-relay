package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Upstream describes the single configured upstream provider for the current MVP.
type Upstream struct {
	BaseURL      string
	APIKeySource string
	APIKeyEnv    string
	APIKey       string
}

// Config contains relay runtime settings.
type Config struct {
	ListenAddr string
	Upstream   Upstream
}

// DefaultPaths returns the standard configuration paths under the user home directory.
func DefaultPaths() (Paths, error) {
	if home := os.Getenv("LLMRELAY_HOME"); home != "" {
		return Paths{
			Dir:        home,
			ConfigFile: filepath.Join(home, DefaultConfigName),
			TokenFile:  filepath.Join(home, DefaultStoreName),
		}, nil
	}

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
		ListenAddr: "127.0.0.1:18080",
		Upstream:   Upstream{},
	}
}

// Init creates the local configuration directory and initial files.
func Init(paths Paths, force bool) error {
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return err
	}

	if _, err := os.Stat(paths.ConfigFile); err == nil && !force {
		return fmt.Errorf("config already exists: %s", paths.ConfigFile)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := Save(paths.ConfigFile, DefaultConfig()); err != nil {
		return err
	}

	if _, err := os.Stat(paths.TokenFile); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(paths.TokenFile, []byte("[]\n"), 0o600)
}

// Load reads a config.toml file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config not found: %s; run llmrelay init", path)
		}
		return Config{}, err
	}

	cfg := DefaultConfig()
	section := ""
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("invalid config line: %s", line)
		}
		key = strings.TrimSpace(key)
		value, err := parseStringValue(strings.TrimSpace(value))
		if err != nil {
			return Config{}, err
		}

		switch section {
		case "":
			if key == "listen_addr" {
				cfg.ListenAddr = value
			}
		case "upstream":
			switch key {
			case "base_url":
				cfg.Upstream.BaseURL = value
			case "api_key_source":
				cfg.Upstream.APIKeySource = value
			case "api_key_env":
				cfg.Upstream.APIKeyEnv = value
			case "api_key":
				cfg.Upstream.APIKey = value
			}
		default:
			return Config{}, fmt.Errorf("unknown config section: %s", section)
		}
	}
	return cfg, nil
}

// Save writes a config.toml file.
func Save(path string, cfg Config) error {
	data := Format(cfg)
	return os.WriteFile(path, []byte(data), 0o600)
}

// Format returns config.toml text.
func Format(cfg Config) string {
	return fmt.Sprintf(`listen_addr = "%s"

[upstream]
base_url = "%s"
api_key_source = "%s"
api_key_env = "%s"
api_key = "%s"
`, escape(cfg.ListenAddr), escape(cfg.Upstream.BaseURL), escape(cfg.Upstream.APIKeySource), escape(cfg.Upstream.APIKeyEnv), escape(cfg.Upstream.APIKey))
}

// FormatRedacted returns config.toml text with inline secrets hidden.
func FormatRedacted(cfg Config) string {
	if cfg.Upstream.APIKey != "" {
		cfg.Upstream.APIKey = "<redacted>"
	}
	return Format(cfg)
}

func parseStringValue(value string) (string, error) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", errors.New("only quoted string config values are supported")
	}
	value = strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\\`, `\`)
	return value, nil
}

func escape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
