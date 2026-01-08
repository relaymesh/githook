package githooks

import "github.com/spf13/cobra"

// NewRootCmd returns the Cobra entrypoint for the CLI/server.
func NewRootCmd() *cobra.Command {
	apiBaseURL = ""
	configPath = "config.yaml"
	root := &cobra.Command{
		Use:   "githooks",
		Short: "Webhook router + worker SDK for Git providers",
		Long: "Githooks routes GitHub/GitLab/Bitbucket webhooks to Watermill topics and provides a worker SDK " +
			"for processing events with provider-aware clients.",
		Example: "  githooks serve --config config.yaml\n" +
			"  githooks --endpoint http://localhost:8080 installations list --state-id <state-id>\n" +
			"  githooks --endpoint http://localhost:8080 namespaces sync --state-id <state-id> --provider gitlab",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringVar(&apiBaseURL, "endpoint", apiBaseURL, "Connect RPC endpoint base URL")
	root.PersistentFlags().StringVar(&configPath, "config", configPath, "Path to config file")
	root.AddCommand(newServeCmd())
	root.AddCommand(newInstallationsCmd())
	root.AddCommand(newNamespacesCmd())
	root.AddCommand(newRulesCmd())
	root.AddCommand(newProvidersCmd())
	root.AddCommand(newDriversCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newInitCmd())
	return root
}

var apiBaseURL string
var configPath string
