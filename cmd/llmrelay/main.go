package main

import (
	"os"

	"github.com/yuanboshe/llm-relay/internal/relay"
)

func main() {
	os.Exit(relay.Run(os.Args[1:], os.Stdout, os.Stderr))
}
