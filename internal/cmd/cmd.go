package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/install"
	"github.com/yuanboshe/llm-relay/internal/relay"
	"github.com/yuanboshe/llm-relay/internal/service"
	"github.com/yuanboshe/llm-relay/internal/tokenstore"
	"github.com/yuanboshe/llm-relay/internal/tunnel"
)

var (
	Version   = "v0.0.0"
	Commit    = ""
	BuildDate = ""
)

type externalRunner interface {
	Run(name string, args ...string) (string, error)
}

type execExternalRunner struct{}

func (execExternalRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

var setupExternalRunner externalRunner = execExternalRunner{}
var setupGOOS = func() string { return runtime.GOOS }

// Run executes the llmrelay CLI.
func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	return RunWithIO(args, os.Stdin, stdout, stderr)
}

// RunWithIO executes the llmrelay CLI with explicit streams for tests.
func RunWithIO(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	root := NewRootCommand(stdin)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		root.SetOut(stderr)
		_ = root.Usage()
		return err
	}
	return nil
}

// NewRootCommand creates a fresh Cobra command tree.
func NewRootCommand(stdin io.Reader) *cobra.Command {
	root := &cobra.Command{
		Use:           "llmrelay",
		Short:         "Local LLM API relay",
		SilenceErrors: true,
		SilenceUsage:  false,
	}

	root.AddCommand(
		newInstallCommand(root),
		newUninstallCommand(stdin),
		newSetupCommand(stdin),
		newVersionCommand(),
		newConfigCommand(stdin),
		newTokenCommand(),
		newServeCommand(),
		newRelayTestCommand(),
		newStartCommand(),
		newStopCommand(),
		newRestartCommand(),
		newStatusCommand(),
		newLogsCommand(),
		newCompletionCommand(root),
		newDoctorCommand(),
	)

	return root
}

func loadConfig() (config.Config, config.Paths, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return config.Config{}, config.Paths{}, err
	}
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return config.Config{}, config.Paths{}, err
	}
	return cfg, paths, nil
}

func normalizeBaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("base URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("base URL must include host")
	}
	return strings.TrimRight(value, "/"), nil
}

func joinBaseURLPath(baseURL string, path string) (string, error) {
	if path == "" {
		path = "/v1/models"
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path must start with /")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("upstream base_url is invalid")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func readKeyFromStdin(stdin io.Reader) (string, error) {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("API key is empty")
	}
	return key, nil
}

func readKeyLine(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	key := strings.TrimSpace(line)
	if key == "" {
		return "", fmt.Errorf("API key is empty")
	}
	return key, nil
}

func resolveUpstreamKey(cfg config.Config) (string, error) {
	switch cfg.Upstream.APIKeySource {
	case "":
		return "", nil
	case "inline":
		return cfg.Upstream.APIKey, nil
	case "env":
		if cfg.Upstream.APIKeyEnv == "" {
			return "", fmt.Errorf("upstream API key env name is empty")
		}
		value := os.Getenv(cfg.Upstream.APIKeyEnv)
		if value == "" {
			return "", fmt.Errorf("upstream API key env %s is not set", cfg.Upstream.APIKeyEnv)
		}
		return value, nil
	default:
		return "", fmt.Errorf("unknown upstream API key source: %s", cfg.Upstream.APIKeySource)
	}
}

func newTokenCommand() *cobra.Command {
	tokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Manage relay tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	tokenCmd.AddCommand(
		newTokenCreateCommand(),
		newTokenListCommand(),
		newTokenShowCommand(),
		newTokenEnableCommand(true),
		newTokenEnableCommand(false),
		newTokenDeleteCommand(),
		newTokenRotateCommand(),
	)

	return tokenCmd
}

func newTokenCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <key-id>",
		Short: "Create relay token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			keyID := args[0]
			if _, _, err := tokenstore.Find(records, keyID); err == nil {
				return fmt.Errorf("token already exists: %s", keyID)
			}
			token, err := tokenstore.GenerateToken()
			if err != nil {
				return err
			}
			records = append(records, tokenstore.NewRecord(keyID, token, time.Now()))
			if err := store.Save(records); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "key-id: %s\nrelay token: %s\n", keyID, token)
			return err
		},
	}
}

func newTokenListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List relay tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			if len(records) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "no tokens")
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "key-id\tenabled\tcreated-at\trotated-at\ttoken"); err != nil {
				return err
			}
			for _, record := range records {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%t\t%s\t%s\t%s\n",
					record.KeyID,
					record.Enabled,
					record.CreatedAt,
					record.RotatedAt,
					tokenDisplay(record),
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newTokenShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <key-id>",
		Short: "Show relay token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			_, record, err := tokenstore.Find(records, args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"key-id: %s\nenabled: %t\ncreated-at: %s\nrotated-at: %s\ntoken: %s\n",
				record.KeyID,
				record.Enabled,
				record.CreatedAt,
				record.RotatedAt,
				tokenDisplay(record),
			)
			return err
		},
	}
}

func newTokenEnableCommand(enable bool) *cobra.Command {
	use := "enable <key-id>"
	short := "Enable relay token"
	if !enable {
		use = "disable <key-id>"
		short = "Disable relay token"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			idx, record, err := tokenstore.Find(records, args[0])
			if err != nil {
				return err
			}
			record.Enabled = enable
			records[idx] = record
			if err := store.Save(records); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "key-id: %s\nenabled: %t\n", record.KeyID, record.Enabled)
			return err
		},
	}
}

func newTokenDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key-id>",
		Short: "Delete relay token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			idx, record, err := tokenstore.Find(records, args[0])
			if err != nil {
				return err
			}
			records = append(records[:idx], records[idx+1:]...)
			if err := store.Save(records); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "deleted token: %s\n", record.KeyID)
			return err
		},
	}
}

func newTokenRotateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate <key-id>",
		Short: "Rotate relay token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			idx, record, err := tokenstore.Find(records, args[0])
			if err != nil {
				return err
			}
			token, err := tokenstore.GenerateToken()
			if err != nil {
				return err
			}
			record.Token = token
			record.TokenHash = tokenstore.HashToken(token)
			record.RotatedAt = time.Now().UTC().Format(time.RFC3339)
			records[idx] = record
			if err := store.Save(records); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "key-id: %s\nrelay token: %s\n", record.KeyID, token)
			return err
		},
	}
}

func tokenHashPrefix(hash string) string {
	if len(hash) <= 16 {
		return hash
	}
	return hash[:16]
}

func tokenDisplay(record tokenstore.Record) string {
	if record.Token != "" {
		return record.Token
	}
	return "<legacy hash-only token; rotate to show plaintext>"
}

func loadTokenRecords() (*tokenstore.Store, []tokenstore.Record, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, nil, err
	}
	store := tokenstore.New(paths.TokenFile)
	records, err := store.Load()
	if err != nil {
		return nil, nil, err
	}
	return store, records, nil
}

func newInstallCommand(root *cobra.Command) *cobra.Command {
	var skipShellInit bool
	var skipCompletion bool
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install llmrelay for the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath, err := os.Executable()
			if err != nil {
				return err
			}
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			var completion bytes.Buffer
			if err := root.GenZshCompletion(&completion); err != nil {
				return err
			}
			result, err := install.Run(install.Options{
				SourcePath:     sourcePath,
				UserHome:       os.Getenv("LLMRELAY_INSTALL_HOME"),
				ZshrcPath:      os.Getenv("LLMRELAY_ZSHRC"),
				ConfigPaths:    paths,
				ZshCompletion:  completion.String(),
				SkipShellInit:  skipShellInit,
				SkipCompletion: skipCompletion,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(out, "installed: %s\n", result.InstalledPath); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "command: %s\n", result.SymlinkPath); err != nil {
				return err
			}
			if result.ConfigCreated {
				if _, err := fmt.Fprintf(out, "config: created %s\n", paths.ConfigFile); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(out, "config: kept %s\n", paths.ConfigFile); err != nil {
				return err
			}
			if result.TokenStoreCreated {
				if _, err := fmt.Fprintf(out, "tokens: created %s\n", paths.TokenFile); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(out, "tokens: kept %s\n", paths.TokenFile); err != nil {
				return err
			}
			if skipShellInit {
				_, err = fmt.Fprintf(out, "zshrc: skipped\n")
				return err
			}
			_, err = fmt.Fprintf(out, "zshrc: updated %s\n", result.ZshrcPath)
			return err
		},
	}
	installCmd.Flags().BoolVar(&skipShellInit, "skip-shell-init", false, "do not update shell startup files")
	installCmd.Flags().BoolVar(&skipCompletion, "skip-completion", false, "do not install zsh completion")
	return installCmd
}

func newUninstallCommand(stdin io.Reader) *cobra.Command {
	var yes bool
	var purge bool
	var removeCloudflared bool
	var dryRun bool
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove llmrelay from the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if !yes && !dryRun {
				reader := bufio.NewReader(cmd.InOrStdin())
				ok, err := promptYesNo(out, reader, "Remove llmrelay from this machine? [y/N]: ", false)
				if err != nil {
					return err
				}
				if !ok {
					_, err = fmt.Fprintln(out, "aborted")
					return err
				}
			}
			cfgPaths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			installHome := os.Getenv("LLMRELAY_INSTALL_HOME")
			if strings.TrimSpace(installHome) == "" {
				installHome, err = os.UserHomeDir()
				if err != nil {
					return err
				}
			}
			if dryRun {
				return printUninstallPlan(out, installHome, cfgPaths, purge, removeCloudflared)
			}
			manager, err := newBackgroundManager()
			if err != nil {
				return err
			}
			status, err := manager.Status()
			if err != nil {
				return err
			}
			if status.State == service.StateRunning || status.State == service.StateStale {
				status, err = manager.Stop()
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(out, "service: stopped"); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintln(out, "service: already stopped"); err != nil {
				return err
			}
			result, err := install.Uninstall(install.UninstallOptions{
				UserHome:    installHome,
				ConfigPaths: cfgPaths,
				Purge:       purge,
			})
			if err != nil {
				return err
			}
			if err := printUninstallResult(out, result); err != nil {
				return err
			}
			if removeCloudflared {
				if err := uninstallCloudflaredService(out); err != nil {
					return err
				}
			}
			return nil
		},
	}
	uninstallCmd.Flags().BoolVar(&yes, "yes", false, "do not prompt for confirmation")
	uninstallCmd.Flags().BoolVar(&purge, "purge", false, "remove config, token, and log data under ~/.llmrelay")
	uninstallCmd.Flags().BoolVar(&removeCloudflared, "remove-cloudflared", false, "remove the macOS cloudflared service if present")
	uninstallCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without changing files")
	return uninstallCmd
}

