package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	configDirName  = ".llmrelay"
	configFileName = "config.json"
	pidFileName    = "relay.pid"
	logFileName    = "relay.log"
)

type Config struct {
	ListenAddr string       `json:"listen_addr"`
	Upstream   Upstream     `json:"upstream"`
	Tokens     []RelayToken `json:"relay_tokens"`
}

type Upstream struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

type RelayToken struct {
	ID        string `json:"id"`
	Hash      string `json:"hash"`
	CreatedAt string `json:"created_at"`
}

func defaultConfig() Config {
	return Config{
		ListenAddr: "127.0.0.1:11434",
		Upstream: Upstream{
			Provider: "openai",
			BaseURL:  "https://api.openai.com",
		},
		Tokens: []RelayToken{},
	}
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func pidPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pidFileName), nil
}

func logPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, logFileName), nil
}

func EnsureConfig() (Config, error) {
	p, err := configPath()
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(p); err == nil {
		return LoadConfig()
	}
	cfg := defaultConfig()
	if err := SaveConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadConfig() (Config, error) {
	p, err := configPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultConfig().ListenAddr
	}
	if cfg.Upstream.Provider == "" {
		cfg.Upstream.Provider = "openai"
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	p, err := configPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o600)
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func AddToken(cfg *Config, token string) error {
	now := time.Now().UTC()
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		return err
	}
	id := "tok_" + hex.EncodeToString(rb)
	cfg.Tokens = append(cfg.Tokens, RelayToken{
		ID:        id,
		Hash:      HashToken(token),
		CreatedAt: now.Format(time.RFC3339),
	})
	return nil
}

func RemoveTokenByID(cfg *Config, id string) error {
	for i, t := range cfg.Tokens {
		if t.ID == id {
			cfg.Tokens = append(cfg.Tokens[:i], cfg.Tokens[i+1:]...)
			return nil
		}
	}
	return errors.New("token id not found")
}

func HasToken(cfg Config, token string) bool {
	h := HashToken(token)
	for _, t := range cfg.Tokens {
		if t.Hash == h {
			return true
		}
	}
	return false
}

func TokenSummaries(cfg Config) []RelayToken {
	out := append([]RelayToken(nil), cfg.Tokens...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}
