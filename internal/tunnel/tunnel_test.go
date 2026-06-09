package tunnel

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

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

type fakeSupervisorStarter struct {
	results []*Process
	errs    []error
	starts  int
}

func (f *fakeSupervisorStarter) Start(ctx context.Context, cfg config.Config, stderr io.Writer) (*Process, error) {
	idx := f.starts
	f.starts++
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.results) && f.results[idx] != nil {
		return f.results[idx], nil
	}
	done := make(chan error, 1)
	done <- nil
	return &Process{done: done}, nil
}

func TestSupervisorRetriesAfterTunnelExit(t *testing.T) {
	cfg := config.DefaultConfig()
	firstDone := make(chan error, 1)
	firstDone <- errors.New("ssh exited")
	secondDone := make(chan error)
	starts := &fakeSupervisorStarter{
		results: []*Process{
			{done: firstDone},
			{done: secondDone},
		},
	}
	sleeps := make([]time.Duration, 0, 2)
	supervisor := Supervisor{
		Starter: starts,
		Pause: func(ctx context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		},
		MinBackoff: 10 * time.Millisecond,
		MaxBackoff: 40 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- supervisor.Run(ctx, cfg, io.Discard)
	}()

	deadline := time.After(2 * time.Second)
	for starts.starts < 2 {
		select {
		case <-deadline:
			t.Fatal("supervisor did not restart tunnel")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if got := supervisor.Status(); got.State != StateRunning {
		t.Fatalf("status = %#v, want running", got)
	}
	if len(sleeps) == 0 {
		t.Fatal("expected supervisor to back off before restarting")
	}
	cancel()
	close(secondDone)
	if err := <-done; err != context.Canceled {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
}