func printUninstallPlan(out io.Writer, installHome string, cfgPaths config.Paths, purge bool, removeCloudflared bool) error {
	installedPath := filepath.Join(installHome, "Library", "Application Support", "llmrelay", "bin", "llmrelay")
	symlinkPath := filepath.Join(installHome, ".local", "bin", "llmrelay")
	completionPath := filepath.Join(installHome, ".zsh", "completions", "_llmrelay")
	if _, err := fmt.Fprintln(out, "dry-run: uninstall plan"); err != nil {
		return err
	}
	for _, line := range []string{
		"would stop background service",
		"would remove: " + installedPath,
		"would remove: " + symlinkPath,
		"would remove: " + completionPath,
		"would remove shell integration from: " + filepath.Join(installHome, ".zshrc"),
	} {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	if purge {
		if _, err := fmt.Fprintln(out, "would purge: "+cfgPaths.Dir); err != nil {
			return err
		}
	}
	if removeCloudflared {
		if _, err := fmt.Fprintln(out, "would remove cloudflared service"); err != nil {
			return err
		}
	}
	return nil
}

func printUninstallResult(out io.Writer, result install.UninstallResult) error {
	lines := []string{}
	if result.RemovedInstalled {
		lines = append(lines, "removed: "+result.InstalledPath)
	}
	if result.RemovedSymlink {
		lines = append(lines, "removed: "+result.SymlinkPath)
	}
	if result.RemovedCompletion {
		lines = append(lines, "removed: "+result.CompletionPath)
	}
	if result.RemovedZshrc {
		lines = append(lines, "removed shell integration: "+result.ZshrcPath)
	}
	if result.Purged {
		lines = append(lines, "purged: "+result.ConfigDir)
	} else {
		lines = append(lines, "kept: "+result.ConfigDir)
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}

func uninstallCloudflaredService(out io.Writer) error {
	if runtime.GOOS != "darwin" {
		_, err := fmt.Fprintln(out, "cloudflared: skipped (macOS only)")
		return err
	}
	if _, err := setupExternalRunner.Run("sudo", "cloudflared", "service", "uninstall"); err != nil {
		return fmt.Errorf("cloudflared uninstall failed: %w", err)
	}
	_, err := fmt.Fprintln(out, "cloudflared: removed")
	return err
}

func newSetupCommand(stdin io.Reader) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run the first-time setup wizard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			if _, _, err := config.Ensure(paths); err != nil {
				return err
			}
			cfg, err := config.Load(paths.ConfigFile)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(stdin)
			if err := printSetupSummary(cmd.OutOrStdout(), cfg); err != nil {
				return err
			}
			baseURL, err := promptConfigString(cmd.ErrOrStderr(), reader, "upstream base_url", cfg.Upstream.BaseURL)
			if err != nil {
				return err
			}
			if baseURL != "" {
				normalized, err := normalizeBaseURL(baseURL)
				if err != nil {
					return err
				}
				cfg.Upstream.BaseURL = normalized
			}
			apiKey, err := promptSecret(cmd.ErrOrStderr(), reader, "upstream API key", cfg.Upstream.APIKey != "" || cfg.Upstream.APIKeySource == "env")
			if err != nil {
				return err
			}
			if apiKey != "" {
				cfg.Upstream.APIKeySource = "inline"
				cfg.Upstream.APIKeyEnv = ""
				cfg.Upstream.APIKey = apiKey
			}
			keyID, err := promptLine(cmd.ErrOrStderr(), reader, "relay token key-id [local]: ")
			if err != nil {
				return err
			}
			if keyID == "" {
				keyID = "local"
			}
			store := tokenstore.New(paths.TokenFile)
			records, err := store.Load()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if _, _, err := tokenstore.Find(records, keyID); err == nil {
				if _, err := fmt.Fprintf(out, "relay token: kept existing key-id %s\n", keyID); err != nil {
					return err
				}
			} else {
				token, err := tokenstore.GenerateToken()
				if err != nil {
					return err
				}
				records = append(records, tokenstore.NewRecord(keyID, token, time.Now()))
				if err := store.Save(records); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(out, "key-id: %s\nrelay token: %s\n", keyID, token); err != nil {
					return err
				}
			}

			remoteAccess, err := promptLine(cmd.ErrOrStderr(), reader, "remote access [cloudflare/none/ssh] (cloudflare): ")
			if err != nil {
				return err
			}
			if remoteAccess == "" {
				remoteAccess = "cloudflare"
			}
			switch strings.ToLower(remoteAccess) {
			case "none":
				cfg.Tunnel.Enabled = false
			case "cloudflare":
				cfg.Tunnel.Enabled = false
			case "ssh":
				if err := promptSSHTunnelConfig(cmd.ErrOrStderr(), reader, &cfg); err != nil {
					return err
				}
			default:
				return fmt.Errorf("remote access must be cloudflare, none, or ssh")
			}
			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				return err
			}
			if strings.ToLower(remoteAccess) == "cloudflare" {
				return maybeInstallCloudflaredService(cmd.ErrOrStderr(), reader)
			}
			return nil
		},
	}
}

