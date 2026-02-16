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

watermill:
  # driver: gochannel
  # drivers: []
  # dlq_driver: ""
  # publish_retry:
  #   attempts: 0
  #   delay_ms: 0
  # gochannel:
  #   output_buffer: 64
  #   persistent: false
  #   block_publish_until_subscriber_ack: false
  # kafka:
  #   brokers: []
  # nats:
  #   cluster_id: ""
  #   client_id: ""
  #   url: ""
  # amqp:
  #   url: ""
  #   mode: ""
  # sql:
  #   # SQL subscribers are for event queues only, not platform storage.
  #   driver: ""
  #   dsn: ""
  #   dialect: ""
  #   initialize_schema: false
  #   auto_initialize_schema: false
  # http:
  #   base_url: ""
  #   mode: ""
  # riverqueue:
  #   driver: ""
  #   dsn: ""
  #   table: ""
  #   queue: ""
  #   kind: ""
  #   max_attempts: 0
  #   priority: 0
  #   tags: []
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
			fmt.Printf("wrote config to %s\n", path)
			return nil
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
