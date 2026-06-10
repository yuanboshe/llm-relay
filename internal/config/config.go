package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultDirName    = ".llmrelay"
	DefaultConfigName = "config.toml"
	DefaultStoreName  = "tokens.json"
	DefaultPIDName    = "llmrelay.pid"
	DefaultLogName    = "llmrelay.log"
)

// Paths contains the default local files used by llm-relay.
type Paths struct {
	Dir        string
	ConfigFile string
	TokenFile  string
	PIDFile    string
	LogFile    string
}

// Upstream describes the single configured upstream provider for the current MVP.
type Upstream struct {
	BaseURL      string
	APIKeySource string
	APIKeyEnv    string
	APIKey       string
}

// Tunnel describes an optional SSH reverse tunnel managed by llmrelay.
type Tunnel struct {
	Enabled    bool
	SSHHost    string
	SSHUser    string
	SSHPort    string
	RemoteHost string
	RemotePort string
}

// Config contains relay runtime settings.
type Config struct {
	ListenAddr string
	PublicURL  string
	Upstream   Upstream
	Tunnel     Tunnel
	Extra      map[string]any
}

// DefaultPaths returns the standard configuration paths under the user home directory.
func DefaultPaths() (Paths, error) {
	if home := os.Getenv("LLMRELAY_HOME"); home != "" {
		return Paths{
			Dir:        home,
			ConfigFile: filepath.Join(home, DefaultConfigName),
			TokenFile:  filepath.Join(home, DefaultStoreName),
			PIDFile:    filepath.Join(home, DefaultPIDName),
			LogFile:    filepath.Join(home, DefaultLogName),
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
		PIDFile:    filepath.Join(dir, DefaultPIDName),
		LogFile:    filepath.Join(dir, DefaultLogName),
	}, nil
}

// DefaultConfig returns a conservative local configuration skeleton.
func DefaultConfig() Config {
	return Config{
		ListenAddr: "127.0.0.1:18080",
		PublicURL:  "",
		Upstream:   Upstream{},
		Tunnel: Tunnel{
			Enabled:    false,
			SSHPort:    "22",
			RemoteHost: "127.0.0.1",
			RemotePort: "18080",
		},
	}
}

// Ensure creates the local configuration directory and missing initial files.
func Ensure(paths Paths) (bool, bool, error) {
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return false, false, err
	}

	configCreated := false
	if _, err := os.Stat(paths.ConfigFile); err == nil {
		// Preserve existing configuration.
	} else if err != nil && !os.IsNotExist(err) {
		return false, false, err
	} else {
		if err := Save(paths.ConfigFile, DefaultConfig()); err != nil {
			return false, false, err
		}
		configCreated = true
	}

	tokenCreated := false
	if _, err := os.Stat(paths.TokenFile); err == nil {
		return configCreated, tokenCreated, nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, false, err
	}
	if err := os.WriteFile(paths.TokenFile, []byte("[]\n"), 0o600); err != nil {
		return false, false, err
	}
	tokenCreated = true
	return configCreated, tokenCreated, nil
}

