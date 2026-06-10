package cmd

import (
	"bufio"
	"bytes"
	"context"
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

func newUpstreamSetURLCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-url <base-url>",
		Short: "Set upstream base URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseURL, err := normalizeBaseURL(args[0])
			if err != nil {
				return err
			}
			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			cfg.Upstream.BaseURL = baseURL
			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "upstream base_url: %s\n", baseURL)
			return err
		},
	}
}

func newUpstreamSetKeyCommand(stdin io.Reader) *cobra.Command {
	var envName string
	var readStdin bool
	var prompt bool

	setKeyCmd := &cobra.Command{
		Use:   "set-key",
		Short: "Set upstream API key source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			if envName != "" {
				selected++
			}
			if readStdin {
				selected++
			}
			if prompt {
				selected++
			}
			if selected != 1 {
				return fmt.Errorf("exactly one of --env, --stdin, or --prompt is required")
			}

			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			switch {
			case envName != "":
				cfg.Upstream.APIKeySource = "env"
				cfg.Upstream.APIKeyEnv = envName
				cfg.Upstream.APIKey = ""
			case readStdin:
				key, err := readKeyFromStdin(stdin)
				if err != nil {
					return err
				}
				cfg.Upstream.APIKeySource = "inline"
				cfg.Upstream.APIKeyEnv = ""
				cfg.Upstream.APIKey = key
			case prompt:
				if _, err := fmt.Fprint(cmd.ErrOrStderr(), "API key: "); err != nil {
					return err
				}
				key, err := readKeyLine(stdin)
				if err != nil {
					return err
				}
				cfg.Upstream.APIKeySource = "inline"
				cfg.Upstream.APIKeyEnv = ""
				cfg.Upstream.APIKey = key
			}
			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "upstream API key updated")
			return err
		},
	}
	setKeyCmd.Flags().StringVar(&envName, "env", "", "environment variable containing upstream API key")
	setKeyCmd.Flags().BoolVar(&readStdin, "stdin", false, "read upstream API key from stdin")
	setKeyCmd.Flags().BoolVar(&prompt, "prompt", false, "prompt for upstream API key")

	return setKeyCmd
}

func newUpstreamTestCommand() *cobra.Command {
	var testPath string

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test upstream connectivity without printing secrets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			if cfg.Upstream.BaseURL == "" {
				return fmt.Errorf("upstream base_url is not configured")
			}
			key, err := resolveUpstreamKey(cfg)
			if err != nil {
				return err
			}
			targetURL, err := joinBaseURLPath(cfg.Upstream.BaseURL, testPath)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodGet, targetURL, nil)
			if err != nil {
				return err
			}
			if key != "" {
				req.Header.Set("Authorization", "Bearer "+key)
			}
			client := http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("upstream request failed: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				return fmt.Errorf("upstream returned HTTP %d; check upstream API key", resp.StatusCode)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "upstream: %s\npath: %s\nstatus: %d\n", cfg.Upstream.BaseURL, testPath, resp.StatusCode); err != nil {
				return err
			}
			if resp.StatusCode == http.StatusNotFound {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "hint: upstream returned 404; if base_url already ends with /v1, try --path /models")
			}
			return err
		},
	}
	testCmd.Flags().StringVar(&testPath, "path", "/v1/models", "upstream path to request")

	return testCmd
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
	}

	tokenCmd.AddCommand(
		newTokenCreateCommand(),
		newTokenListCommand(),
		newTokenInspectCommand(),
		newTokenVerifyCommand(),
		newTokenEnableCommand(true),
		newTokenEnableCommand(false),
		newTokenDeleteCommand(),
		newTokenRotateCommand(),
	)

	return tokenCmd
}

func newTokenCreateCommand() *cobra.Command {
	var name string
	var note string

	createCmd := &cobra.Command{
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
			records = append(records, tokenstore.NewRecordWithMetadata(keyID, token, time.Now(), name, note))
			if err := store.Save(records); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "key-id: %s\nrelay token: %s\n", keyID, token)
			return err
		},
	}
	createCmd.Flags().StringVar(&name, "name", "", "human-readable token name")
	createCmd.Flags().StringVar(&note, "note", "", "human-readable token note")

	return createCmd
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
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "key-id\tname\tnote\tenabled\tcreated-at\trotated-at\ttoken-hash-prefix"); err != nil {
				return err
			}
			for _, record := range records {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t%s\t%t\t%s\t%s\t%s\n",
					record.KeyID,
					record.Name,
					record.Note,
					record.Enabled,
					record.CreatedAt,
					record.RotatedAt,
					tokenHashPrefix(record.TokenHash),
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newTokenInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <key-id>",
		Short: "Inspect relay token metadata",
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
				"key-id: %s\nname: %s\nnote: %s\nenabled: %t\ncreated-at: %s\nrotated-at: %s\ntoken-hash-prefix: %s\n",
				record.KeyID,
				record.Name,
				record.Note,
				record.Enabled,
				record.CreatedAt,
				record.RotatedAt,
				tokenHashPrefix(record.TokenHash),
			)
			return err
		},
	}
}