func printSetupSummary(out io.Writer, cfg config.Config) error {
	if _, err := fmt.Fprintln(out, "current configuration:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  listen_addr: %s\n", emptyDisplay(cfg.ListenAddr)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  upstream base_url: %s\n", emptyDisplay(cfg.Upstream.BaseURL)); err != nil {
		return err
	}
	keyState := "not configured"
	if cfg.Upstream.APIKeySource == "env" && cfg.Upstream.APIKeyEnv != "" {
		keyState = "env:" + cfg.Upstream.APIKeyEnv
	} else if cfg.Upstream.APIKey != "" {
		keyState = "inline configured"
	}
	if _, err := fmt.Fprintf(out, "  upstream API key: %s\n", keyState); err != nil {
		return err
	}
	if cfg.Tunnel.Enabled {
		_, err := fmt.Fprintf(out, "  ssh-tunnel: enabled (%s:%s)\n", cfg.Tunnel.RemoteHost, cfg.Tunnel.RemotePort)
		return err
	}
	_, err := fmt.Fprintln(out, "  ssh-tunnel: disabled")
	return err
}

func emptyDisplay(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<not configured>"
	}
	return value
}

func promptConfigString(out io.Writer, reader *bufio.Reader, label string, current string) (string, error) {
	if current == "" {
		return promptLine(out, reader, label+": ")
	}
	update, err := promptYesNo(out, reader, fmt.Sprintf("%s is %s; update? [y/N]: ", label, current), false)
	if err != nil || !update {
		return "", err
	}
	return promptLine(out, reader, "new "+label+": ")
}

func promptSecret(out io.Writer, reader *bufio.Reader, label string, configured bool) (string, error) {
	if !configured {
		return promptLine(out, reader, label+": ")
	}
	update, err := promptYesNo(out, reader, label+" is configured; update? [y/N]: ", false)
	if err != nil || !update {
		return "", err
	}
	return promptLine(out, reader, "new "+label+": ")
}

func promptYesNo(out io.Writer, reader *bufio.Reader, prompt string, defaultYes bool) (bool, error) {
	answer, err := promptLine(out, reader, prompt)
	if err != nil {
		return false, err
	}
	if answer == "" {
		return defaultYes, nil
	}
	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("answer must be yes or no")
	}
}

func promptSSHTunnelConfig(out io.Writer, reader *bufio.Reader, cfg *config.Config) error {
	sshHost, err := promptLine(out, reader, "ssh host: ")
	if err != nil {
		return err
	}
	sshUser, err := promptLine(out, reader, "ssh user: ")
	if err != nil {
		return err
	}
	sshPort, err := promptLine(out, reader, "ssh port [22]: ")
	if err != nil {
		return err
	}
	if sshPort == "" {
		sshPort = "22"
	}
	remoteHost, err := promptLine(out, reader, "remote bind host [127.0.0.1]: ")
	if err != nil {
		return err
	}
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}
	remotePort, err := promptLine(out, reader, "remote bind port [18080]: ")
	if err != nil {
		return err
	}
	if remotePort == "" {
		remotePort = "18080"
	}
	cfg.Tunnel.Enabled = true
	cfg.Tunnel.SSHHost = sshHost
	cfg.Tunnel.SSHUser = sshUser
	cfg.Tunnel.SSHPort = sshPort
	cfg.Tunnel.RemoteHost = remoteHost
	cfg.Tunnel.RemotePort = remotePort
	return nil
}

func installCloudflaredService(token string) error {
	if setupGOOS() != "darwin" {
		return fmt.Errorf("cloudflared service auto-install is only supported on macOS; install cloudflared manually and run cloudflared service install <TUNNEL_TOKEN>")
	}
	commands := []struct {
		name string
		args []string
	}{
		{name: "brew", args: []string{"install", "cloudflared"}},
		{name: "sudo", args: []string{"cloudflared", "service", "install", token}},
		{name: "sudo", args: []string{"launchctl", "start", "com.cloudflare.cloudflared"}},
	}
	for _, command := range commands {
		if _, err := setupExternalRunner.Run(command.name, command.args...); err != nil {
			return fmt.Errorf("%s failed: %w", command.name, err)
		}
	}
	return nil
}

func maybeInstallCloudflaredService(out io.Writer, reader *bufio.Reader) error {
	if cloudflaredServiceInstalled() {
		update, err := promptYesNo(out, reader, "Cloudflare connector is already installed; update token? [y/N]: ", false)
		if err != nil || !update {
			return err
		}
		token, err := promptLine(out, reader, "Cloudflare tunnel token: ")
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("Cloudflare tunnel token is required")
		}
		return reinstallCloudflaredService(token)
	}
	token, err := promptLine(out, reader, "Cloudflare tunnel token: ")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("Cloudflare tunnel token is required")
	}
	return installCloudflaredService(token)
}

func cloudflaredServiceInstalled() bool {
	if setupGOOS() != "darwin" {
		return false
	}
	out, err := setupExternalRunner.Run("cloudflared", "--version")
	if err != nil || strings.TrimSpace(out) == "" {
		return false
	}
	if _, err := setupExternalRunner.Run("launchctl", "print", "system/com.cloudflare.cloudflared"); err == nil {
		return true
	}
	if _, err := setupExternalRunner.Run("launchctl", "print", "gui/"+fmt.Sprint(os.Getuid())+"/com.cloudflare.cloudflared"); err == nil {
		return true
	}
	return false
}

