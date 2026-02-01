package githooks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	cloudv1 "githooks/pkg/gen/cloud/v1"
	cloudv1connect "githooks/pkg/gen/cloud/v1/cloudv1connect"
)

func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Manage providers",
		Long:  "Manage per-tenant provider instances stored on the server.",
		Example: "  githooks --endpoint http://localhost:8080 providers list\n" +
			"  githooks --endpoint http://localhost:8080 providers list --provider github\n" +
			"  githooks --endpoint http://localhost:8080 providers set --provider github --config-file github.json",
	}
	cmd.AddCommand(newProviderInstancesListCmd())
	cmd.AddCommand(newProviderInstancesGetCmd())
	cmd.AddCommand(newProviderInstancesSetCmd())
	cmd.AddCommand(newProviderInstancesDeleteCmd())
	return cmd
}

func newProviderInstancesListCmd() *cobra.Command {
	var provider string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List provider instances",
		Example: "  githooks --endpoint http://localhost:8080 providers list\n" +
			"  githooks --endpoint http://localhost:8080 providers list --provider github",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewProvidersServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListProvidersRequest{Provider: strings.TrimSpace(provider)})
			resp, err := client.ListProviders(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (github/gitlab/bitbucket)")
	return cmd
}

func newProviderInstancesGetCmd() *cobra.Command {
	var provider string
	var hash string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a provider instance",
		Example: "  githooks --endpoint http://localhost:8080 providers get --provider github --hash <instance-hash>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(provider) == "" {
				return fmt.Errorf("provider is required")
			}
			if strings.TrimSpace(hash) == "" {
				return fmt.Errorf("hash is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewProvidersServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.GetProviderRequest{
				Provider: strings.TrimSpace(provider),
				Hash:     strings.TrimSpace(hash),
			})
			resp, err := client.GetProvider(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (github/gitlab/bitbucket)")
	cmd.Flags().StringVar(&hash, "hash", "", "Instance hash")
	return cmd
}

func newProviderInstancesSetCmd() *cobra.Command {
	var provider string
	var configFile string
	var configJSON string
	var enabled bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create a provider instance",
		Example: "  githooks --endpoint http://localhost:8080 providers set --provider github --config-file github.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(provider) == "" {
				return fmt.Errorf("provider is required")
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
			client := cloudv1connect.NewProvidersServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.UpsertProviderRequest{
				Provider: &cloudv1.ProviderRecord{
					Provider:   strings.TrimSpace(provider),
					ConfigJson: payload,
					Enabled:    enabled,
				},
			})
			resp, err := client.UpsertProvider(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (github/gitlab/bitbucket)")
	cmd.Flags().StringVar(&configFile, "config-file", "", "Path to provider JSON config")
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Inline provider JSON config")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable this provider instance")
	return cmd
}

func newProviderInstancesDeleteCmd() *cobra.Command {
	var provider string
	var hash string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a provider instance",
		Example: "  githooks --endpoint http://localhost:8080 providers delete --provider github --hash <instance-hash>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(provider) == "" {
				return fmt.Errorf("provider is required")
			}
			if strings.TrimSpace(hash) == "" {
				return fmt.Errorf("hash is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewProvidersServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.DeleteProviderRequest{
				Provider: strings.TrimSpace(provider),
				Hash:     strings.TrimSpace(hash),
			})
			resp, err := client.DeleteProvider(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (github/gitlab/bitbucket)")
	cmd.Flags().StringVar(&hash, "hash", "", "Instance hash")
	return cmd
}
