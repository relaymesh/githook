package oauth

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
)

func callbackURL(r *http.Request, provider, publicBaseURL string) string {
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL != "" {
		return fmt.Sprintf("%s/oauth/%s/callback", publicBaseURL, provider)
	}
	scheme := forwardedProto(r)
	host := forwardedHost(r)
	if scheme == "" {
		scheme = "http"
	}
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s/oauth/%s/callback", scheme, host, provider)
}

func forwardedProto(r *http.Request) string {
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return ""
}

func forwardedHost(r *http.Request) string {
	if host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); host != "" {
		return host
	}
	return ""
}

func providerFromPath(path string) string {
	path = strings.TrimRight(path, "/")
	switch {
	case strings.HasSuffix(path, "/oauth/github/callback"):
		return "github"
	case strings.HasSuffix(path, "/oauth/gitlab/callback"):
		return "gitlab"
	case strings.HasSuffix(path, "/oauth/bitbucket/callback"):
		return "bitbucket"
	default:
		return ""
	}
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", buf)
}
