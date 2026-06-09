package cmd

import (
	"fmt"
	"io"
	"os"
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
		newServeCommand(),
		newCompletionCommand(root),
	)

	return root
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
			for _, record := range records {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\tenabled=%t\tcreated=%s\trotated=%s\n", record.KeyID, record.Enabled, record.CreatedAt, record.RotatedAt)
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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "key-id: %s\nenabled: %t\ncreated-at: %s\nrotated-at: %s\n", record.KeyID, record.Enabled, record.CreatedAt, record.RotatedAt)
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

	return configCmd
}

func newServeCommand() *cobra.Command {
	var addr string

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start relay server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := relay.NewServer(addr)
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "llmrelay server would listen on %s with routes %v\n", server.Addr(), server.Routes())
			return err
		},
	}
	serveCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "HTTP listen address")

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
