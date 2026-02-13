package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

// RulesClient fetches rule records from the server API.
type RulesClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// ListRuleTopics returns the unique emit topics from all rules.
func (c *RulesClient) ListRuleTopics(ctx context.Context) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, errors.New("base url is required")
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	interceptor := validate.NewInterceptor()
	connectClient := cloudv1connect.NewRulesServiceClient(
		client,
		base,
		connect.WithInterceptors(interceptor),
	)
	req := connect.NewRequest(&cloudv1.ListRulesRequest{})
	if token, err := oauth2Token(ctx); err == nil && token != "" {
		req.Header().Set("Authorization", "Bearer "+token)
	}
	if tenantID := TenantIDFromContext(ctx); tenantID != "" {
		req.Header().Set("X-Tenant-ID", tenantID)
	}
	resp, err := connectClient.ListRules(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rules api failed: %w", err)
	}

	topics := map[string]struct{}{}
	for _, record := range resp.Msg.GetRules() {
		for _, topic := range record.GetEmit() {
			trimmed := strings.TrimSpace(topic)
			if trimmed == "" {
				continue
			}
			topics[trimmed] = struct{}{}
		}
	}
	out := make([]string, 0, len(topics))
	for topic := range topics {
		out = append(out, topic)
	}
	return out, nil
}
