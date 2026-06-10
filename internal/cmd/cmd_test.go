package cmd

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setInstallTestHome(t *testing.T) string {
	t.Helper()
	installHome := t.TempDir()
	t.Setenv("LLMRELAY_INSTALL_HOME", installHome)
	t.Setenv("LLMRELAY_ZSHRC", filepath.Join(t.TempDir(), ".zshrc"))
	return installHome
}

type fakeExternalRunner struct {
	calls []string
	err   error
}

func (r *fakeExternalRunner) Run(name string, args ...string) (string, error) {
	r.calls = append(r.calls, strings.Join(append([]string{name}, args...), " "))
	return "", r.err
}

func setSetupExternalTestRunner(t *testing.T, runner *fakeExternalRunner, goos string) {
	t.Helper()
	oldRunner := setupExternalRunner
	oldGOOS := setupGOOS
	setupExternalRunner = runner
	setupGOOS = func() string { return goos }
	t.Cleanup(func() {
		setupExternalRunner = oldRunner
		setupGOOS = oldGOOS
	})
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "v0.1.0" {
		t.Fatalf("version output = %q, want v0.1.0", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionStringIncludesBuildMetadata(t *testing.T) {
	if got := versionString("v0.1.0", "abc123", "2026-06-10T00:00:00Z"); got != "v0.1.0\ncommit: abc123\ndate: 2026-06-10T00:00:00Z" {
		t.Fatalf("versionString = %q, want metadata", got)
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

func TestRunServeRequiresConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"serve"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(serve) returned nil error, want missing config error")
	}
	if !strings.Contains(err.Error(), "run llmrelay install") {
		t.Fatalf("error = %q, want install hint", err.Error())
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
	for _, want := range []string{"install", "setup", "start", "stop", "restart", "status", "logs"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output = %q, want %q command", out, want)
		}
	}
	for _, removed := range []string{"init", "upstream"} {
		if strings.Contains(out, removed) {
			t.Fatalf("help output = %q, should not show removed command %q", out, removed)
		}
	}
}

func TestRunStatusReportsStopped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(status) returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "state: stopped") {
		t.Fatalf("status output = %q, want stopped state", out)
	}
	if !strings.Contains(out, filepath.Join(home, "llmrelay.pid")) {
		t.Fatalf("status output = %q, want pid path", out)
	}
	if !strings.Contains(out, filepath.Join(home, "llmrelay.log")) {
		t.Fatalf("status output = %q, want log path", out)
	}
}

func TestRunStatusReportsConfiguredTunnel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	configText := `listen_addr = "127.0.0.1:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = ""
api_key_env = ""
api_key = ""

[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "28080"
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"status"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(status) returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "ssh-tunnel: enabled") {
		t.Fatalf("status output = %q, want ssh tunnel enabled", out)
	}
	if !strings.Contains(out, "ssh-tunnel-remote: 127.0.0.1:28080") {
		t.Fatalf("status output = %q, want ssh tunnel remote", out)
	}
}

func TestRunLogsReadsTail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	logPath := filepath.Join(home, "llmrelay.log")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"logs", "--tail", "2"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(logs --tail 2) returned error: %v", err)
	}
	if stdout.String() != "two\nthree\n" {
		t.Fatalf("logs output = %q, want last two lines", stdout.String())
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

func TestRunInstallCreatesConfigInEnvHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	installHome := setInstallTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"install"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(install) returned error: %v", err)
	}

	configPath := filepath.Join(home, "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	tokenPath := filepath.Join(home, "tokens.json")
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("token file not created: %v", err)
	}
	installed := filepath.Join(installHome, "Library", "Application Support", "llmrelay", "bin", "llmrelay")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("installed binary not created at final command name: %v", err)
	}
	if !strings.Contains(stdout.String(), "installed: "+installed) {
		t.Fatalf("install output = %q, want installed path", stdout.String())
	}
}

func TestRunInitIsRemoved(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"init"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(init) returned nil error, want removed command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
}

func TestRunConfigPathIsRemoved(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"config", "path"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(config path) returned nil error, want removed command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
}

func TestRunSetupConfiguresUpstreamAndCreatesToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	keyValue := strings.Join([]string{"setup", "secret", "value"}, "-")
	input := strings.Join([]string{
		"https://api.example.test/v1/",
		keyValue,
		"local",
		"none",
	}, "\n") + "\n"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := RunWithIO([]string{"setup"}, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("Run(setup) returned error: %v", err)
	}

	configData, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	configText := string(configData)
	if !strings.Contains(configText, `base_url = "https://api.example.test/v1"`) {
		t.Fatalf("config = %q, want normalized base URL", configText)
	}
	if !strings.Contains(configText, `api_key_source = "inline"`) {
		t.Fatalf("config = %q, want inline key source", configText)
	}
	if !strings.Contains(configText, keyValue) {
		t.Fatalf("config = %q, want configured key", configText)
	}
	tokenData, err := os.ReadFile(filepath.Join(home, "tokens.json"))
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	if !strings.Contains(string(tokenData), `"key_id": "local"`) {
		t.Fatalf("tokens = %q, want local token", string(tokenData))
	}
	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("setup output leaked upstream key: %q", out)
	}
	if !strings.Contains(out, "relay token: llmr_") {
		t.Fatalf("setup output = %q, want relay token", out)
	}
}

func TestRunSetupInstallsCloudflaredOnMacOS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	runner := &fakeExternalRunner{}
	setSetupExternalTestRunner(t, runner, "darwin")
	keyValue := strings.Join([]string{"setup", "secret", "value"}, "-")
	tunnelToken := strings.Join([]string{"cloudflare", "secret", "token"}, "-")
	input := strings.Join([]string{
		"https://api.example.test/v1/",
		keyValue,
		"local",
		"",
		tunnelToken,
	}, "\n") + "\n"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := RunWithIO([]string{"setup"}, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("Run(setup) returned error: %v", err)
	}

	wantCalls := []string{
		"brew install cloudflared",
		"sudo cloudflared service install " + tunnelToken,
		"sudo launchctl start com.cloudflare.cloudflared",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("runner calls = %#v, want %#v", runner.calls, wantCalls)
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, keyValue) {
		t.Fatalf("setup leaked upstream key: %q", combined)
	}
	if strings.Contains(combined, tunnelToken) {
		t.Fatalf("setup leaked Cloudflare tunnel token: %q", combined)
	}
	configData, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(configData), `enabled = false`) {
		t.Fatalf("config = %q, want ssh tunnel disabled for Cloudflare flow", string(configData))
	}
}

func TestRunSetupRejectsCloudflaredAutoInstallOutsideMacOS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	runner := &fakeExternalRunner{}
	setSetupExternalTestRunner(t, runner, "linux")
	input := strings.Join([]string{
		"https://api.example.test/v1/",
		"secret",
		"local",
		"cloudflare",
		"cloudflare-secret-token",
	}, "\n") + "\n"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := RunWithIO([]string{"setup"}, strings.NewReader(input), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(setup) returned nil error, want non-macOS cloudflared error")
	}
	if !strings.Contains(err.Error(), "cloudflared service auto-install is only supported on macOS") {
		t.Fatalf("error = %q, want macOS support error", err.Error())
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v, want none", runner.calls)
	}
}

func TestRunSetupConfiguresSSHTunnelOnlyWhenSelected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	runner := &fakeExternalRunner{}
	setSetupExternalTestRunner(t, runner, "darwin")
	input := strings.Join([]string{
		"https://api.example.test/v1/",
		"secret",
		"local",
		"ssh",
		"relay-server",
		"ubuntu",
		"2222",
		"127.0.0.1",
		"28080",
	}, "\n") + "\n"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := RunWithIO([]string{"setup"}, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("Run(setup) returned error: %v", err)
	}

	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v, want no cloudflared commands", runner.calls)
	}
	configData, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	configText := string(configData)
	for _, want := range []string{
		`enabled = true`,
		`ssh_host = "relay-server"`,
		`ssh_user = "ubuntu"`,
		`ssh_port = "2222"`,
		`remote_host = "127.0.0.1"`,
		`remote_port = "28080"`,
	} {
		if !strings.Contains(configText, want) {
			t.Fatalf("config = %q, want %q", configText, want)
		}
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

func TestRunTokenCreateStoresPlaintextToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"install"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(install) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()

	if err := Run([]string{"token", "create", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token create) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "llmr_") {
		t.Fatalf("token create output = %q, want relay token", out)
	}
	tokenFile := filepath.Join(home, "tokens.json")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	if !strings.Contains(string(data), `"token": "llmr_`) {
		t.Fatalf("token store = %q, want plaintext token", string(data))
	}
	if !strings.Contains(string(data), "sha256:") {
		t.Fatalf("token store = %q, want sha256 hash", string(data))
	}
}

