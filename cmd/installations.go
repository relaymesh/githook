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

func newInstallationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "installations",
		Short: "Inspect stored installations",
		Long:  "Query installation records resolved from provider OAuth/App flows.",
		Example: "  githook --endpoint http://localhost:8080 installations list --state-id <state-id>\n" +
			"  githook --endpoint http://localhost:8080 installations get --provider github --installation-id <id>",
	}
	cmd.AddCommand(newInstallationsListCmd())
	cmd.AddCommand(newInstallationsGetCmd())
	return cmd
}

func newInstallationsListCmd() *cobra.Command {
	var stateID string
	var provider string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List installations by state ID",
		Example: "  githook --endpoint http://localhost:8080 installations list --state-id <state-id>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if stateID == "" {
				return fmt.Errorf("state-id is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewInstallationsServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListInstallationsRequest{
				StateId:  stateID,
				Provider: strings.TrimSpace(provider),
			})
			resp, err := client.ListInstallations(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&stateID, "state-id", "", "State ID to query")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	return cmd
}

func newInstallationsGetCmd() *cobra.Command {
	var provider string
	var installationID string
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get installation by provider and installation ID",
		Example: "  githook --endpoint http://localhost:8080 installations get --provider github --installation-id <id>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider == "" || installationID == "" {
				return fmt.Errorf("provider and installation-id are required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewInstallationsServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.GetInstallationByIDRequest{
				Provider:       provider,
				InstallationId: installationID,
			})
			resp, err := client.GetInstallationByID(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider (github|gitlab|bitbucket)")
	cmd.Flags().StringVar(&installationID, "installation-id", "", "Installation ID")
	return cmd
}
