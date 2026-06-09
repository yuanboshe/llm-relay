package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/relay"
)

const version = "dev"

// Run executes the llm-relay CLI.
func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return nil
	}

	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(stdout, version)
		return err
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runConfig(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: llm-relay config path")
		return errors.New("missing config subcommand")
	}

	switch args[0] {
	case "path":
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, paths.ConfigFile)
		return err
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func runServe(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)

	addr := fs.String("addr", "127.0.0.1:8080", "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	server := relay.NewServer(*addr)
	_, err := fmt.Fprintf(stdout, "llm-relay server would listen on %s with routes %v\n", server.Addr(), server.Routes())
	return err
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: llm-relay <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  version       print version")
	fmt.Fprintln(w, "  config path   print default config path")
	fmt.Fprintln(w, "  serve         start relay server")
}
