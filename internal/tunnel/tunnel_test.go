package tunnel

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yuanboshe/llm-relay/internal/config"
)

func TestBuildSSHArgsUsesReverseTunnelToLocalhostForWildcardListen(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ListenAddr = "0.0.0.0:18080"
	cfg.Tunnel.Enabled = true
	cfg.Tunnel.SSHHost = "relay-server"
	cfg.Tunnel.SSHUser = "ubuntu"
	cfg.Tunnel.SSHPort = "2222"
	cfg.Tunnel.RemoteHost = "127.0.0.1"
	cfg.Tunnel.RemotePort = "28080"

	args, err := BuildSSHArgs(cfg)
	if err != nil {
		t.Fatalf("BuildSSHArgs returned error: %v", err)
	}

	want := []string{
		"-N",
		"-T",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-p", "2222",
		"-R", "127.0.0.1:28080:127.0.0.1:18080",
		"ubuntu@relay-server",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildSSHArgsUsesConfiguredLocalhostListen(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ListenAddr = "127.0.0.1:19090"
	cfg.Tunnel.Enabled = true
	cfg.Tunnel.SSHHost = "relay-server"
	cfg.Tunnel.SSHUser = "ubuntu"
	cfg.Tunnel.RemoteHost = "127.0.0.1"
	cfg.Tunnel.RemotePort = "28080"

	args, err := BuildSSHArgs(cfg)
	if err != nil {
		t.Fatalf("BuildSSHArgs returned error: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "127.0.0.1:28080:127.0.0.1:19090") {
		t.Fatalf("args = %q, want reverse tunnel spec", joined)
	}
}

func TestBuildSSHArgsRequiresTunnelFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Tunnel.Enabled = true

	if _, err := BuildSSHArgs(cfg); err == nil {
		t.Fatal("BuildSSHArgs returned nil error, want missing ssh host error")
	}
}
