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

	"githook/pkg/auth"
	cloudv1 "githook/pkg/gen/cloud/v1"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
)

// RulesClient fetches rule records from the server API.
type RulesClient struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
	OAuth2     *auth.OAuth2Config
}

// RuleRecord mirrors the server rule response.
type RuleRecord struct {
	ID       string   `json:"id"`
	When     string   `json:"when"`
	Emit     []string `json:"emit"`
	DriverID string   `json:"driver_id"`
}

// ListRules returns all rules.
func (c *RulesClient) ListRules(ctx context.Context) ([]RuleRecord, error) {
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
	setAuthHeaders(ctx, req.Header(), c.APIKey, c.OAuth2)
	if tenantID := TenantIDFromContext(ctx); tenantID != "" {
		req.Header().Set("X-Tenant-ID", tenantID)
	}
	resp, err := connectClient.ListRules(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rules api failed: %w", err)
	}
	out := make([]RuleRecord, 0, len(resp.Msg.GetRules()))
	for _, record := range resp.Msg.GetRules() {
		if record == nil {
			continue
		}
		out = append(out, RuleRecord{
			ID:       record.GetId(),
			When:     record.GetWhen(),
			Emit:     record.GetEmit(),
			DriverID: record.GetDriverId(),
		})
	}
	return out, nil
}

// ListRuleTopics returns the unique emit topics from all rules.
func (c *RulesClient) ListRuleTopics(ctx context.Context) ([]string, error) {
	rules, err := c.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	topics := map[string]struct{}{}
	for _, record := range rules {
		for _, topic := range record.Emit {
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
