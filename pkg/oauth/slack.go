package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"githook/pkg/auth"
)

type slackOAuthResponse struct {
	OK           bool            `json:"ok"`
	Error        string          `json:"error"`
	AccessToken  string          `json:"access_token"`
	TokenType    string          `json:"token_type"`
	Scope        string          `json:"scope"`
	BotUserID    string          `json:"bot_user_id"`
	AppID        string          `json:"app_id"`
	Team         slackTeam       `json:"team"`
	Enterprise   slackEnterprise `json:"enterprise"`
	AuthedUser   slackAuthedUser `json:"authed_user"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    int64           `json:"expires_in"`
}

type slackTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type slackEnterprise struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type slackAuthedUser struct {
	ID           string `json:"id"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func slackAuthorizeURL(cfg auth.ProviderConfig, state, redirectURL string) (string, error) {
	if cfg.OAuth.ClientID == "" {
		return "", fmt.Errorf("slack oauth_client_id is required")
	}
	baseURL := strings.TrimRight(cfg.API.WebBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://slack.com"
	}
	u, err := url.Parse(baseURL + "/oauth/v2/authorize")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.OAuth.ClientID)
	if redirectURL != "" {
		q.Set("redirect_uri", redirectURL)
	}
	if len(cfg.OAuth.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.OAuth.Scopes, ","))
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func exchangeSlackToken(ctx context.Context, cfg auth.ProviderConfig, code, redirectURL string) (slackOAuthResponse, error) {
	if strings.TrimSpace(code) == "" {
		return slackOAuthResponse{}, errors.New("slack oauth code missing")
	}
	values := url.Values{}
	values.Set("client_id", cfg.OAuth.ClientID)
	values.Set("client_secret", cfg.OAuth.ClientSecret)
	values.Set("code", code)
	if redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/oauth.v2.access", strings.NewReader(values.Encode()))
	if err != nil {
		return slackOAuthResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return slackOAuthResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return slackOAuthResponse{}, fmt.Errorf("slack token exchange failed: %s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var token slackOAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return slackOAuthResponse{}, err
	}
	if !token.OK {
		if token.Error == "" {
			token.Error = "unknown_error"
		}
		return slackOAuthResponse{}, fmt.Errorf("slack token exchange failed: %s", token.Error)
	}
	if token.AccessToken == "" {
		return slackOAuthResponse{}, errors.New("slack access token missing")
	}
	return token, nil
}

func slackTokenExpiry(expiresIn int64) *time.Time {
	if expiresIn <= 0 {
		return nil
	}
	expires := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	return &expires
}

func (t slackOAuthResponse) MetadataJSON() string {
	payload := map[string]interface{}{
		"token_type":        t.TokenType,
		"scope":             t.Scope,
		"bot_user_id":       t.BotUserID,
		"app_id":            t.AppID,
		"team_id":           t.Team.ID,
		"team_name":         t.Team.Name,
		"enterprise_id":     t.Enterprise.ID,
		"enterprise_name":   t.Enterprise.Name,
		"authed_user_id":    t.AuthedUser.ID,
		"authed_user_scope": t.AuthedUser.Scope,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}
