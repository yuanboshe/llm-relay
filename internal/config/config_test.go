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
	if filepath.Base(paths.PIDFile) != DefaultPIDName {
		t.Fatalf("PIDFile = %q, want suffix %q", paths.PIDFile, DefaultPIDName)
	}
	if filepath.Base(paths.LogFile) != DefaultLogName {
		t.Fatalf("LogFile = %q, want suffix %q", paths.LogFile, DefaultLogName)
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
	if paths.PIDFile != filepath.Join(home, DefaultPIDName) {
		t.Fatalf("PIDFile = %q, want pid file under env home", paths.PIDFile)
	}
	if paths.LogFile != filepath.Join(home, DefaultLogName) {
		t.Fatalf("LogFile = %q, want log file under env home", paths.LogFile)
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
	if cfg.Tunnel.RemoteHost != "127.0.0.1" {
		t.Fatalf("Tunnel.RemoteHost = %q, want default localhost", cfg.Tunnel.RemoteHost)
	}
	if cfg.Tunnel.RemotePort != "18080" {
		t.Fatalf("Tunnel.RemotePort = %q, want default remote port", cfg.Tunnel.RemotePort)
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
	if strings.Contains(configText, "public_url") {
		t.Fatalf("config = %q, want no public_url", configText)
	}
	if !strings.Contains(configText, "[upstream]") {
		t.Fatalf("config = %q, want upstream section", configText)
	}
	if !strings.Contains(configText, "[tunnel]") {
		t.Fatalf("config = %q, want tunnel section", configText)
	}
	if !strings.Contains(configText, `enabled = false`) {
		t.Fatalf("config = %q, want disabled tunnel by default", configText)
	}

	tokenData, err := os.ReadFile(paths.TokenFile)
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	if strings.TrimSpace(string(tokenData)) != "[]" {
		t.Fatalf("tokens = %q, want empty json array", string(tokenData))
	}
}

func TestPublicURLIsNotKnownConfigKey(t *testing.T) {
	if IsKnownKey("public_url") {
		t.Fatal("public_url is known, want unsupported config key")
	}
}

func TestLoadIgnoresLegacyPublicURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultConfigName)
	text := `listen_addr = "127.0.0.1:18080"
public_url = "https://relay.example.test"

[upstream]
base_url = "https://api.example.test/v1"
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if _, ok := cfg.Extra["public_url"]; ok {
		t.Fatalf("Extra contains legacy public_url: %#v", cfg.Extra)
	}
	formatted := Format(cfg)
	if strings.Contains(formatted, "public_url") {
		t.Fatalf("Format = %q, want no public_url", formatted)
	}
}

func TestDefaultConfigNameUsesTOMLSuffix(t *testing.T) {
	if DefaultConfigName != "config.toml" {
		t.Fatalf("DefaultConfigName = %q, want config.toml", DefaultConfigName)
	}
}

func TestLoadParsesTunnelSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultConfigName)
	text := `listen_addr = "0.0.0.0:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "env"
api_key_env = "LLMRELAY_TEST_KEY"
api_key = ""

[tunnel]
enabled = true
ssh_host = "relay-host"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Tunnel.Enabled {
		t.Fatal("Tunnel.Enabled = false, want true")
	}
	if cfg.Tunnel.SSHHost != "relay-host" {
		t.Fatalf("Tunnel.SSHHost = %q, want relay-host", cfg.Tunnel.SSHHost)
	}
	if cfg.Tunnel.SSHUser != "ubuntu" {
		t.Fatalf("Tunnel.SSHUser = %q, want ubuntu", cfg.Tunnel.SSHUser)
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
