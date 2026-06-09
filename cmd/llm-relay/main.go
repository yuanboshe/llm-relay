package main

import (
	"fmt"
	"os"

	"github.com/yuanboshe/llm-relay/internal/cmd"
)

func main() {
	if err := cmd.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "llm-relay command failed: %v\n", err)
		os.Exit(1)
	}
}
