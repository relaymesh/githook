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

// DriverRecord mirrors the server driver response.
type DriverRecord struct {
	Name       string `json:"name"`
	ConfigJSON string `json:"config_json"`
	Enabled    bool   `json:"enabled"`
}

// DriversClient fetches driver records from the server API.
type DriversClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// GetDriver fetches the driver record by name.
func (c *DriversClient) GetDriver(ctx context.Context, name string) (*DriverRecord, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil, errors.New("driver name is required")
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, errors.New("base url is required")
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	interceptor := validate.NewInterceptor()
	connectClient := cloudv1connect.NewDriversServiceClient(
		client,
		base,
		connect.WithInterceptors(interceptor),
	)
	req := connect.NewRequest(&cloudv1.GetDriverRequest{
		Name: name,
	})
	if token, err := oauth2Token(ctx); err == nil && token != "" {
		req.Header().Set("Authorization", "Bearer "+token)
	}
	if tenantID := TenantIDFromContext(ctx); tenantID != "" {
		req.Header().Set("X-Tenant-ID", tenantID)
	}
	resp, err := connectClient.GetDriver(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("drivers api failed: %w", err)
	}
	if resp.Msg.GetDriver() == nil {
		return nil, nil
	}
	record := resp.Msg.GetDriver()
	return &DriverRecord{
		Name:       record.GetName(),
		ConfigJSON: record.GetConfigJson(),
		Enabled:    record.GetEnabled(),
	}, nil
}
