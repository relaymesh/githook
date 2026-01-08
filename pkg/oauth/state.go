package oauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type oauthState struct {
	State       string `json:"state"`
	TenantID    string `json:"tenant_id,omitempty"`
	InstanceKey string `json:"instance_key,omitempty"`
}

func encodeState(state, tenantID, instanceKey string) string {
	tenantID = strings.TrimSpace(tenantID)
	instanceKey = strings.TrimSpace(instanceKey)
	if tenantID == "" && instanceKey == "" {
		return state
	}
	payload := oauthState{
		State:       state,
		TenantID:    tenantID,
		InstanceKey: instanceKey,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return state
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeState(value string) oauthState {
	value = strings.TrimSpace(value)
	if value == "" {
		return oauthState{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return oauthState{State: value}
	}
	var payload oauthState
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthState{State: value}
	}
	if payload.State == "" {
		payload.State = value
	}
	return payload
}
