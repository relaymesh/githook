package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	providerFlagDescription = "Provider (github|gitlab|bitbucket)"
	driverNameDescription   = "Driver name (amqp|nats|kafka)"
	stateIDDescription      = "State ID filter (optional)"
)

func printJSON(value interface{}) error {
	return writeJSON(os.Stdout, value)
}

func writeJSON(w io.Writer, value interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("encode json output: %w", err)
	}
	return nil
}

func printStdoutf(format string, args ...interface{}) error {
	_, err := fmt.Fprintf(os.Stdout, format, args...)
	if err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}

func requireNonEmpty(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func hideDeprecatedAlias(cmd *cobra.Command, message string) {
	cmd.Hidden = true
	cmd.Deprecated = message
}
