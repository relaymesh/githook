package main

import (
	"fmt"
	"os"

	"githooks/cmd/githooks"
)

func main() {
	if err := githooks.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
