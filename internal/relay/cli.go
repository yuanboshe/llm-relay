package relay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: llmrelay <command>")
		return 2
	}

	switch args[0] {
	case "init":
		cfg, err := EnsureConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	case "upstream":
		return runUpstream(args[1:], stdout, stderr)
	case "token":
		return runToken(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stderr)
	case "start":
		return runStart(stdout, stderr)
	case "stop":
		return runStop(stdout, stderr)
	case "status":
		return runStatus(stdout)
	case "logs":
		return runLogs(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

func runUpstream(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: llmrelay upstream <set|show>")
		return 2
	}
	cfg, err := EnsureConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "set":
		fs := flag.NewFlagSet("upstream set", flag.ContinueOnError)
		fs.SetOutput(stderr)
		provider := fs.String("provider", cfg.Upstream.Provider, "upstream provider (openai|anthropic|custom)")
		baseURL := fs.String("base-url", cfg.Upstream.BaseURL, "upstream base URL")
		apiKey := fs.String("api-key", cfg.Upstream.APIKey, "upstream API key")
		if err := fs.Parse(args[1:]); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		cfg.Upstream.Provider = strings.TrimSpace(*provider)
		cfg.Upstream.BaseURL = strings.TrimRight(strings.TrimSpace(*baseURL), "/")
		cfg.Upstream.APIKey = strings.TrimSpace(*apiKey)
		if err := SaveConfig(cfg); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "upstream config updated")
		return 0
	case "show":
		out := cfg.Upstream
		if out.APIKey != "" {
			out.APIKey = "***"
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	default:
		fmt.Fprintln(stderr, "usage: llmrelay upstream <set|show>")
		return 2
	}
}

func runToken(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: llmrelay token <create|list|revoke>")
		return 2
	}
	cfg, err := EnsureConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "create":
		token, err := generateRelayToken()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := AddToken(&cfg, token); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := SaveConfig(cfg); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, token)
		return 0
	case "list":
		for _, t := range TokenSummaries(cfg) {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", t.ID, t.CreatedAt, t.Hash)
		}
		return 0
	case "revoke":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: llmrelay token revoke <token-id>")
			return 2
		}
		if err := RemoveTokenByID(&cfg, args[1]); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := SaveConfig(cfg); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "token revoked")
		return 0
	default:
		fmt.Fprintln(stderr, "usage: llmrelay token <create|list|revoke>")
		return 2
	}
}

func runServe(args []string, stderr io.Writer) int {
	cfg, err := EnsureConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", cfg.ListenAddr, "listen address")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg.ListenAddr = *addr
	logFile, err := logPath()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	logger, err := NewLogger(logFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer logger.Close()
	srv, err := NewRelayServer(cfg, logger)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	pid, _ := pidPath()
	_ = os.WriteFile(pid, []byte(strconv.Itoa(os.Getpid())), 0o600)
	defer os.Remove(pid)
	logger.Log(map[string]any{"level": "info", "msg": "server_started", "listen_addr": cfg.ListenAddr})
	if err := (&http.Server{Addr: cfg.ListenAddr, Handler: srv.Handler()}).ListenAndServe(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runStart(stdout, stderr io.Writer) int {
	cfg, err := EnsureConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if running, _ := isRunning(); running {
		fmt.Fprintln(stdout, "already running")
		return 0
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	lp, err := logPath()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	f, err := os.OpenFile(lp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer f.Close()
	cmd := exec.Command(exe, "serve", "--addr", cfg.ListenAddr)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := WaitForServer(cfg.ListenAddr, 3*time.Second); err != nil {
		_ = cmd.Process.Kill()
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "started pid=%d\n", cmd.Process.Pid)
	return 0
}

func runStop(stdout, stderr io.Writer) int {
	pid, err := readPID()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "stopped")
	return 0
}

func runStatus(stdout io.Writer) int {
	running, pid := isRunning()
	if !running {
		fmt.Fprintln(stdout, "stopped")
		return 0
	}
	fmt.Fprintf(stdout, "running pid=%d\n", pid)
	return 0
}

func runLogs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tail := fs.Int("tail", 50, "number of lines")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	lp, err := logPath()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	lines, err := TailLogs(lp, *tail)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

func runDoctor(stdout, stderr io.Writer) int {
	cfg, err := EnsureConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	checks := []struct {
		name string
		ok   bool
	}{
		{name: "config_exists", ok: true},
		{name: "upstream_base_url", ok: cfg.Upstream.BaseURL != ""},
		{name: "upstream_api_key", ok: cfg.Upstream.APIKey != ""},
		{name: "token_count>0", ok: len(cfg.Tokens) > 0},
	}
	status := 0
	for _, c := range checks {
		fmt.Fprintf(stdout, "%s=%t\n", c.name, c.ok)
		if !c.ok {
			status = 1
		}
	}
	return status
}

func generateRelayToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tokenPrefix + hex.EncodeToString(b), nil
}

func readPID() (int, error) {
	p, err := pidPath()
	if err != nil {
		return 0, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func isRunning() (bool, int) {
	pid, err := readPID()
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, 0
	}
	return true, pid
}
