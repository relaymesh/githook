package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"githook/pkg/core"
	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Rule engine helpers",
		Long:  "Manage rules stored in the server and test rule expressions against webhook payloads.",
		Example: "  githook --endpoint http://localhost:8080 rules list\n" +
			"  githook --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml",
	}
	cmd.AddCommand(newRulesMatchCmd())
	cmd.AddCommand(newRulesListCmd())
	cmd.AddCommand(newRulesGetCmd())
	cmd.AddCommand(newRulesCreateCmd())
	cmd.AddCommand(newRulesUpdateCmd())
	cmd.AddCommand(newRulesDeleteCmd())
	return cmd
}

func newRulesMatchCmd() *cobra.Command {
	var provider string
	var eventName string
	var payloadFile string
	var rulesFile string
	var strict bool
	cmd := &cobra.Command{
		Use:     "match",
		Short:   "Match rules against an event payload",
		Example: "  githook --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if payloadFile == "" || rulesFile == "" {
				return fmt.Errorf("payload-file and rules-file are required")
			}
			payload, err := os.ReadFile(payloadFile)
			if err != nil {
				return err
			}
			rulesCfg, err := core.LoadRulesConfig(rulesFile)
			if err != nil {
				return err
			}
			ruleMessages := make([]*cloudv1.Rule, 0, len(rulesCfg.Rules))
			for _, rule := range rulesCfg.Rules {
				ruleMessages = append(ruleMessages, &cloudv1.Rule{
					When:    rule.When,
					Emit:    rule.Emit.Values(),
					Drivers: append([]string(nil), rule.Drivers...),
				})
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.MatchRulesRequest{
				Event: &cloudv1.EventPayload{
					Provider: strings.TrimSpace(provider),
					Name:     strings.TrimSpace(eventName),
					Payload:  payload,
				},
				Rules:  ruleMessages,
				Strict: strict,
			})
			resp, err := client.MatchRules(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&eventName, "event", "", "Event name")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "", "Path to JSON payload")
	cmd.Flags().StringVar(&rulesFile, "rules-file", "", "Path to rules YAML")
	cmd.Flags().BoolVar(&strict, "strict", false, "Enable strict mode")
	return cmd
}

func newRulesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List stored rules",
		Example: "  githook --endpoint http://localhost:8080 rules list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.ListRulesRequest{})
			resp, err := client.ListRules(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	return cmd
}

func newRulesGetCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Get a rule by ID",
		Example: "  githook --endpoint http://localhost:8080 rules get --id <rule-id>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("id is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.GetRuleRequest{Id: id})
			resp, err := client.GetRule(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Rule ID")
	return cmd
}

func newRulesCreateCmd() *cobra.Command {
	var when string
	var emits []string
	var drivers []string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new rule",
		Example: "  githook --endpoint http://localhost:8080 rules create --when 'action == \"opened\"' --emit pr.opened.ready",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if when == "" || len(emits) == 0 {
				return fmt.Errorf("when and at least one emit are required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.CreateRuleRequest{
				Rule: &cloudv1.Rule{
					When:    strings.TrimSpace(when),
					Emit:    emits,
					Drivers: drivers,
				},
			})
			resp, err := client.CreateRule(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&when, "when", "", "Rule expression")
	cmd.Flags().StringSliceVar(&emits, "emit", nil, "Emit topic (repeatable)")
	cmd.Flags().StringSliceVar(&drivers, "driver", nil, "Driver override (repeatable)")
	return cmd
}

func newRulesUpdateCmd() *cobra.Command {
	var id string
	var when string
	var emits []string
	var drivers []string
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update an existing rule",
		Example: "  githook --endpoint http://localhost:8080 rules update --id <rule-id> --when 'action == \"closed\"' --emit pr.merged",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" || when == "" || len(emits) == 0 {
				return fmt.Errorf("id, when, and at least one emit are required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.UpdateRuleRequest{
				Id: id,
				Rule: &cloudv1.Rule{
					When:    strings.TrimSpace(when),
					Emit:    emits,
					Drivers: drivers,
				},
			})
			resp, err := client.UpdateRule(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Rule ID")
	cmd.Flags().StringVar(&when, "when", "", "Rule expression")
	cmd.Flags().StringSliceVar(&emits, "emit", nil, "Emit topic (repeatable)")
	cmd.Flags().StringSliceVar(&drivers, "driver", nil, "Driver override (repeatable)")
	return cmd
}

func newRulesDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a rule",
		Example: "  githook --endpoint http://localhost:8080 rules delete --id <rule-id>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("id is required")
			}
			opts, err := connectClientOptions()
			if err != nil {
				return err
			}
			client := cloudv1connect.NewRulesServiceClient(http.DefaultClient, apiBaseURL, opts...)
			req := connect.NewRequest(&cloudv1.DeleteRuleRequest{Id: id})
			resp, err := client.DeleteRule(context.Background(), req)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Rule ID")
	return cmd
}