func TestRunTokenLifecycle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
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
	rotatedToken := stdout.String()
	if !strings.Contains(rotatedToken, "llmr_") {
		t.Fatalf("rotate output = %q, want relay token", stdout.String())
	}
	tokenData, err := os.ReadFile(filepath.Join(home, "tokens.json"))
	if err != nil {
		t.Fatalf("read token store: %v", err)
	}
	rotatedToken = strings.TrimSpace(strings.TrimPrefix(strings.Split(rotatedToken, "relay token: ")[1], "\n"))
	if !strings.Contains(string(tokenData), rotatedToken) {
		t.Fatalf("token store = %q, want rotated plaintext token", string(tokenData))
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

func TestRunTokenMetadataAndVerify(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	nameValue := strings.Join([]string{"example", "token", "name"}, " ")
	noteValue := strings.Join([]string{"example", "token", "note"}, " ")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"token", "create", "local", "--name", nameValue, "--note", noteValue},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}
	out := stdout.String()
	if !strings.Contains(out, "relay token:") {
		t.Fatalf("create output = %q, want relay token", out)
	}
	lines := strings.Split(out, "\n")
	var tokenValue string
	for _, line := range lines {
		if strings.HasPrefix(line, "relay token: ") {
			tokenValue = strings.TrimSpace(strings.TrimPrefix(line, "relay token: "))
		}
	}
	if tokenValue == "" {
		t.Fatalf("create output = %q, want token value", out)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "inspect", "local"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token inspect) returned error: %v", err)
	}
	inspectOut := stdout.String()
	for _, want := range []string{
		"key-id: local",
		"name: " + nameValue,
		"note: " + noteValue,
		"enabled: true",
		"token-hash-prefix: sha256:",
	} {
		if !strings.Contains(inspectOut, want) {
			t.Fatalf("inspect output = %q, want %q", inspectOut, want)
		}
	}
	if strings.Contains(inspectOut, tokenValue) {
		t.Fatalf("inspect leaked token: %q", inspectOut)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"token", "list"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(token list) returned error: %v", err)
	}
	listOut := stdout.String()
	for _, want := range []string{
		"key-id",
		"name",
		"note",
		"enabled",
		"local",
		nameValue,
		noteValue,
	} {
		if !strings.Contains(listOut, want) {
			t.Fatalf("list output = %q, want %q", listOut, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if err := RunWithIO([]string{"token", "verify", "local", "--stdin"}, strings.NewReader(tokenValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(token verify --stdin) returned error: %v", err)
	}
	verifyOut := stdout.String()
	if !strings.Contains(verifyOut, "token: valid") {
		t.Fatalf("verify output = %q, want valid", verifyOut)
	}
	if strings.Contains(verifyOut, tokenValue) {
		t.Fatalf("verify leaked token: %q", verifyOut)
	}
}

func TestRunConfigSetURLAndShow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"config", "set-url", "https://api.example.test/v1/"},
		{"config", "show"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}

	out := stdout.String()
	if !strings.Contains(out, `base_url = "https://api.example.test/v1"`) {
		t.Fatalf("config show = %q, want normalized base URL", out)
	}
}

func TestRunConfigSetKeyStdinRedactsShow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	keyValue := strings.Join([]string{"runtime", "key", "value"}, "-")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"install"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(install) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := RunWithIO([]string{"config", "set-key", "--stdin"}, strings.NewReader(keyValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(config set-key --stdin) returned error: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"config", "show"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(config show) returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("config show leaked key: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("config show = %q, want redacted key", out)
	}
}

func TestRunConfigTestUsesKeyWithoutPrintingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	keyValue := strings.Join([]string{"runtime", "test", "key"}, "-")
	var seenAuth string
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	commands := [][]string{
		{"install"},
		{"config", "set-url", server.URL},
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
	if err := RunWithIO([]string{"config", "set-key", "--stdin"}, strings.NewReader(keyValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(config set-key --stdin) returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"config", "test", "--path", "/v1/models"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(config test) returned error: %v", err)
	}

	if seenAuth != "Bearer "+keyValue {
		t.Fatalf("Authorization = %q, want bearer key", seenAuth)
	}
	if seenPath != "/v1/models" {
		t.Fatalf("path = %q, want /v1/models", seenPath)
	}
	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("config test leaked key: %q", out)
	}
	if !strings.Contains(out, "status: 200") {
		t.Fatalf("config test output = %q, want status", out)
	}
}

func TestRunConfigTestHintsModelsPathOnNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"config", "set-url", server.URL + "/v1"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}
	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"config", "test", "--path", "/v1/models"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(config test) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "status: 404") {
		t.Fatalf("config test output = %q, want 404 status", out)
	}
	if !strings.Contains(out, "try --path /models") {
		t.Fatalf("config test output = %q, want /models hint", out)
	}
}

