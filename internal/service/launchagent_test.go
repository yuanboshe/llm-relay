package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	output string
	calls  []string
}

func (f *fakeCommandRunner) Run(name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return f.output, nil
}

func TestLaunchAgentStartWritesPlistAndRunsLaunchctl(t *testing.T) {
	dir := t.TempDir()
	runner := &fakeCommandRunner{}
	manager := LaunchAgentManager{
		Label:      "com.yuanboshe.llmrelay",
		UID:        501,
		PlistPath:  filepath.Join(dir, "Library", "LaunchAgents", "com.yuanboshe.llmrelay.plist"),
		Executable: filepath.Join(dir, "Library", "Application Support", "llmrelay", "bin", "llmrelay"),
		LogFile:    filepath.Join(dir, ".llmrelay", "llmrelay.log"),
		Runner:     runner,
	}

	status, err := manager.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if status.State != StateRunning {
		t.Fatalf("State = %q, want running", status.State)
	}
	data, err := os.ReadFile(manager.PlistPath)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	plist := string(data)
	for _, want := range []string{
		"<string>com.yuanboshe.llmrelay</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<string>" + manager.Executable + "</string>",
		"<string>serve</string>",
		"<string>" + manager.LogFile + "</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist = %q, want %q", plist, want)
		}
	}
	wantCalls := []string{
		"launchctl bootout gui/501/com.yuanboshe.llmrelay",
		"launchctl bootstrap gui/501 " + manager.PlistPath,
		"launchctl kickstart -k gui/501/com.yuanboshe.llmrelay",
		"launchctl print gui/501/com.yuanboshe.llmrelay",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestLaunchAgentStopBootsOutService(t *testing.T) {
	runner := &fakeCommandRunner{}
	manager := LaunchAgentManager{
		Label:     "com.yuanboshe.llmrelay",
		UID:       501,
		PlistPath: filepath.Join(t.TempDir(), "com.yuanboshe.llmrelay.plist"),
		Runner:    runner,
	}

	status, err := manager.Stop()
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if status.State != StateStopped {
		t.Fatalf("State = %q, want stopped", status.State)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "launchctl bootout gui/501/com.yuanboshe.llmrelay" {
		t.Fatalf("calls = %#v, want bootout", runner.calls)
	}
}

func TestLaunchAgentStatusParsesPID(t *testing.T) {
	runner := &fakeCommandRunner{output: `gui/501/com.yuanboshe.llmrelay = {
	active count = 1
	pid = 4321
}`}
	manager := LaunchAgentManager{
		Label:     "com.yuanboshe.llmrelay",
		UID:       501,
		PlistPath: filepath.Join(t.TempDir(), "com.yuanboshe.llmrelay.plist"),
		LogFile:   filepath.Join(t.TempDir(), "llmrelay.log"),
		Runner:    runner,
	}

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.State != StateRunning || status.PID != 4321 {
		t.Fatalf("status = %#v, want running pid 4321", status)
	}
}