func reinstallCloudflaredService(token string) error {
	if setupGOOS() != "darwin" {
		return fmt.Errorf("cloudflared service auto-install is only supported on macOS; install cloudflared manually and run cloudflared service install <TUNNEL_TOKEN>")
	}
	_, _ = setupExternalRunner.Run("sudo", "cloudflared", "service", "uninstall")
	if _, err := setupExternalRunner.Run("sudo", "cloudflared", "service", "install", token); err != nil {
		return fmt.Errorf("sudo failed: %w", err)
	}
	if _, err := setupExternalRunner.Run("sudo", "launchctl", "start", "com.cloudflare.cloudflared"); err != nil {
		return fmt.Errorf("sudo failed: %w", err)
	}
	return nil
}

func promptLine(out io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func newVersionCommand() *cobra.Command {
	var verbose bool
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionString(Version, Commit, BuildDate, verbose))
			return err
		},
	}
	versionCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print build metadata")
	return versionCmd
}

func versionString(version string, commit string, buildDate string, verbose bool) string {
	if !verbose || (commit == "" && buildDate == "") {
		return version
	}
	var lines []string
	lines = append(lines, version)
	if commit != "" {
		lines = append(lines, "commit: "+commit)
	}
	if buildDate != "" {
		lines = append(lines, "date: "+buildDate)
	}
	return strings.Join(lines, "\n")
}

func newConfigCommand(stdin io.Reader) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show local configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.Load(paths.ConfigFile)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), config.FormatRedacted(cfg))
			return err
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate local configuration without network requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := config.Load(paths.ConfigFile)
			if err != nil {
				return err
			}
			if err := validateConfig(paths, cfg); err != nil {
				return err
			}
			for _, key := range config.UnknownKeys(cfg) {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "warning: unknown config key %s\n", key); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "config: ok")
			return err
		},
	})
	configCmd.AddCommand(
		newConfigSetCommand(stdin),
	)

	return configCmd
}

func newConfigSetCommand(stdin io.Reader) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set a local configuration value",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
				return fmt.Errorf("config key must be a dotted path")
			}
			if err := rejectRemovedConfigSetKey(key); err != nil {
				return err
			}
			var rawValue string
			var err error
			if len(args) == 2 {
				rawValue = args[1]
				if rawValue == "-" {
					rawValue, err = readKeyFromStdin(stdin)
					if err != nil {
						return err
					}
				}
			} else {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s: ", key); err != nil {
					return err
				}
				rawValue, err = readKeyLine(stdin)
				if err != nil {
					return err
				}
			}
			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			if err := setConfigValue(&cfg, key, rawValue); err != nil {
				return err
			}
			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				return err
			}
			if key == "upstream.api_key" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "upstream.api_key: updated")
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, displayConfigSetValue(rawValue))
			return err
		},
	}
}

func rejectRemovedConfigSetKey(key string) error {
	switch key {
	case "public_url":
		return fmt.Errorf("public_url is no longer configured; pass the URL to llmrelay test <key-id> <url>")
	default:
		return nil
	}
}

func setConfigValue(cfg *config.Config, key string, rawValue string) error {
	if cfg.Extra == nil {
		cfg.Extra = map[string]any{}
	}
	switch key {
	case "listen_addr":
		cfg.ListenAddr = rawValue
	case "public_url":
		return rejectRemovedConfigSetKey(key)
	case "upstream.base_url":
		value, err := normalizeBaseURL(rawValue)
		if err != nil {
			return err
		}
		cfg.Upstream.BaseURL = value
	case "upstream.api_key":
		if strings.TrimSpace(rawValue) == "" {
			return fmt.Errorf("API key is empty")
		}
		cfg.Upstream.APIKeySource = "inline"
		cfg.Upstream.APIKeyEnv = ""
		cfg.Upstream.APIKey = strings.TrimSpace(rawValue)
	case "upstream.api_key_env":
		if strings.TrimSpace(rawValue) == "" {
			return fmt.Errorf("API key environment variable is empty")
		}
		cfg.Upstream.APIKeySource = "env"
		cfg.Upstream.APIKeyEnv = strings.TrimSpace(rawValue)
		cfg.Upstream.APIKey = ""
	case "upstream.api_key_source":
		cfg.Upstream.APIKeySource = rawValue
	case "tunnel.enabled":
		value, err := parseConfigBool(rawValue)
		if err != nil {
			return err
		}
		cfg.Tunnel.Enabled = value
	case "tunnel.ssh_host":
		cfg.Tunnel.SSHHost = rawValue
	case "tunnel.ssh_user":
		cfg.Tunnel.SSHUser = rawValue
	case "tunnel.ssh_port":
		cfg.Tunnel.SSHPort = rawValue
	case "tunnel.remote_host":
		cfg.Tunnel.RemoteHost = rawValue
	case "tunnel.remote_port":
		cfg.Tunnel.RemotePort = rawValue
	default:
		cfg.Extra[key] = parseConfigValue(rawValue)
		return nil
	}
	delete(cfg.Extra, key)
	return nil
}

