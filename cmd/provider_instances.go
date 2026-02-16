package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Manage providers",
		Long:  "Manage per-tenant provider instances stored on the server.",
		Example: "  githook --endpoint http://localhost:8080 providers list\n" +
			"  githook --endpoint http://localhost:8080 providers list --provider github\n" +
			"  githook --endpoint http://localhost:8080 providers set --provider github --config-file github.yaml",
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
		Example: "  githook --endpoint http://localhost:8080 providers list\n" +
			"  githook --endpoint http://localhost:8080 providers list --provider github",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewProvidersServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListProvidersRequest{Provider: strings.TrimSpace(provider)})
			applyTenantHeader(req)
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
		Use:     "get",
		Short:   "Get a provider instance",
		Example: "  githook --endpoint http://localhost:8080 providers get --provider github --hash <instance-hash>",
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
			applyTenantHeader(req)
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
	var enabled bool
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Create a provider instance",
		Example: "  githook --endpoint http://localhost:8080 providers set --provider github --config-file github.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(provider) == "" {
				return fmt.Errorf("provider is required")
			}
			var err error
			if strings.TrimSpace(configFile) == "" {
				return fmt.Errorf("config-file is required")
			}
			payload, err := loadConfigPayload(configFile)
			if err != nil {
				return err
			}
			payload, err = injectPrivateKeyPem(payload, "")
			if err != nil {
				return fmt.Errorf("failed to inject private key: %w", err)
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
			applyTenantHeader(req)
			resp, err := client.UpsertProvider(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name (github/gitlab/bitbucket)")
	cmd.Flags().StringVar(&configFile, "config-file", "", "Path to provider YAML/JSON config")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable this provider instance")
	return cmd
}

func injectPrivateKeyPem(payload, keyPath string) (string, error) {
	var target map[string]any
	if err := json.Unmarshal([]byte(payload), &target); err != nil {
		return "", err
	}
	app, _ := target["app"].(map[string]any)
	if app == nil {
		app = make(map[string]any)
	}
	var pem string
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return "", err
		}
		pem = string(data)
	}
	if strings.TrimSpace(pem) == "" {
		if existing, ok := app["private_key_pem"].(string); ok && strings.TrimSpace(existing) != "" {
			pem = existing
		}
	}
	if strings.TrimSpace(pem) == "" {
		if pathVal, ok := app["private_key_path"].(string); ok && strings.TrimSpace(pathVal) != "" {
			data, err := os.ReadFile(pathVal)
			if err != nil {
				return "", err
			}
			pem = string(data)
		}
	}
	if strings.TrimSpace(pem) != "" {
		app["private_key_pem"] = pem
		delete(app, "private_key_path")
	}
	target["app"] = app
	out, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func newProviderInstancesDeleteCmd() *cobra.Command {
	var provider string
	var hash string
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a provider instance",
		Example: "  githook --endpoint http://localhost:8080 providers delete --provider github --hash <instance-hash>",
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
			applyTenantHeader(req)
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
