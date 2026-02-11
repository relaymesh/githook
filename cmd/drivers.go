package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

func newDriversCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drivers",
		Short: "Manage driver configs",
		Long:  "Manage per-tenant Watermill driver configs stored on the server.",
		Example: "  githook --endpoint http://localhost:8080 drivers list\n" +
			"  githook --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.json",
	}
	cmd.AddCommand(newDriversListCmd())
	cmd.AddCommand(newDriversGetCmd())
	cmd.AddCommand(newDriversSetCmd())
	cmd.AddCommand(newDriversDeleteCmd())
	return cmd
}

func newDriversListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List driver configs",
		Example: "  githook --endpoint http://localhost:8080 drivers list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewDriversServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListDriversRequest{})
			resp, err := client.ListDrivers(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func newDriversGetCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a driver config",
		Example: "  githook --endpoint http://localhost:8080 drivers get --name amqp",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("name is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewDriversServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.GetDriverRequest{Name: strings.TrimSpace(name)})
			resp, err := client.GetDriver(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Driver name (amqp/nats/kafka/sql/http/...)")
	return cmd
}

func newDriversSetCmd() *cobra.Command {
	var name string
	var configFile string
	var configJSON string
	var enabled bool
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Create or update a driver config",
		Example: "  githook --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("name is required")
			}
			payload := strings.TrimSpace(configJSON)
			if configFile != "" {
				data, err := os.ReadFile(configFile)
				if err != nil {
					return err
				}
				payload = strings.TrimSpace(string(data))
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewDriversServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.UpsertDriverRequest{
				Driver: &cloudv1.DriverRecord{
					Name:       strings.TrimSpace(name),
					ConfigJson: payload,
					Enabled:    enabled,
				},
			})
			resp, err := client.UpsertDriver(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Driver name (amqp/nats/kafka/sql/http/...)")
	cmd.Flags().StringVar(&configFile, "config-file", "", "Path to driver JSON config")
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Inline driver JSON config")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable this driver")
	return cmd
}

func newDriversDeleteCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a driver config",
		Example: "  githook --endpoint http://localhost:8080 drivers delete --name amqp",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("name is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewDriversServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.DeleteDriverRequest{Name: strings.TrimSpace(name)})
			resp, err := client.DeleteDriver(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Driver name (amqp/nats/kafka/sql/http/...)")
	return cmd
}