func parseConfigValue(value string) any {
	if parsed, err := parseConfigBool(value); err == nil {
		return parsed
	}
	return value
}

func parseConfigBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("boolean value must be true or false")
	}
}

func displayConfigSetValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<empty>"
	}
	return value
}

func validateConfig(paths config.Paths, cfg config.Config) error {
	if err := checkFileMode(paths.ConfigFile); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return fmt.Errorf("listen_addr is empty")
	}
	if cfg.Upstream.BaseURL == "" {
		return fmt.Errorf("upstream base_url is not configured; run llmrelay config set upstream.base_url <base-url>")
	}
	if _, err := normalizeBaseURL(cfg.Upstream.BaseURL); err != nil {
		return err
	}
	if _, err := resolveUpstreamKey(cfg); err != nil {
		return err
	}
	if cfg.Tunnel.Enabled {
		if strings.TrimSpace(cfg.Tunnel.SSHHost) == "" {
			return fmt.Errorf("tunnel ssh_host is empty")
		}
		if strings.TrimSpace(cfg.Tunnel.SSHUser) == "" {
			return fmt.Errorf("tunnel ssh_user is empty")
		}
		if strings.TrimSpace(cfg.Tunnel.SSHPort) == "" {
			return fmt.Errorf("tunnel ssh_port is empty")
		}
		if strings.TrimSpace(cfg.Tunnel.RemoteHost) == "" {
			return fmt.Errorf("tunnel remote_host is empty")
		}
		if strings.TrimSpace(cfg.Tunnel.RemotePort) == "" {
			return fmt.Errorf("tunnel remote_port is empty")
		}
	}
	return nil
}

func checkFileMode(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s permissions are too broad", path)
	}
	return nil
}

func newServeCommand() *cobra.Command {
	var addr string

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP relay server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			if addr != "" {
				cfg.ListenAddr = addr
			}
			key, err := resolveUpstreamKey(cfg)
			if err != nil {
				return err
			}
			cfg.Upstream.APIKey = key
			store := tokenstore.New(paths.TokenFile)
			records, err := store.Load()
			if err != nil {
				return err
			}
			server := relay.NewProxyServer(relay.Options{
				Addr:   cfg.ListenAddr,
				Config: cfg,
				Tokens: records,
			})
			serveCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if cfg.Tunnel.Enabled {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "ssh reverse tunnel enabled: %s:%s\n", cfg.Tunnel.RemoteHost, cfg.Tunnel.RemotePort)
				supervisor := &tunnel.Supervisor{}
				go func() {
					if err := supervisor.Run(serveCtx, cfg, cmd.ErrOrStderr()); err != nil && err != context.Canceled {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "ssh tunnel supervisor exited: %v\n", err)
					}
				}()
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "llmrelay serving on %s\n", server.Addr())
			return server.ListenAndServe(serveCtx)
		},
	}
	serveCmd.Flags().StringVar(&addr, "addr", "", "override HTTP listen address")

	return serveCmd
}

func newRelayTestCommand() *cobra.Command {
	testCmd := &cobra.Command{
		Use:   "test [key-id] [url]",
		Short: "Test upstream connectivity or a relay token",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return runUpstreamTest(cmd.OutOrStdout(), cfg)
			}
			baseURL := ""
			if len(args) == 2 {
				baseURL = args[1]
			}
			return runRelayTest(cmd.OutOrStdout(), cfg, paths, args[0], baseURL)
		},
	}
	testCmd.AddCommand(&cobra.Command{
		Use:   "upstream",
		Short: "Test upstream connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			return runUpstreamTest(cmd.OutOrStdout(), cfg)
		},
	})
	return testCmd
}

func runUpstreamTest(out io.Writer, cfg config.Config) error {
	if cfg.Upstream.BaseURL == "" {
		return fmt.Errorf("upstream base_url is not configured")
	}
	key, err := resolveUpstreamKey(cfg)
	if err != nil {
		return err
	}
	resp, targetURL, err := requestFirstOK(cfg.Upstream.BaseURL, upstreamCandidatePaths(cfg.Upstream.BaseURL), key)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("upstream returned HTTP %d; check upstream API key", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("upstream returned HTTP 404 after trying /models and /v1/models")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	if _, err := fmt.Fprintf(out, "upstream ok\nurl: %s\n", targetURL); err != nil {
		return err
	}
	if err := printModelSamples(out, resp.Body); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}

func requestFirstOK(baseURL string, paths []string, bearerToken string) (*http.Response, string, error) {
	client := http.Client{Timeout: 5 * time.Second}
	var lastResp *http.Response
	var lastURL string
	for _, path := range paths {
		targetURL, err := joinBaseURLPath(baseURL, path)
		if err != nil {
			return nil, "", err
		}
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, "", err
		}
		if bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+bearerToken)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("upstream request failed: %w", err)
		}
		if lastResp != nil {
			_ = lastResp.Body.Close()
		}
		lastResp = resp
		lastURL = targetURL
		if resp.StatusCode != http.StatusNotFound {
			return resp, targetURL, nil
		}
	}
	return lastResp, lastURL, nil
}

func upstreamCandidatePaths(baseURL string) []string {
	parsed, err := url.Parse(baseURL)
	if err == nil && strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/v1") {
		return []string{"/models", "/v1/models"}
	}
	return []string{"/v1/models", "/models"}
}

