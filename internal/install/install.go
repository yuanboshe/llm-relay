package install

import (
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

	installedPath := filepath.Join(userHome, "Library", "Application Support", "llmrelay", "bin", commandName)
	symlinkPath := filepath.Join(userHome, ".local", "bin", commandName)
	completionPath := filepath.Join(userHome, completionDirName, "_"+commandName)
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
