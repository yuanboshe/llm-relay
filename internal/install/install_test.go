package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuanboshe/llm-relay/internal/config"
)

func TestInstallCopiesBinaryToFinalCommandNameAndInitializesFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "llmrelay-darwin-arm64")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	configHome := filepath.Join(dir, ".llmrelay")
	paths := config.Paths{
		Dir:        configHome,
		ConfigFile: filepath.Join(configHome, config.DefaultConfigName),
		TokenFile:  filepath.Join(configHome, config.DefaultStoreName),
	}

	result, err := Run(Options{
		SourcePath:     source,
		UserHome:       dir,
		ZshrcPath:      filepath.Join(dir, ".zshrc"),
		ConfigPaths:    paths,
		ZshCompletion:  "completion script",
		SkipShellInit:  false,
		SkipCompletion: false,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	wantInstalled := filepath.Join(dir, "Library", "Application Support", "llmrelay", "bin", "llmrelay")
	if result.InstalledPath != wantInstalled {
		t.Fatalf("InstalledPath = %q, want %q", result.InstalledPath, wantInstalled)
	}
	data, err := os.ReadFile(wantInstalled)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(data) != "binary" {
		t.Fatalf("installed binary = %q, want copied source", string(data))
	}
	if _, err := os.Stat(paths.ConfigFile); err != nil {
		t.Fatalf("config not initialized: %v", err)
	}
	if _, err := os.Stat(paths.TokenFile); err != nil {
		t.Fatalf("token store not initialized: %v", err)
	}
	if filepath.Base(result.SymlinkPath) != "llmrelay" {
		t.Fatalf("SymlinkPath = %q, want final command name", result.SymlinkPath)
	}
}

func TestInstallIsIdempotentAndPreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "downloaded-name")
	if err := os.WriteFile(source, []byte("new binary"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	configHome := filepath.Join(dir, ".llmrelay")
	paths := config.Paths{
		Dir:        configHome,
		ConfigFile: filepath.Join(configHome, config.DefaultConfigName),
		TokenFile:  filepath.Join(configHome, config.DefaultStoreName),
	}
	if err := os.MkdirAll(configHome, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	existingConfig := []byte("existing config")
	if err := os.WriteFile(paths.ConfigFile, existingConfig, 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := Run(Options{
			SourcePath:     source,
			UserHome:       dir,
			ZshrcPath:      filepath.Join(dir, ".zshrc"),
			ConfigPaths:    paths,
			ZshCompletion:  "completion script",
			SkipShellInit:  false,
			SkipCompletion: false,
		}); err != nil {
			t.Fatalf("Run #%d returned error: %v", i+1, err)
		}
	}

	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != string(existingConfig) {
		t.Fatalf("config = %q, want existing config preserved", string(data))
	}
	zshrc, err := os.ReadFile(filepath.Join(dir, ".zshrc"))
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	if count := strings.Count(string(zshrc), "# >>> llmrelay shell integration >>>"); count != 1 {
		t.Fatalf("zshrc marker count = %d, want 1 in %q", count, string(zshrc))
	}
}

func TestInstallWritesZshCompletion(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "llmrelay-darwin-arm64")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	paths := config.Paths{
		Dir:        filepath.Join(dir, ".llmrelay"),
		ConfigFile: filepath.Join(dir, ".llmrelay", config.DefaultConfigName),
		TokenFile:  filepath.Join(dir, ".llmrelay", config.DefaultStoreName),
	}

	result, err := Run(Options{
		SourcePath:    source,
		UserHome:      dir,
		ZshrcPath:     filepath.Join(dir, ".zshrc"),
		ConfigPaths:   paths,
		ZshCompletion: "#compdef llmrelay\n",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if filepath.Base(result.CompletionPath) != "_llmrelay" {
		t.Fatalf("CompletionPath = %q, want _llmrelay", result.CompletionPath)
	}
	completion, err := os.ReadFile(result.CompletionPath)
	if err != nil {
		t.Fatalf("read completion: %v", err)
	}
	if !strings.Contains(string(completion), "#compdef llmrelay") {
		t.Fatalf("completion = %q, want llmrelay compdef", string(completion))
	}
}

func TestUninstallRemovesInstallArtifactsAndKeepsConfigByDefault(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, ".llmrelay")
	paths := config.Paths{
		Dir:        configHome,
		ConfigFile: filepath.Join(configHome, config.DefaultConfigName),
		TokenFile:  filepath.Join(configHome, config.DefaultStoreName),
		PIDFile:    filepath.Join(configHome, config.DefaultPIDName),
		LogFile:    filepath.Join(configHome, config.DefaultLogName),
	}
	installedPath := filepath.Join(dir, "Library", "Application Support", "llmrelay", "bin", "llmrelay")
	symlinkPath := filepath.Join(dir, ".local", "bin", "llmrelay")
	completionPath := filepath.Join(dir, ".zsh", "completions", "_llmrelay")
	zshrcPath := filepath.Join(dir, ".zshrc")

	files := []struct {
		path string
		data string
	}{
		{installedPath, "binary"},
		{symlinkPath, "link"},
		{completionPath, "completion"},
		{paths.ConfigFile, "listen_addr = \"127.0.0.1:18080\"\n"},
		{paths.TokenFile, "[]\n"},
		{paths.PIDFile, "1234\n"},
		{paths.LogFile, "log\n"},
		{zshrcPath, "before\n" + shellBlock(dir) + "after\n"},
	}
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", file.path, err)
		}
		if err := os.WriteFile(file.path, []byte(file.data), 0o600); err != nil {
			t.Fatalf("write %s: %v", file.path, err)
		}
	}

	result, err := Uninstall(UninstallOptions{
		UserHome:    dir,
		ZshrcPath:   zshrcPath,
		ConfigPaths: paths,
	})
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if result.InstalledPath != installedPath || result.SymlinkPath != symlinkPath || result.CompletionPath != completionPath {
		t.Fatalf("result = %#v, want install paths", result)
	}
	for _, path := range []string{installedPath, symlinkPath, completionPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists or stat failed: %v", path, err)
		}
	}
	zshrc, err := os.ReadFile(zshrcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	if strings.Contains(string(zshrc), shellBlockStart) || strings.Contains(string(zshrc), shellBlockEnd) {
		t.Fatalf("zshrc still contains shell block: %q", string(zshrc))
	}
	for _, path := range []string{paths.ConfigFile, paths.TokenFile, paths.PIDFile, paths.LogFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected preserved %s: %v", path, err)
		}
	}
}

func TestUninstallPurgesConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, ".llmrelay")
	paths := config.Paths{
		Dir:        configHome,
		ConfigFile: filepath.Join(configHome, config.DefaultConfigName),
		TokenFile:  filepath.Join(configHome, config.DefaultStoreName),
	}
	if err := os.MkdirAll(configHome, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	if err := os.WriteFile(paths.ConfigFile, []byte("config"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(paths.TokenFile, []byte("[]\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}

	if _, err := Uninstall(UninstallOptions{
		UserHome:    dir,
		ConfigPaths: paths,
		Purge:       true,
	}); err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if _, err := os.Stat(configHome); !os.IsNotExist(err) {
		t.Fatalf("config home still exists or stat failed: %v", err)
	}
}