// Init creates the local configuration directory and initial files.
func Init(paths Paths, force bool) error {
	if force {
		if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
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
	configCreated, _, err := Ensure(paths)
	if err != nil {
		return err
	}
	if !configCreated {
		return fmt.Errorf("config already exists: %s", paths.ConfigFile)
	}
	return nil
}

// Load reads a config.toml file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config not found: %s; run llmrelay install", path)
		}
		return Config{}, err
	}

	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return Config{}, err
	}

	cfg := DefaultConfig()
	cfg.Extra = map[string]any{}
	cfg.ListenAddr = getString(doc, "listen_addr", cfg.ListenAddr)
	cfg.PublicURL = getString(doc, "public_url", cfg.PublicURL)
	if upstream, ok := doc["upstream"].(map[string]any); ok {
		cfg.Upstream.BaseURL = getString(upstream, "base_url", cfg.Upstream.BaseURL)
		cfg.Upstream.APIKeySource = getString(upstream, "api_key_source", cfg.Upstream.APIKeySource)
		cfg.Upstream.APIKeyEnv = getString(upstream, "api_key_env", cfg.Upstream.APIKeyEnv)
		cfg.Upstream.APIKey = getString(upstream, "api_key", cfg.Upstream.APIKey)
	}
	if tunnel, ok := doc["tunnel"].(map[string]any); ok {
		cfg.Tunnel.Enabled = getBool(tunnel, "enabled", cfg.Tunnel.Enabled)
		cfg.Tunnel.SSHHost = getString(tunnel, "ssh_host", cfg.Tunnel.SSHHost)
		cfg.Tunnel.SSHUser = getString(tunnel, "ssh_user", cfg.Tunnel.SSHUser)
		cfg.Tunnel.SSHPort = getString(tunnel, "ssh_port", cfg.Tunnel.SSHPort)
		cfg.Tunnel.RemoteHost = getString(tunnel, "remote_host", cfg.Tunnel.RemoteHost)
		cfg.Tunnel.RemotePort = getString(tunnel, "remote_port", cfg.Tunnel.RemotePort)
	}
	for key, value := range flattenMap("", doc) {
		if !IsKnownKey(key) {
			cfg.Extra[key] = value
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
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "listen_addr = \"%s\"\n", escape(cfg.ListenAddr))
	_, _ = fmt.Fprintf(&b, "public_url = \"%s\"\n\n", escape(cfg.PublicURL))
	_, _ = fmt.Fprintf(&b, "[upstream]\n")
	_, _ = fmt.Fprintf(&b, "base_url = \"%s\"\n", escape(cfg.Upstream.BaseURL))
	_, _ = fmt.Fprintf(&b, "api_key_source = \"%s\"\n", escape(cfg.Upstream.APIKeySource))
	_, _ = fmt.Fprintf(&b, "api_key_env = \"%s\"\n", escape(cfg.Upstream.APIKeyEnv))
	_, _ = fmt.Fprintf(&b, "api_key = \"%s\"\n\n", escape(cfg.Upstream.APIKey))
	_, _ = fmt.Fprintf(&b, "[tunnel]\n")
	_, _ = fmt.Fprintf(&b, "enabled = %t\n", cfg.Tunnel.Enabled)
	_, _ = fmt.Fprintf(&b, "ssh_host = \"%s\"\n", escape(cfg.Tunnel.SSHHost))
	_, _ = fmt.Fprintf(&b, "ssh_user = \"%s\"\n", escape(cfg.Tunnel.SSHUser))
	_, _ = fmt.Fprintf(&b, "ssh_port = \"%s\"\n", escape(cfg.Tunnel.SSHPort))
	_, _ = fmt.Fprintf(&b, "remote_host = \"%s\"\n", escape(cfg.Tunnel.RemoteHost))
	_, _ = fmt.Fprintf(&b, "remote_port = \"%s\"\n", escape(cfg.Tunnel.RemotePort))
	extraBySection := map[string]map[string]any{}
	for key, value := range cfg.Extra {
		if !IsKnownKey(key) {
			section, name := splitExtraKey(key)
			if extraBySection[section] == nil {
				extraBySection[section] = map[string]any{}
			}
			extraBySection[section][name] = value
		}
	}
	sections := make([]string, 0, len(extraBySection))
	for section := range extraBySection {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	for _, section := range sections {
		_, _ = fmt.Fprintf(&b, "\n[%s]\n", section)
		names := make([]string, 0, len(extraBySection[section]))
		for name := range extraBySection[section] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			_, _ = fmt.Fprintf(&b, "%s = %s\n", name, formatExtraValue(extraBySection[section][name]))
		}
	}
	return b.String()
}

// FormatRedacted returns config.toml text with inline secrets hidden.
func FormatRedacted(cfg Config) string {
	if cfg.Upstream.APIKey != "" {
		cfg.Upstream.APIKey = "<redacted>"
	}
	return Format(cfg)
}

// IsKnownKey reports whether a dotted key is consumed by the runtime.
func IsKnownKey(key string) bool {
	switch key {
	case "listen_addr",
		"public_url",
		"upstream.base_url",
		"upstream.api_key_source",
		"upstream.api_key_env",
		"upstream.api_key",
		"tunnel.enabled",
		"tunnel.ssh_host",
		"tunnel.ssh_user",
		"tunnel.ssh_port",
		"tunnel.remote_host",
		"tunnel.remote_port":
		return true
	default:
		return false
	}
}

// UnknownKeys returns preserved config keys that are not consumed by the runtime.
func UnknownKeys(cfg Config) []string {
	keys := make([]string, 0, len(cfg.Extra))
	for key := range cfg.Extra {
		if !IsKnownKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func getString(doc map[string]any, key string, fallback string) string {
	value, ok := doc[key].(string)
	if !ok {
		return fallback
	}
	return value
}

func getBool(doc map[string]any, key string, fallback bool) bool {
	value, ok := doc[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func flattenMap(prefix string, doc map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range doc {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			for nestedKey, nestedValue := range flattenMap(fullKey, nested) {
				result[nestedKey] = nestedValue
			}
			continue
		}
		result[fullKey] = value
	}
	return result
}

func splitExtraKey(key string) (string, string) {
	section, name, ok := strings.Cut(key, ".")
	if !ok {
		return "extra", key
	}
	remaining := name
	for {
		nextSection, nextName, ok := strings.Cut(remaining, ".")
		if !ok {
			return section, remaining
		}
		section += "." + nextSection
		remaining = nextName
	}
}

func formatExtraValue(value any) string {
	switch typed := value.(type) {
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case string:
		return `"` + escape(typed) + `"`
	default:
		return `"` + escape(fmt.Sprint(typed)) + `"`
	}
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

func parseBoolValue(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("only true or false boolean config values are supported")
	}
}

func escape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