func newTokenVerifyCommand() *cobra.Command {
	var fromStdin bool

	verifyCmd := &cobra.Command{
		Use:   "verify <key-id>",
		Short: "Verify a relay token against local store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !fromStdin {
				return fmt.Errorf("use --stdin to read a token for verification")
			}
			_, records, err := loadTokenRecords()
			if err != nil {
				return err
			}
			_, record, err := tokenstore.Find(records, args[0])
			if err != nil {
				return err
			}
			token, err := readKeyFromStdin(cmd.InOrStdin())
			if err != nil {
				return err
			}
			if !tokenstore.RecordMatchesToken(record, token) {
				return fmt.Errorf("token is invalid")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "token: valid")
			return err
		},
	}
	verifyCmd.Flags().BoolVar(&fromStdin, "stdin", false, "read relay token from stdin")

	return verifyCmd
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
	return &cobra.Command{
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
				SourcePath:    sourcePath,
				UserHome:      os.Getenv("LLMRELAY_INSTALL_HOME"),
				ZshrcPath:     os.Getenv("LLMRELAY_ZSHRC"),
				ConfigPaths:   paths,
				ZshCompletion: completion.String(),
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
			_, err = fmt.Fprintf(out, "zshrc: updated %s\n", result.ZshrcPath)
			return err
		},
	}
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
			baseURL, err := promptLine(cmd.ErrOrStderr(), reader, "upstream base_url: ")
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
			apiKey, err := promptLine(cmd.ErrOrStderr(), reader, "upstream API key: ")
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
				token, err := promptLine(cmd.ErrOrStderr(), reader, "Cloudflare tunnel token: ")
				if err != nil {
					return err
				}
				if token == "" {
					return fmt.Errorf("Cloudflare tunnel token is required")
				}
				return installCloudflaredService(token)
			}
			return nil
		},
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
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionString(Version, Commit, BuildDate))
			return err
		},
	}
}

func versionString(version string, commit string, buildDate string) string {
	if commit == "" && buildDate == "" {
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
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "config: ok")
			return err
		},
	})
	configCmd.AddCommand(
		newUpstreamSetURLCommand(),
		newUpstreamSetKeyCommand(stdin),
		newUpstreamTestCommand(),
	)

	return configCmd
}

func validateConfig(paths config.Paths, cfg config.Config) error {
	if err := checkFileMode(paths.ConfigFile); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return fmt.Errorf("listen_addr is empty")
	}
	if cfg.Upstream.BaseURL == "" {
		return fmt.Errorf("upstream base_url is not configured; run llmrelay config set-url <base-url>")
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
	var baseURL string
	var keyID string
	var testPath string

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test the relay endpoint with a local relay token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, paths, err := loadConfig()
			if err != nil {
				return err
			}
			store := tokenstore.New(paths.TokenFile)
			records, err := store.Load()
			if err != nil {
				return err
			}
			selected, err := selectRelayTestToken(records, keyID)
			if err != nil {
				return err
			}
			if baseURL == "" {
				baseURL, err = relayLocalBaseURL(cfg.ListenAddr)
				if err != nil {
					return err
				}
			}
			targetURL, err := joinRelayBaseURLPath(baseURL, testPath)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodGet, targetURL, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+selected.Token)
			client := http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("relay request failed; run llmrelay start and retry: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				return fmt.Errorf("relay returned HTTP 401; selected relay token was rejected")
			}
			if resp.StatusCode == http.StatusForbidden {
				return fmt.Errorf("relay returned HTTP 403; selected relay token is disabled")
			}
			if resp.StatusCode == http.StatusBadGateway {
				return fmt.Errorf("relay returned HTTP 502; run llmrelay config test --path /models to check upstream")
			}
			openAIBase, anthropicBase, err := relayClientBaseURLs(baseURL)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(out, "relay: ok\nurl: %s\nstatus: %d\nkey-id: %s\n\n", targetURL, resp.StatusCode, selected.KeyID); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "OpenAI-compatible base_url:\n  %s\n\n", openAIBase); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Anthropic-compatible base_url:\n  %s\n", anthropicBase)
			return err
		},
	}
	testCmd.Flags().StringVar(&baseURL, "url", "", "relay base URL to test")
	testCmd.Flags().StringVar(&keyID, "key-id", "", "relay token key-id to use")
	testCmd.Flags().StringVar(&testPath, "path", "/v1/models", "relay path to request")
	return testCmd
}

func selectRelayTestToken(records []tokenstore.Record, keyID string) (tokenstore.Record, error) {
	if keyID != "" {
		_, record, err := tokenstore.Find(records, keyID)
		if err != nil {
			return tokenstore.Record{}, err
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

func relayClientBaseURLs(baseURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("relay URL must use http or https and include host")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "/v1" {
		openAI := parsed.String()
		parsed.Path = ""
		return openAI, parsed.String(), nil
	}
	anthropic := parsed.String()
	parsed.Path = basePath + "/v1"
	return parsed.String(), anthropic, nil
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
