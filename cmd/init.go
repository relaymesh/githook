package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	initConfigTemplateServer = `server:
  port: 8080

endpoint: http://localhost:8080

storage:
  driver: postgres
  dsn: postgres://user:pass@localhost:5432/githook?sslmode=disable
  auto_migrate: true

redirect_base_url: http://localhost:8080

auth:
  oauth2:
    enabled: false
`
	initConfigTemplateCLI = `endpoint: http://localhost:8080

auth:
  oauth2:
    enabled: false
`
	initConfigTemplateWorker = `endpoint: http://localhost:8080

auth:
  oauth2:
    enabled: false

relaybus:
  # driver: amqp
  # drivers: []
  # kafka:
  #   broker: ""
  #   brokers: []
  #   group_id: ""
  #   topic_prefix: ""
  #   max_messages: 0
  # nats:
  #   url: ""
  #   subject_prefix: ""
  #   max_messages: 0
  # amqp:
  #   url: ""
  #   exchange: ""
  #   routing_key_template: "{topic}"
  #   queue: ""
  #   auto_ack: false
  #   max_messages: 0
`
)

func newInitCmd() *cobra.Command {
	var force bool
	var profile string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter config file",
		Example: "  githook init --config server.yaml --profile server\n" +
			"  githook init --config cli.yaml --profile cli\n" +
			"  githook init --config worker.yaml --profile worker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := strings.TrimSpace(configPath)
			if path == "" {
				return fmt.Errorf("config path is required")
			}
			template, err := initTemplate(profile)
			if err != nil {
				return err
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("config already exists: %s", path)
				}
			}
			if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
				return err
			}
			return printStdoutf("wrote config to %s\n", path)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	cmd.Flags().StringVar(&profile, "profile", "server", "Config profile (server|cli|worker)")
	return cmd
}

func initTemplate(profile string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "server":
		return initConfigTemplateServer, nil
	case "cli":
		return initConfigTemplateCLI, nil
	case "worker":
		return initConfigTemplateWorker, nil
	default:
		return "", fmt.Errorf("unknown profile: %s", profile)
	}
}
