package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/relay"
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
		newVersionCommand(),
		newConfigCommand(),
		newServeCommand(),
		newCompletionCommand(root),
	)

	return root
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
