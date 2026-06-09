package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/relay"
	"github.com/yuanboshe/llm-relay/internal/tokenstore"
)

const version = "dev"

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
		newInitCommand(),
		newVersionCommand(),
		newConfigCommand(),
		newTokenCommand(),
		newUpstreamCommand(stdin),
		newServeCommand(),
		newCompletionCommand(root),
		newDoctorCommand(),
	)

	return root
}

func newUpstreamCommand(stdin io.Reader) *cobra.Command {
	upstreamCmd := &cobra.Command{
		Use:   "upstream",
		Short: "Manage upstream provider configuration",
	}

	upstreamCmd.AddCommand(
		newUpstreamShowCommand(),
		newUpstreamSetURLCommand(),
		newUpstreamSetKeyCommand(stdin),
		newUpstreamTestCommand(),
	)

	return upstreamCmd
}

func newUpstreamShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show upstream configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), config.FormatRedacted(cfg))
			return err
		},
	}
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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "upstream: %s\npath: %s\nstatus: %d\n", cfg.Upstream.BaseURL, testPath, resp.StatusCode)
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
		Short: "Verify a relay token against local hash",
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
			if !tokenstore.VerifyToken(token, record.TokenHash) {
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

func newInitCommand() *cobra.Command {
	var force bool

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize local configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			if err := config.Init(paths, force); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "initialized llmrelay config at %s\n", paths.ConfigFile)
			return err
		},
	}
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")

	return initCmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local configuration",
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print default config path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), paths.ConfigFile)
			return err
		},
	})
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
		return fmt.Errorf("upstream base_url is not configured; run llmrelay upstream set-url <base-url>")
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
		Short: "Placeholder for the future HTTP relay server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := relay.NewServer(addr)
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "llmrelay server would listen on %s with routes %v\n", server.Addr(), server.Routes())
			return err
		},
	}
	serveCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:18080", "HTTP listen address")

	return serveCmd
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
				_, _ = fmt.Fprintln(out, "tunnel: enabled")
			} else {
				_, _ = fmt.Fprintln(out, "tunnel: disabled")
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