func modelSamples(body io.Reader, limit int) ([]string, int) {
	if body == nil || limit <= 0 {
		return nil, 0
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&payload); err != nil {
		return nil, 0
	}
	samples := make([]string, 0, limit)
	total := 0
	for _, model := range payload.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		total++
		if len(samples) < limit {
			samples = append(samples, id)
		}
	}
	return samples, total
}

func printModelSamples(out io.Writer, body io.Reader) error {
	samples, total := modelSamples(body, 3)
	if len(samples) == 0 {
		_, err := fmt.Fprintln(out, "models: unavailable")
		return err
	}
	line := strings.Join(samples, ", ")
	if total > len(samples) {
		line += fmt.Sprintf(" (+%d more)", total-len(samples))
	}
	_, err := fmt.Fprintf(out, "models: %s\n", line)
	return err
}

func runRelayTest(out io.Writer, cfg config.Config, paths config.Paths, keyID string, baseURL string) error {
	store := tokenstore.New(paths.TokenFile)
	records, err := store.Load()
	if err != nil {
		return err
	}
	selected, err := selectRelayTestToken(records, keyID)
	if err != nil {
		return err
	}
	localTest := strings.TrimSpace(baseURL) == ""
	if baseURL == "" {
		baseURL, err = relayLocalBaseURL(cfg.ListenAddr)
		if err != nil {
			return err
		}
	}
	resp, targetURL, err := requestRelay(baseURL, selected.Token)
	if err != nil {
		if localTest {
			return fmt.Errorf("relay request failed; run llmrelay start and retry: %w", err)
		}
		return fmt.Errorf("relay request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("relay returned HTTP 401; token %s was rejected", selected.KeyID)
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("relay returned HTTP 403; token %s is disabled", selected.KeyID)
	}
	if resp.StatusCode == http.StatusBadGateway {
		return fmt.Errorf("relay returned HTTP 502; run llmrelay test upstream to check upstream")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("relay returned HTTP %d", resp.StatusCode)
	}
	if _, err := fmt.Fprintf(out, "relay ok\nkey-id: %s\nurl: %s\n", selected.KeyID, targetURL); err != nil {
		return err
	}
	if err := printModelSamples(out, resp.Body); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}

func requestRelay(baseURL string, relayToken string) (*http.Response, string, error) {
	targetURL, err := joinRelayBaseURLPath(baseURL, "/v1/models")
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+relayToken)
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	return resp, targetURL, nil
}

func selectRelayTestToken(records []tokenstore.Record, keyID string) (tokenstore.Record, error) {
	if keyID != "" {
		_, record, err := tokenstore.Find(records, keyID)
		if err != nil {
			return tokenstore.Record{}, fmt.Errorf("relay token %s not found", keyID)
		}
		if !record.Enabled {
			return tokenstore.Record{}, fmt.Errorf("relay token %s is disabled", keyID)
		}
		if record.Token == "" {
			return tokenstore.Record{}, fmt.Errorf("relay token %s has no plaintext token; rotate or create a relay token", keyID)
		}
		return record, nil
	}
	for _, record := range records {
		if record.Enabled && record.Token != "" {
			return record, nil
		}
	}
	return tokenstore.Record{}, fmt.Errorf("no enabled relay token found; run llmrelay token create local")
}

func relayLocalBaseURL(listenAddr string) (string, error) {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:18080"
	}
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("invalid listen_addr: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func joinRelayBaseURLPath(baseURL string, path string) (string, error) {
	if path == "" {
		path = "/v1/models"
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path must start with /")
	}
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("relay URL must use http or https and include host")
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath != "" && strings.HasPrefix(path, basePath+"/") {
		parsed.Path = path
	} else {
		parsed.Path = basePath + path
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start llmrelay in the background",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newServiceManager()
			if err != nil {
				return err
			}
			status, err := manager.Start()
			if err != nil {
				return err
			}
			return printServiceStatus(cmd.OutOrStdout(), status)
		},
	}
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background llmrelay process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newServiceManager()
			if err != nil {
				return err
			}
			status, err := manager.Stop()
			if err != nil {
				return err
			}
			return printServiceStatus(cmd.OutOrStdout(), status)
		},
	}
}

func newRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the background llmrelay process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newServiceManager()
			if err != nil {
				return err
			}
			status, err := manager.Restart()
			if err != nil {
				return err
			}
			return printServiceStatus(cmd.OutOrStdout(), status)
		},
	}
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show background llmrelay process status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newServiceManager()
			if err != nil {
				return err
			}
			status, err := manager.Status()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if err := printServiceStatus(out, status); err != nil {
				return err
			}
			cfg, _, err := loadConfig()
			if err == nil {
				return printTunnelStatus(out, cfg)
			}
			return nil
		},
	}
}

func newLogsCommand() *cobra.Command {
	var tail int
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Print background llmrelay logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			out, err := service.TailLog(paths.LogFile, tail)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	logsCmd.Flags().IntVar(&tail, "tail", 100, "number of log lines to print")
	return logsCmd
}

type backgroundManager interface {
	Start() (service.Status, error)
	Stop() (service.Status, error)
	Restart() (service.Status, error)
	Status() (service.Status, error)
}

var newBackgroundManager = newServiceManager

