package main

import (
	"fmt"
	"os"

	"githooks/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
