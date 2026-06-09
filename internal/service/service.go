package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// State describes the current background service state.
type State string

const (
	StateRunning State = "running"
	StateStopped State = "stopped"
	StateStale   State = "stale"
)

// Status describes the current service status.
type Status struct {
	State   State
	PID     int
	PIDFile string
	LogFile string
	Message string
}

// Controller manages process lifecycle operations.
type Controller interface {
	IsRunning(pid int) bool
	Stop(pid int) error
}

// Starter launches the service process.
type Starter interface {
	Start(executable string, args []string, logPath string) (int, error)
}

// Manager manages pid/log files and process lifecycle.
type Manager struct {
	PIDFile          string
	LogFile          string
	Executable       string
	Starter          Starter
	Controller       Controller
	StopTimeout      time.Duration
	StopPollInterval time.Duration
}

func (m Manager) controller() Controller {
	if m.Controller != nil {
		return m.Controller
	}
	return DefaultController{}
}

func (m Manager) starter() Starter {
	if m.Starter != nil {
		return m.Starter
	}
	return DefaultController{}
}

func (m Manager) executable() (string, error) {
	if strings.TrimSpace(m.Executable) != "" {
		return m.Executable, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}

// Status reports the current state of the service.
func (m Manager) Status() (Status, error) {
	pid, ok, err := readPID(m.PIDFile)
	if err != nil {
		return Status{}, err
	}
	if !ok {
		return Status{
			State:   StateStopped,
			PIDFile: m.PIDFile,
			LogFile: m.LogFile,
		}, nil
	}
	if !m.controller().IsRunning(pid) {
		return Status{
			State:   StateStale,
			PID:     pid,
			PIDFile: m.PIDFile,
			LogFile: m.LogFile,
			Message: "stale pid file",
		}, nil
	}
	return Status{
		State:   StateRunning,
		PID:     pid,
		PIDFile: m.PIDFile,
		LogFile: m.LogFile,
	}, nil
}

// Start launches the llmrelay serve command and writes its pid file.
func (m Manager) Start() (Status, error) {
	if err := ensureParentDir(m.PIDFile); err != nil {
		return Status{}, err
	}
	if err := ensureParentDir(m.LogFile); err != nil {
		return Status{}, err
	}

	current, ok, err := readPID(m.PIDFile)
	if err != nil {
		return Status{}, err
	}
	if ok && m.controller().IsRunning(current) {
		return Status{}, fmt.Errorf("llmrelay is already running (pid %d)", current)
	}
	if ok && !m.controller().IsRunning(current) {
		_ = os.Remove(m.PIDFile)
	}

	executable, err := m.executable()
	if err != nil {
		return Status{}, err
	}
	pid, err := m.starter().Start(executable, []string{"serve"}, m.LogFile)
	if err != nil {
		return Status{}, err
	}
	if err := writePID(m.PIDFile, pid); err != nil {
		return Status{}, err
	}
	return Status{
		State:   StateRunning,
		PID:     pid,
		PIDFile: m.PIDFile,
		LogFile: m.LogFile,
	}, nil
}

// Stop stops the running service if present.
func (m Manager) Stop() (Status, error) {
	pid, ok, err := readPID(m.PIDFile)
	if err != nil {
		return Status{}, err
	}
	if !ok {
		return Status{
			State:   StateStopped,
			PIDFile: m.PIDFile,
			LogFile: m.LogFile,
		}, nil
	}
	if !m.controller().IsRunning(pid) {
		_ = os.Remove(m.PIDFile)
		return Status{
			State:   StateStale,
			PID:     pid,
			PIDFile: m.PIDFile,
			LogFile: m.LogFile,
			Message: "stale pid file removed",
		}, nil
	}
	if err := m.controller().Stop(pid); err != nil {
		return Status{}, err
	}
	timeout := m.StopTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pollInterval := m.StopPollInterval
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		if !m.controller().IsRunning(pid) {
			_ = os.Remove(m.PIDFile)
			return Status{
				State:   StateStopped,
				PID:     pid,
				PIDFile: m.PIDFile,
				LogFile: m.LogFile,
			}, nil
		}
		if time.Now().After(deadline) {
			return Status{}, fmt.Errorf("process %d did not stop", pid)
		}
		time.Sleep(pollInterval)
	}
}

// Restart restarts the service.
func (m Manager) Restart() (Status, error) {
	_, err := m.Stop()
	if err != nil {
		return Status{}, err
	}
	return m.Start()
}

// TailLog returns the last n lines of the log file.
func TailLog(path string, n int) (string, error) {
	if n <= 0 {
		n = 100
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("log file not found: %s", path)
		}
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func ensureParentDir(path string) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == string(os.PathSeparator) {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

func readPID(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, false, nil
	}
	pid, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, fmt.Errorf("invalid pid file %s: %w", path, err)
	}
	return pid, true, nil
}

func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", pid)), 0o600)
}