func newServiceManager() (backgroundManager, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	executable := installedExecutablePath()
	if runtime.GOOS == "darwin" {
		manager, err := service.DefaultLaunchAgentManager(executable, paths.LogFile)
		if err != nil {
			return nil, err
		}
		return manager, nil
	}
	return service.Manager{
		PIDFile:    paths.PIDFile,
		LogFile:    paths.LogFile,
		Executable: executable,
	}, nil
}

func installedExecutablePath() string {
	home := os.Getenv("LLMRELAY_INSTALL_HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			executable, exeErr := os.Executable()
			if exeErr == nil {
				return executable
			}
			return ""
		}
	}
	installed := filepath.Join(home, "Library", "Application Support", "llmrelay", "bin", "llmrelay")
	if _, err := os.Stat(installed); err == nil {
		return installed
	}
	executable, err := os.Executable()
	if err != nil {
		return installed
	}
	return executable
}

func printServiceStatus(out io.Writer, status service.Status) error {
	if status.State == "" {
		status.State = service.StateStopped
	}
	if _, err := fmt.Fprintf(out, "state: %s\n", status.State); err != nil {
		return err
	}
	if status.PID > 0 {
		if _, err := fmt.Fprintf(out, "pid: %d\n", status.PID); err != nil {
			return err
		}
	}
	if status.PIDFile != "" {
		if _, err := fmt.Fprintf(out, "pid-file: %s\n", status.PIDFile); err != nil {
			return err
		}
	}
	if status.LogFile != "" {
		if _, err := fmt.Fprintf(out, "log-file: %s\n", status.LogFile); err != nil {
			return err
		}
	}
	if status.Message != "" {
		if _, err := fmt.Fprintf(out, "message: %s\n", status.Message); err != nil {
			return err
		}
	}
	return nil
}

func printTunnelStatus(out io.Writer, cfg config.Config) error {
	if cfg.Tunnel.Enabled {
		if _, err := fmt.Fprintln(out, "ssh-tunnel: enabled"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "ssh-tunnel-remote: %s:%s\n", cfg.Tunnel.RemoteHost, cfg.Tunnel.RemotePort); err != nil {
			return err
		}
		return nil
	}
	_, err := fmt.Fprintln(out, "ssh-tunnel: disabled")
	return err
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
	}

	completionCmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenBashCompletion(cmd.OutOrStdout())
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenZshCompletion(cmd.OutOrStdout())
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion script",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenPowerShellCompletion(cmd.OutOrStdout())
		},
	})

	return completionCmd
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local llmrelay environment without printing secrets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			var failed bool
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "home: %s\n", paths.Dir)
			if envHome := os.Getenv("LLMRELAY_HOME"); envHome != "" {
				_, _ = fmt.Fprintf(out, "LLMRELAY_HOME: %s\n", envHome)
			} else {
				_, _ = fmt.Fprintln(out, "LLMRELAY_HOME: not set")
			}

			if err := reportPathStatus(out, "config.toml", paths.ConfigFile); err != nil {
				failed = true
			}
			if err := reportPathStatus(out, "tokens.json", paths.TokenFile); err != nil {
				failed = true
			}

			cfg, err := config.Load(paths.ConfigFile)
			if err != nil {
				failed = true
				_, _ = fmt.Fprintf(out, "config: error (%v)\n", err)
			} else if err := validateConfig(paths, cfg); err != nil {
				failed = true
				_, _ = fmt.Fprintf(out, "config: error (%v)\n", err)
			} else {
				_, _ = fmt.Fprintln(out, "config: ok")
			}
			if cfg.Tunnel.Enabled {
				_, _ = fmt.Fprintln(out, "ssh-tunnel: enabled")
			} else {
				_, _ = fmt.Fprintln(out, "ssh-tunnel: disabled")
			}

			if err == nil {
				if _, err := resolveUpstreamKey(cfg); err != nil {
					failed = true
					_, _ = fmt.Fprintf(out, "upstream key: error (%v)\n", err)
				} else {
					_, _ = fmt.Fprintln(out, "upstream key: ok")
				}
			}

			store := tokenstore.New(paths.TokenFile)
			records, tokenErr := store.Load()
			if tokenErr != nil {
				failed = true
				_, _ = fmt.Fprintf(out, "tokens: error (%v)\n", tokenErr)
			} else {
				disabled := 0
				for _, record := range records {
					if !record.Enabled {
						disabled++
					}
				}
				_, _ = fmt.Fprintf(out, "tokens: %d total, %d disabled\n", len(records), disabled)
			}

			if failed {
				return fmt.Errorf("doctor found issues")
			}
			return nil
		},
	}
}

func reportPathStatus(out io.Writer, label string, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(out, "%s: missing (%s)\n", label, path)
			return err
		}
		_, _ = fmt.Fprintf(out, "%s: error (%v)\n", label, err)
		return err
	}
	if info.IsDir() {
		err := fmt.Errorf("is a directory")
		_, _ = fmt.Fprintf(out, "%s: error (%v)\n", label, err)
		return err
	}
	if err := checkFileMode(path); err != nil {
		_, _ = fmt.Fprintf(out, "%s: warning (%v)\n", label, err)
		return nil
	}
	_, _ = fmt.Fprintf(out, "%s: ok (%s)\n", label, path)
	return nil
}
