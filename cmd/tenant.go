package cmd

import (
	"strings"

	"connectrpc.com/connect"
)

var tenantID = "default"

func applyTenantHeader[T any](req *connect.Request[T]) {
	tenant := strings.TrimSpace(tenantID)
	if tenant == "" {
		tenant = "default"
	}
	req.Header().Set("X-Tenant-ID", tenant)
}
