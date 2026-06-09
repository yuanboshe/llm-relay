package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"

	"github.com/yuanboshe/llm-relay/internal/config"
)

// Process wraps the managed ssh process.
type Process struct {
	cmd  *exec.Cmd
	done chan error
}

// BuildSSHArgs returns the OpenSSH arguments for the configured reverse tunnel.
func BuildSSHArgs(cfg config.Config) ([]string, error) {
	tunnel := cfg.Tunnel
	if strings.TrimSpace(tunnel.SSHHost) == "" {
		return nil, fmt.Errorf("tunnel ssh_host is empty")
	}
	if strings.TrimSpace(tunnel.SSHUser) == "" {
		return nil, fmt.Errorf("tunnel ssh_user is empty")
	}
	if strings.TrimSpace(tunnel.SSHPort) == "" {
		tunnel.SSHPort = "22"
	}
	if strings.TrimSpace(tunnel.RemoteHost) == "" {
		tunnel.RemoteHost = "127.0.0.1"
	}
	if strings.TrimSpace(tunnel.RemotePort) == "" {
		return nil, fmt.Errorf("tunnel remote_port is empty")
	}
	localHost, localPort, err := splitListenAddr(cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	remoteSpec := fmt.Sprintf("%s:%s:%s:%s", tunnel.RemoteHost, tunnel.RemotePort, localHost, localPort)
	return []string{
		"-N",
		"-T",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-p", tunnel.SSHPort,
		"-R", remoteSpec,
		fmt.Sprintf("%s@%s", tunnel.SSHUser, tunnel.SSHHost),
	}, nil
}

// Start starts a managed OpenSSH reverse tunnel.
func Start(ctx context.Context, cfg config.Config, stderr io.Writer) (*Process, error) {
	args, err := BuildSSHArgs(cfg)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	process := &Process{
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		process.done <- cmd.Wait()
	}()
	return process, nil
}

// Done returns a channel that receives the ssh process exit status.
func (p *Process) Done() <-chan error {
	return p.done
}

func splitListenAddr(listenAddr string) (string, string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", "", fmt.Errorf("invalid listen_addr for tunnel: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return host, port, nil
}
