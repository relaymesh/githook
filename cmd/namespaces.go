package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

func newNamespacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespaces",
		Short: "Manage git namespaces",
		Long:  "List, sync, and toggle webhooks for provider namespaces (repos/projects).",
		Example: "  githook --endpoint http://localhost:8080 namespaces list --provider github\n" +
			"  githook --endpoint http://localhost:8080 namespaces sync --provider gitlab",
	}
	cmd.AddCommand(newNamespacesListCmd())
	cmd.AddCommand(newNamespacesSyncCmd())
	cmd.AddCommand(newNamespacesWebhookCmd())
	return cmd
}

func newNamespacesListCmd() *cobra.Command {
	var stateID string
	var provider string
	var owner string
	var repo string
	var fullName string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List namespaces",
		Example: "  githook --endpoint http://localhost:8080 namespaces list --provider github",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewNamespacesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListNamespacesRequest{
				StateId:  stateID,
				Provider: strings.TrimSpace(provider),
				Owner:    strings.TrimSpace(owner),
				Repo:     strings.TrimSpace(repo),
				FullName: strings.TrimSpace(fullName),
			})
			resp, err := client.ListNamespaces(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&stateID, "state-id", "", "State ID filter (optional)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	cmd.Flags().StringVar(&owner, "owner", "", "Owner filter")
	cmd.Flags().StringVar(&repo, "repo", "", "Repo filter")
	cmd.Flags().StringVar(&fullName, "full-name", "", "Full name filter")
	return cmd
}

func newNamespacesSyncCmd() *cobra.Command {
	var stateID string
	var provider string
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Sync namespaces from the provider",
		Example: "  githook --endpoint http://localhost:8080 namespaces sync --provider gitlab",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider == "" {
				return fmt.Errorf("provider is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewNamespacesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.SyncNamespacesRequest{
				StateId:  stateID,
				Provider: provider,
			})
			resp, err := client.SyncNamespaces(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&stateID, "state-id", "", "State ID filter (optional)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	return cmd
}

func newNamespacesWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhook",
		Short:   "Manage namespace webhook state",
		Example: "  githook --endpoint http://localhost:8080 namespaces webhook get --provider gitlab --repo-id <repo-id>",
	}
	cmd.AddCommand(newNamespacesWebhookGetCmd())
	cmd.AddCommand(newNamespacesWebhookSetCmd())
	return cmd
}

func newNamespacesWebhookGetCmd() *cobra.Command {
	var stateID string
	var provider string
	var repoID string
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get namespace webhook state",
		Example: "  githook --endpoint http://localhost:8080 namespaces webhook get --provider gitlab --repo-id <repo-id>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider == "" || repoID == "" {
				return fmt.Errorf("provider and repo-id are required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewNamespacesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.GetNamespaceWebhookRequest{
				StateId:  stateID,
				Provider: provider,
				RepoId:   repoID,
			})
			resp, err := client.GetNamespaceWebhook(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&stateID, "state-id", "", "State ID filter (optional)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repo ID")
	return cmd
}

func newNamespacesWebhookSetCmd() *cobra.Command {
	var stateID string
	var provider string
	var repoID string
	var enabled bool
	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Enable or disable a namespace webhook",
		Example: "  githook --endpoint http://localhost:8080 namespaces webhook set --provider gitlab --repo-id <repo-id> --enabled",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider == "" || repoID == "" {
				return fmt.Errorf("provider and repo-id are required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewNamespacesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.SetNamespaceWebhookRequest{
				StateId:  stateID,
				Provider: provider,
				RepoId:   repoID,
				Enabled:  enabled,
			})
			resp, err := client.SetNamespaceWebhook(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&stateID, "state-id", "", "State ID filter (optional)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repo ID")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Enable or disable webhook")
	return cmd
}
