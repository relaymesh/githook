package worker

import (
	"context"
	"errors"
)

// ResolveInstallation fetches the installation record for the event's installation_id.
func ResolveInstallation(ctx context.Context, evt *Event, client *InstallationsClient) (*InstallationRecord, error) {
	if evt == nil {
		return nil, errors.New("event is required")
	}
	if client == nil {
		return nil, errors.New("installations client is required")
	}
	installationID := ""
	if evt.Metadata != nil {
		installationID = evt.Metadata[MetadataKeyInstallationID]
	}
	if installationID == "" {
		return nil, errors.New("installation_id missing from metadata")
	}
	return client.GetByInstallationID(ctx, evt.Provider, installationID)
}
