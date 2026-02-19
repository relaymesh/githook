package oauth

import (
	"log"
	"net/http"
	"strings"

	"githook/pkg/auth"
	"githook/pkg/storage"
)

func (h *Handler) handleSlack(w http.ResponseWriter, r *http.Request, logger *log.Logger, cfg auth.ProviderConfig) {
	stateValue := decodeState(r.URL.Query().Get("state"))
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	storeCtx := storage.WithTenant(r.Context(), stateValue.TenantID)
	cfg, instanceKey, instanceRedirect := h.resolveInstanceConfig(storeCtx, "slack", stateValue.InstanceKey, cfg)

	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		http.Error(w, "oauth client config missing", http.StatusInternalServerError)
		return
	}

	redirectURL := callbackURL(r, "slack", h.Endpoint)
	token, err := exchangeSlackToken(r.Context(), cfg, code, redirectURL)
	if err != nil {
		logger.Printf("slack oauth exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	accountID := strings.TrimSpace(token.Team.ID)
	accountName := strings.TrimSpace(token.Team.Name)
	if accountID == "" && token.Enterprise.ID != "" {
		accountID = strings.TrimSpace(token.Enterprise.ID)
		accountName = strings.TrimSpace(token.Enterprise.Name)
	}
	installationID := strings.TrimSpace(token.AppID)
	if installationID == "" {
		installationID = strings.TrimSpace(token.BotUserID)
	}
	if accountID == "" || installationID == "" {
		http.Error(w, "slack installation details missing", http.StatusBadRequest)
		return
	}

	warning := ""
	if warning == "" && storeAvailable(h.Store) {
		if resolveExistingInstallationID(storeCtx, h.Store, "slack", accountID, instanceKey) != "" {
			warning = "already installed"
		}
	}

	record := storage.InstallRecord{
		TenantID:            stateValue.TenantID,
		Provider:            "slack",
		AccountID:           accountID,
		AccountName:         accountName,
		InstallationID:      installationID,
		ProviderInstanceKey: instanceKey,
		AccessToken:         token.AccessToken,
		RefreshToken:        strings.TrimSpace(token.RefreshToken),
		ExpiresAt:           slackTokenExpiry(token.ExpiresIn),
		MetadataJSON:        token.MetadataJSON(),
	}

	if storeAvailable(h.Store) {
		logUpsertAttempt(logger, record, token.AccessToken)
		if err := h.Store.UpsertInstallation(storeCtx, record); err != nil {
			logger.Printf("slack installation upsert failed: %v", err)
			warning = "install record not saved"
		}
		dedupeInstallations(storeCtx, h.Store, "slack", accountID, instanceKey, record.InstallationID)
	}

	params := map[string]string{
		"provider":     "slack",
		"account_id":   accountID,
		"account_name": accountName,
		"warning":      warning,
	}
	h.redirectOrJSON(w, r, params, instanceRedirect)
}
