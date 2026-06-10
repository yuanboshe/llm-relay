package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuanboshe/llm-relay/internal/config"
)

const (
	commandName       = "llmrelay"
	shellBlockStart   = "# >>> llmrelay shell integration >>>"
	shellBlockEnd     = "# <<< llmrelay shell integration <<<"
	completionDirName = ".zsh/completions"
)

// Options configures a self-install run.
type Options struct {
	SourcePath     string
	UserHome       string
	ZshrcPath      string
	ConfigPaths    config.Paths
	ZshCompletion  string
	SkipShellInit  bool
	SkipCompletion bool
}

// Result describes files touched by the installer.
type Result struct {
	InstalledPath     string
	SymlinkPath       string
	ZshrcPath         string
	CompletionPath    string
	ConfigCreated     bool
	TokenStoreCreated bool
}

// UninstallOptions configures a removal run.
type UninstallOptions struct {
	UserHome    string
	ZshrcPath   string
	ConfigPaths config.Paths
	Purge       bool
}

// UninstallResult describes files removed by the uninstaller.
type UninstallResult struct {
	InstalledPath     string
	SymlinkPath       string
	ZshrcPath         string
	CompletionPath    string
	ConfigDir         string
	Purged            bool
	RemovedInstalled  bool
	RemovedSymlink    bool
	RemovedZshrc      bool
	RemovedCompletion bool
}

// Run installs the current binary into a stable user path and initializes local files.
func Run(opts Options) (Result, error) {
	if strings.TrimSpace(opts.SourcePath) == "" {
		return Result{}, fmt.Errorf("source path is empty")
	}
	userHome, err := resolveUserHome(opts.UserHome)
	if err != nil {
		return Result{}, err
	}
	paths := opts.ConfigPaths
	if paths.Dir == "" || paths.ConfigFile == "" || paths.TokenFile == "" {
		paths, err = config.DefaultPaths()
		if err != nil {
			return Result{}, err
		}
	}

	installedPath, symlinkPath, completionPath := installationPaths(userHome)
	zshrcPath := opts.ZshrcPath
	if strings.TrimSpace(zshrcPath) == "" {
		zshrcPath = filepath.Join(userHome, ".zshrc")
	}

	if err := copyFile(opts.SourcePath, installedPath, 0o755); err != nil {
		return Result{}, fmt.Errorf("install binary: %w", err)
	}
	if err := linkOrCopy(installedPath, symlinkPath); err != nil {
		return Result{}, fmt.Errorf("install command link: %w", err)
	}
	configCreated, tokenCreated, err := config.Ensure(paths)
	if err != nil {
		return Result{}, fmt.Errorf("initialize config: %w", err)
	}
	if !opts.SkipCompletion {
		if err := writeFile(completionPath, []byte(opts.ZshCompletion), 0o644); err != nil {
			return Result{}, fmt.Errorf("install zsh completion: %w", err)
		}
	}
	if !opts.SkipShellInit {
		if err := ensureShellBlock(zshrcPath, shellBlock(userHome)); err != nil {
			return Result{}, fmt.Errorf("update zshrc: %w", err)
		}
	}

	return Result{
		InstalledPath:     installedPath,
		SymlinkPath:       symlinkPath,
		ZshrcPath:         zshrcPath,
		CompletionPath:    completionPath,
		ConfigCreated:     configCreated,
		TokenStoreCreated: tokenCreated,
	}, nil
}

// Uninstall removes user-level installation artifacts.
func Uninstall(opts UninstallOptions) (UninstallResult, error) {
	userHome, err := resolveUserHome(opts.UserHome)
	if err != nil {
		return UninstallResult{}, err
	}
	paths := opts.ConfigPaths
	if paths.Dir == "" || paths.ConfigFile == "" || paths.TokenFile == "" {
		paths, err = config.DefaultPaths()
		if err != nil {
			return UninstallResult{}, err
		}
	}
	zshrcPath := opts.ZshrcPath
	if strings.TrimSpace(zshrcPath) == "" {
		zshrcPath = filepath.Join(userHome, ".zshrc")
	}
	installedPath, symlinkPath, completionPath := installationPaths(userHome)
	result := UninstallResult{
		InstalledPath:  installedPath,
		SymlinkPath:    symlinkPath,
		ZshrcPath:      zshrcPath,
		CompletionPath: completionPath,
		ConfigDir:      paths.Dir,
		Purged:         opts.Purge,
	}

	var errs []string
	if err := removeTrackedFile(installedPath, &result.RemovedInstalled); err != nil {
		errs = append(errs, err.Error())
	}
	if err := removeTrackedFile(symlinkPath, &result.RemovedSymlink); err != nil {
		errs = append(errs, err.Error())
	}
	if err := removeTrackedFile(completionPath, &result.RemovedCompletion); err != nil {
		errs = append(errs, err.Error())
	}
	if removed, err := removeShellBlock(zshrcPath); err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", zshrcPath, err))
	} else if removed {
		result.RemovedZshrc = true
	}
	if opts.Purge && paths.Dir != "" {
		if err := os.RemoveAll(paths.Dir); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", paths.Dir, err))
		}
	}
	if len(errs) > 0 {
		return result, errors.New(strings.Join(errs, "; "))
	}
	return result, nil
}

func removeTrackedFile(path string, removed *bool) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	if removed != nil {
		*removed = true
	}
	return nil
}

func resolveUserHome(userHome string) (string, error) {
	if strings.TrimSpace(userHome) != "" {
		return userHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func installationPaths(userHome string) (string, string, string) {
	return filepath.Join(userHome, "Library", "Application Support", "llmrelay", "bin", commandName),
		filepath.Join(userHome, ".local", "bin", commandName),
		filepath.Join(userHome, completionDirName, "_"+commandName)
}

func copyFile(src string, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFile(dst, data, mode)
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func linkOrCopy(target string, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		return err
	}
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(target, link); err == nil {
		return nil
	}
	return copyFile(target, link, 0o755)
}

func ensureShellBlock(path string, block string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	if strings.Contains(text, shellBlockStart) {
		return nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += block
	return writeFile(path, []byte(text), 0o644)
}

func removeShellBlock(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	text := string(data)
	if !strings.Contains(text, shellBlockStart) {
		return false, nil
	}
	start := strings.Index(text, shellBlockStart)
	end := strings.Index(text[start:], shellBlockEnd)
	if end < 0 {
		return false, fmt.Errorf("shell integration block end marker not found in %s", path)
	}
	end += start + len(shellBlockEnd)
	trimmed := strings.TrimSpace(text[:start] + "\n" + text[end:])
	if trimmed != "" {
		trimmed += "\n"
	}
	return true, writeFile(path, []byte(trimmed), 0o644)
}

func shellBlock(userHome string) string {
	binDir := filepath.Join(userHome, ".local", "bin")
	completionDir := filepath.Join(userHome, completionDirName)
	return fmt.Sprintf(`%s
export PATH="%s:$PATH"
fpath=("%s" $fpath)
autoload -Uz compinit
compinit
%s
`, shellBlockStart, escapeDoubleQuoted(binDir), escapeDoubleQuoted(completionDir), shellBlockEnd)
}

func escapeDoubleQuoted(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, `$`, `\$`)
	value = strings.ReplaceAll(value, "`", "\\`")
	return value
}
