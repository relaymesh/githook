package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const initConfigTemplate = `server:
  port: 8080
  public_base_url: http://localhost:8080

endpoint: http://localhost:8080

providers:
  github:
    enabled: true
    webhook:
      secret: ${GITHUB_WEBHOOK_SECRET}

watermill:
  driver: gochannel
`

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create a starter config file",
		Example: "  githook init --config config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := strings.TrimSpace(configPath)
			if path == "" {
				return fmt.Errorf("config path is required")
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("config already exists: %s", path)
				}
			}
			if err := os.WriteFile(path, []byte(initConfigTemplate), 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote config to %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	return cmd
}
