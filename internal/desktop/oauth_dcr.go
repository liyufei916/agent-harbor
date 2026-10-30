package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type oauthClientCreds struct {
	ID     string
	Secret string
	Source string // env | dcr | default
}

// resolveOAuthClient：显式环境变量优先；否则尝试 RFC 7591 DCR；再回退默认 client_id。
func resolveOAuthClient(ctx context.Context, authority, redirectURI string) oauthClientCreds {
	if v := strings.TrimSpace(os.Getenv("HARBOR_MCP_OAUTH_CLIENT_ID")); v != "" {
		return oauthClientCreds{ID: v, Secret: oauthClientSecret(), Source: "env"}
	}
	if c, err := tryDynamicClientRegistration(ctx, authority, redirectURI); err == nil && c.ID != "" {
		return c
	}
	return oauthClientCreds{ID: oauthClientID(), Secret: oauthClientSecret(), Source: "default"}
}

func tryDynamicClientRegistration(ctx context.Context, authority, redirectURI string) (oauthClientCreds, error) {
	metaURL := strings.TrimRight(authority, "/") + "/.well-known/oauth-authorization-server"
	regURL, err := fetchRegistrationEndpoint(ctx, metaURL)
	if err != nil || regURL == "" {
		// 部分 IdP 把元数据挂在 OpenID 发现文档。
		regURL, err = fetchRegistrationEndpoint(ctx, strings.TrimRight(authority, "/")+"/.well-known/openid-configuration")
		if err != nil || regURL == "" {
			return oauthClientCreds{}, fmt.Errorf("no registration_endpoint")
		}
	}
	body := map[string]any{
		"client_name":                "Agent Harbor",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"application_type":           "native",
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, regURL, bytes.NewReader(raw))
	if err != nil {
		return oauthClientCreds{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return oauthClientCreds{}, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oauthClientCreds{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthClientCreds{}, fmt.Errorf("dcr status %d", resp.StatusCode)
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if json.Unmarshal(payload, &out) != nil || strings.TrimSpace(out.ClientID) == "" {
		return oauthClientCreds{}, fmt.Errorf("dcr missing client_id")
	}
	return oauthClientCreds{ID: out.ClientID, Secret: out.ClientSecret, Source: "dcr"}, nil
}

func fetchRegistrationEndpoint(ctx context.Context, metaURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("metadata status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var meta struct {
		RegistrationEndpoint string `json:"registration_endpoint"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return "", fmt.Errorf("bad metadata")
	}
	return strings.TrimSpace(meta.RegistrationEndpoint), nil
}
