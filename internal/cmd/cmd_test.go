package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "dev" {
		t.Fatalf("version output = %q, want dev", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"missing"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(missing) returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
	if !strings.Contains(stderr.String(), "llmrelay") {
		t.Fatalf("stderr = %q, want llmrelay command name", stderr.String())
	}
}

func TestRunServeDryOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"serve", "--addr", "127.0.0.1:9090"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(serve) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "127.0.0.1:9090") {
		t.Fatalf("serve output = %q, want address", out)
	}
	if !strings.Contains(out, "/v1/chat/completions") {
		t.Fatalf("serve output = %q, want OpenAI route", out)
	}
}

func TestRunHelpUsesFinalCommandName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"help"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(help) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "llmrelay") {
		t.Fatalf("help output = %q, want llmrelay command name", out)
	}
	if strings.Contains(out, "llm-relay <command>") {
		t.Fatalf("help output = %q, still contains old command usage", out)
	}
}

func TestRunCompletionBash(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"completion", "bash"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(completion bash) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "llmrelay") {
		t.Fatalf("completion output = %q, want llmrelay references", out)
	}
}

func TestRunInitCreatesConfigInEnvHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"init"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(init) returned error: %v", err)
	}

	configPath := filepath.Join(home, "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	tokenPath := filepath.Join(home, "tokens.json")
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("token file not created: %v", err)
	}
}

func TestRunConfigShowRedactsInlineAPIKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	keyValue := strings.Join([]string{"test", "secret", "value"}, "-")
	configText := fmt.Sprintf(`listen_addr = "127.0.0.1:18080"

[upstream]
base_url = "https://api.example.test"
api_key_source = "inline"
api_key_env = ""
api_key = "%s"
`, keyValue)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run([]string{"config", "show"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(config show) returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("config show leaked api key: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("config show = %q, want redacted marker", out)
	}
}

func TestRunTokenCreateStoresOnlyHash(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"init"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(init) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()

	if err := Run([]string{"token", "create", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token create) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "llmr_") {
		t.Fatalf("token create output = %q, want one-time relay token", out)
	}
	tokenFile := filepath.Join(home, "tokens.json")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	if strings.Contains(string(data), "llmr_") {
		t.Fatalf("token store leaked plaintext token: %q", string(data))
	}
	if !strings.Contains(string(data), "sha256:") {
		t.Fatalf("token store = %q, want sha256 hash", string(data))
	}
}

func TestRunTokenLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"init"},
		{"token", "create", "local"},
		{"token", "disable", "local"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "inspect", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token inspect) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "enabled: false") {
		t.Fatalf("inspect disabled output = %q, want disabled state", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "enable", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token enable) returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "rotate", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token rotate) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "llmr_") {
		t.Fatalf("rotate output = %q, want one-time relay token", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "delete", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token delete) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token list) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "no tokens") {
		t.Fatalf("list output = %q, want no tokens", stdout.String())
	}
}

func TestRunUpstreamSetURLAndShow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"init"},
		{"upstream", "set-url", "https://api.example.test/v1/"},
		{"upstream", "show"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}

	out := stdout.String()
	if !strings.Contains(out, `base_url = "https://api.example.test/v1"`) {
		t.Fatalf("upstream show = %q, want normalized base URL", out)
	}
}

func TestRunUpstreamSetKeyStdinRedactsShow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	keyValue := strings.Join([]string{"runtime", "key", "value"}, "-")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"init"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(init) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := RunWithIO([]string{"upstream", "set-key", "--stdin"}, strings.NewReader(keyValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(upstream set-key --stdin) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"upstream", "show"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(upstream show) returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("upstream show leaked key: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("upstream show = %q, want redacted key", out)
	}
}

func TestRunUpstreamTestUsesKeyWithoutPrintingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	keyValue := strings.Join([]string{"runtime", "test", "key"}, "-")
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	commands := [][]string{
		{"init"},
		{"upstream", "set-url", server.URL},
	}
	for _, args := range commands {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}
	stdout.Reset()
	stderr.Reset()
	if err := RunWithIO([]string{"upstream", "set-key", "--stdin"}, strings.NewReader(keyValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(upstream set-key --stdin) returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"upstream", "test"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(upstream test) returned error: %v", err)
	}

	if seenAuth != "Bearer "+keyValue {
		t.Fatalf("Authorization = %q, want bearer key", seenAuth)
	}
	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("upstream test leaked key: %q", out)
	}
	if !strings.Contains(out, "status: 200") {
		t.Fatalf("upstream test output = %q, want status", out)
	}
}
