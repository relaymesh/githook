package main

import (
	"fmt"
	githook "githook/cmd"
	"os"
)

func main() {
	if err := githook.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
