package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeStarter struct {
	pid      int
	started  bool
	args     []string
	logPath  string
	exePath  string
	startErr error
}

func (f *fakeStarter) Start(exePath string, args []string, logPath string) (int, error) {
	f.started = true
	f.exePath = exePath
	f.args = append([]string(nil), args...)
	f.logPath = logPath
	if f.startErr != nil {
		return 0, f.startErr
	}
	return f.pid, nil
}

type fakeController struct {
	running map[int]bool
	stopped []int
}

func (f *fakeController) IsRunning(pid int) bool {
	return f.running[pid]
}

func (f *fakeController) Stop(pid int) error {
	f.stopped = append(f.stopped, pid)
	f.running[pid] = false
	return nil
}

func TestManagerStatusReportsStoppedWithoutPIDFile(t *testing.T) {
	dir := t.TempDir()
	manager := Manager{
		PIDFile:    filepath.Join(dir, "llmrelay.pid"),
		LogFile:    filepath.Join(dir, "llmrelay.log"),
		Executable: "llmrelay",
		Controller: &fakeController{running: map[int]bool{}},
	}

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.State != StateStopped {
		t.Fatalf("State = %q, want stopped", status.State)
	}
}

func TestManagerStartWritesPIDAndServeCommand(t *testing.T) {
	dir := t.TempDir()
	starter := &fakeStarter{pid: 1234}
	manager := Manager{
		PIDFile:    filepath.Join(dir, "llmrelay.pid"),
		LogFile:    filepath.Join(dir, "llmrelay.log"),
		Executable: "llmrelay",
		Starter:    starter,
		Controller: &fakeController{running: map[int]bool{}},
	}

	status, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !starter.started {
		t.Fatal("starter was not called")
	}
	if strings.Join(starter.args, " ") != "serve" {
		t.Fatalf("args = %#v, want serve", starter.args)
	}
	if starter.logPath != manager.LogFile {
		t.Fatalf("log path = %q, want %q", starter.logPath, manager.LogFile)
	}
	if status.PID != 1234 || status.State != StateRunning {
		t.Fatalf("status = %#v, want running pid 1234", status)
	}
	data, err := os.ReadFile(manager.PIDFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "1234" {
		t.Fatalf("pid file = %q, want 1234", string(data))
	}
}

func TestManagerStartReportsAlreadyRunningPID(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "llmrelay.pid")
	if err := os.WriteFile(pidFile, []byte("1234\n"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	starter := &fakeStarter{pid: 5678}
	manager := Manager{
		PIDFile:    pidFile,
		LogFile:    filepath.Join(dir, "llmrelay.log"),
		Executable: "llmrelay",
		Starter:    starter,
		Controller: &fakeController{running: map[int]bool{1234: true}},
	}

	status, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if status.State != StateRunning || status.PID != 1234 {
		t.Fatalf("status = %#v, want running pid 1234", status)
	}
	if status.Message != "already running" {
		t.Fatalf("message = %q, want already running", status.Message)
	}
	if starter.started {
		t.Fatal("starter was called for already running service")
	}
}

func TestManagerStartCleansStalePID(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "llmrelay.pid")
	if err := os.WriteFile(pidFile, []byte("1234\n"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	starter := &fakeStarter{pid: 5678}
	manager := Manager{
		PIDFile:    pidFile,
		LogFile:    filepath.Join(dir, "llmrelay.log"),
		Executable: "llmrelay",
		Starter:    starter,
		Controller: &fakeController{running: map[int]bool{1234: false}},
	}

	status, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if status.PID != 5678 {
		t.Fatalf("PID = %d, want 5678", status.PID)
	}
}

func TestManagerStopSignalsRunningProcessAndRemovesPID(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "llmrelay.pid")
	if err := os.WriteFile(pidFile, []byte("1234\n"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	controller := &fakeController{running: map[int]bool{1234: true}}
	manager := Manager{
		PIDFile:    pidFile,
		LogFile:    filepath.Join(dir, "llmrelay.log"),
		Executable: "llmrelay",
		Controller: controller,
	}

	status, err := manager.Stop()
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if status.State != StateStopped {
		t.Fatalf("State = %q, want stopped", status.State)
	}
	if len(controller.stopped) != 1 || controller.stopped[0] != 1234 {
		t.Fatalf("stopped = %#v, want [1234]", controller.stopped)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists or stat failed: %v", err)
	}
}

func TestManagerStopKeepsPIDWhenProcessStillRuns(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "llmrelay.pid")
	if err := os.WriteFile(pidFile, []byte("1234\n"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	controller := &fakeController{running: map[int]bool{1234: true}}
	controllerStopKeepsRunning := &stickyController{fakeController: controller}
	manager := Manager{
		PIDFile:          pidFile,
		LogFile:          filepath.Join(dir, "llmrelay.log"),
		Executable:       "llmrelay",
		Controller:       controllerStopKeepsRunning,
		StopTimeout:      time.Millisecond,
		StopPollInterval: time.Millisecond,
	}

	if _, err := manager.Stop(); err == nil {
		t.Fatal("Stop returned nil error, want process still running error")
	}
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("pid file was removed or stat failed: %v", err)
	}
}

type stickyController struct {
	*fakeController
}

func (s *stickyController) Stop(pid int) error {
	s.stopped = append(s.stopped, pid)
	return nil
}

func TestTailLogReturnsLastLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "llmrelay.log")
	if err := os.WriteFile(logFile, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	lines, err := TailLog(logFile, 2)
	if err != nil {
		t.Fatalf("TailLog returned error: %v", err)
	}
	if lines != "three\nfour\n" {
		t.Fatalf("lines = %q, want last two lines", lines)
	}
}