func TestRunRelayTestUsesLocalConfigAndToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	tokenValue := "llmr_test_local_token"
	var seenAuth string
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	configText := fmt.Sprintf(`listen_addr = "%s"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = "redacted"

[tunnel]
enabled = false
ssh_host = ""
ssh_user = ""
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
`, listener.Addr().String())
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	tokenJSON := fmt.Sprintf(`[{"key_id":"local","token":"%s","enabled":true}]`, tokenValue)
	if err := os.WriteFile(filepath.Join(home, "tokens.json"), []byte(tokenJSON), 0o600); err != nil {
		t.Fatalf("write tokens: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"test"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(test) returned error: %v", err)
	}

	if seenAuth != "Bearer "+tokenValue {
		t.Fatalf("Authorization = %q, want relay token", seenAuth)
	}
	out := stdout.String()
	for _, want := range []string{
		"relay: ok",
		"status: 200",
		"key-id: local",
		"OpenAI-compatible base_url:",
		"Anthropic-compatible base_url:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("test output = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, tokenValue) {
		t.Fatalf("test output leaked relay token: %q", out)
	}
}

func TestRunRelayTestURLOverridePrintsClientBaseURLs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	tokenValue := "llmr_test_public_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+tokenValue {
			t.Fatalf("Authorization = %q, want relay token", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	configText := `listen_addr = "0.0.0.0:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = "redacted"
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	tokenJSON := fmt.Sprintf(`[{"key_id":"public","token":"%s","enabled":true}]`, tokenValue)
	if err := os.WriteFile(filepath.Join(home, "tokens.json"), []byte(tokenJSON), 0o600); err != nil {
		t.Fatalf("write tokens: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"test", "--url", server.URL}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(test --url) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, server.URL+"/v1/models") {
		t.Fatalf("test output = %q, want tested URL", out)
	}
	if !strings.Contains(out, server.URL+"/v1") {
		t.Fatalf("test output = %q, want OpenAI base URL", out)
	}
	if strings.Contains(out, tokenValue) {
		t.Fatalf("test output leaked relay token: %q", out)
	}
}

func TestRunRelayTestReportsMissingToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	configText := `listen_addr = "127.0.0.1:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = "redacted"
`
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "tokens.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("write tokens: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"test"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(test) returned nil error, want missing token error")
	}
	if !strings.Contains(err.Error(), "no enabled relay token found") {
		t.Fatalf("error = %q, want missing token", err.Error())
	}
}

func TestRunRelayTestReportsConnectionRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	configText := fmt.Sprintf(`listen_addr = "%s"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = "redacted"
`, addr)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "tokens.json"), []byte(`[{"key_id":"local","token":"llmr_token","enabled":true}]`), 0o600); err != nil {
		t.Fatalf("write tokens: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Run([]string{"test"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(test) returned nil error, want connection error")
	}
	if !strings.Contains(err.Error(), "run llmrelay start") {
		t.Fatalf("error = %q, want start hint", err.Error())
	}
}

func TestRunUpstreamIsRemoved(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"upstream", "show"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(upstream show) returned nil error, want removed command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
}

func TestRunConfigValidateReportsMissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"config", "validate"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(config validate) returned nil error, want missing config error")
	}
	if !strings.Contains(err.Error(), "run llmrelay install") {
		t.Fatalf("error = %q, want install hint", err.Error())
	}
}

func TestRunConfigValidateChecksEnvKeyReference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	envName := "LLMRELAY_TEST_MISSING_KEY"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"config", "set-url", "https://api.example.test/v1"},
		{"config", "set-key", "--env", envName},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	err := Run([]string{"config", "validate"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(config validate) returned nil error, want env key error")
	}
	if !strings.Contains(err.Error(), envName) {
		t.Fatalf("error = %q, want env name", err.Error())
	}
}

func TestRunConfigValidateSucceedsForInlineKeyWithoutPrintingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	keyValue := strings.Join([]string{"local", "inline", "value"}, "-")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"config", "set-url", "https://api.example.test/v1"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
	}
	stdout.Reset()
	stderr.Reset()
	if err := RunWithIO([]string{"config", "set-key", "--stdin"}, strings.NewReader(keyValue+"\n"), &stdout, &stderr); err != nil {
		t.Fatalf("Run(config set-key --stdin) returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run([]string{"config", "validate"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(config validate) returned error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("config validate leaked key: %q", out)
	}
	if !strings.Contains(out, "config: ok") {
		t.Fatalf("config validate output = %q, want ok", out)
	}
}

func TestRunConfigValidateChecksTunnelRequiredFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	keyValue := strings.Join([]string{"local", "tunnel", "value"}, "-")
	configText := fmt.Sprintf(`listen_addr = "127.0.0.1:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = "%s"

[tunnel]
enabled = true
ssh_host = ""
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
`, keyValue)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configText), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"config", "validate"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(config validate) returned nil error, want tunnel error")
	}
	if !strings.Contains(err.Error(), "tunnel ssh_host is empty") {
		t.Fatalf("error = %q, want tunnel ssh_host error", err.Error())
	}
	if strings.Contains(stdout.String()+stderr.String()+err.Error(), keyValue) {
		t.Fatalf("config validate leaked key")
	}
}

func TestRunDoctorReportsUninitializedHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"doctor"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) returned nil error, want diagnostic failure")
	}
	out := stdout.String()
	if !strings.Contains(out, "config.toml: missing") {
		t.Fatalf("doctor output = %q, want missing config", out)
	}
	if !strings.Contains(out, "tokens.json: missing") {
		t.Fatalf("doctor output = %q, want missing tokens", out)
	}
}

func TestRunDoctorReportsConfiguredEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LLMRELAY_HOME", home)
	setInstallTestHome(t)
	envName := "LLMRELAY_TEST_DOCTOR_KEY"
	keyValue := strings.Join([]string{"doctor", "runtime", "value"}, "-")
	t.Setenv(envName, keyValue)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for _, args := range [][]string{
		{"install"},
		{"config", "set-url", "https://api.example.test/v1"},
		{"config", "set-key", "--env", envName},
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
	if err := Run([]string{"doctor"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(doctor) returned error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, keyValue) {
		t.Fatalf("doctor leaked key: %q", out)
	}
	for _, want := range []string{
		"config.toml: ok",
		"tokens.json: ok",
		"config: ok",
		"ssh-tunnel: disabled",
		"upstream key: ok",
		"tokens: 1 total, 1 disabled",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output = %q, want %q", out, want)
		}
	}
}
