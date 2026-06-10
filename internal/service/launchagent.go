package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const defaultLaunchAgentLabel = "com.yuanboshe.llmrelay"

// CommandRunner runs external commands.
type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// LaunchAgentManager manages llmrelay as a macOS LaunchAgent.
type LaunchAgentManager struct {
	Label      string
	UID        int
	PlistPath  string
	Executable string
	LogFile    string
	Runner     CommandRunner
}

// DefaultLaunchAgentManager returns a LaunchAgent manager for the current user.
func DefaultLaunchAgentManager(executable string, logFile string) (LaunchAgentManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return LaunchAgentManager{}, err
	}
	return LaunchAgentManager{
		Label:      defaultLaunchAgentLabel,
		UID:        os.Getuid(),
		PlistPath:  filepath.Join(home, "Library", "LaunchAgents", defaultLaunchAgentLabel+".plist"),
		Executable: executable,
		LogFile:    logFile,
	}, nil
}

// Start writes the LaunchAgent plist, bootstraps it, and starts the service.
func (m LaunchAgentManager) Start() (Status, error) {
	current, _ := m.Status()
	if current.State == StateRunning {
		current.Message = "already running"
		return current, nil
	}
	if err := m.writePlist(); err != nil {
		return Status{}, err
	}
	target := m.target()
	if current.Message == "launchagent loaded" {
		_, _ = m.runner().Run("launchctl", "bootout", target)
	}
	if _, err := m.runner().Run("launchctl", "bootstrap", m.domain(), m.PlistPath); err != nil {
		return Status{}, fmt.Errorf("launchctl bootstrap: %w", err)
	}
	if _, err := m.runner().Run("launchctl", "kickstart", target); err != nil {
		return Status{}, fmt.Errorf("launchctl kickstart: %w", err)
	}
	status, err := m.Status()
	if err != nil {
		return Status{State: StateRunning, LogFile: m.LogFile}, nil
	}
	if status.State == StateStopped {
		status.State = StateRunning
	}
	return status, nil
}

// Stop unloads the LaunchAgent.
func (m LaunchAgentManager) Stop() (Status, error) {
	if _, err := m.runner().Run("launchctl", "bootout", m.target()); err != nil {
		return Status{}, fmt.Errorf("launchctl bootout: %w", err)
	}
	return Status{
		State:   StateStopped,
		LogFile: m.LogFile,
		Message: "launchagent unloaded",
	}, nil
}

// Restart restarts the LaunchAgent.
func (m LaunchAgentManager) Restart() (Status, error) {
	if _, err := m.Stop(); err != nil {
		return Status{}, err
	}
	return m.Start()
}

// Status reports LaunchAgent status by parsing launchctl print output.
func (m LaunchAgentManager) Status() (Status, error) {
	out, err := m.runner().Run("launchctl", "print", m.target())
	if err != nil {
		return Status{
			State:   StateStopped,
			LogFile: m.LogFile,
			Message: "launchagent not loaded",
		}, nil
	}
	pid := parseLaunchctlPID(out)
	state := StateRunning
	if pid == 0 {
		state = StateStopped
	}
	return Status{
		State:   state,
		PID:     pid,
		LogFile: m.LogFile,
		Message: "launchagent loaded",
	}, nil
}

func (m LaunchAgentManager) runner() CommandRunner {
	if m.Runner != nil {
		return m.Runner
	}
	return execCommandRunner{}
}

func (m LaunchAgentManager) domain() string {
	return fmt.Sprintf("gui/%d", m.UID)
}

func (m LaunchAgentManager) target() string {
	return m.domain() + "/" + m.label()
}

func (m LaunchAgentManager) label() string {
	if m.Label != "" {
		return m.Label
	}
	return defaultLaunchAgentLabel
}

func (m LaunchAgentManager) writePlist() error {
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(m.PlistPath, []byte(m.plist()), 0o644)
}

func (m LaunchAgentManager) plist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>serve</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(m.label()), xmlEscape(m.Executable), xmlEscape(m.LogFile), xmlEscape(m.LogFile))
}

func parseLaunchctlPID(output string) int {
	matches := regexp.MustCompile(`(?m)^\s*pid = ([0-9]+)$`).FindStringSubmatch(output)
	if len(matches) != 2 {
		return 0
	}
	pid, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return pid
}

func xmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	return value
}
