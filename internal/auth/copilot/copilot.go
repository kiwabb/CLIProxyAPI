package copilot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	Provider = "copilot"

	GitHubBaseURL    = "https://github.com"
	GitHubAPIBaseURL = "https://api.github.com"
	GitHubClientID   = "Iv1.b507a08c87ecfe98"
	GitHubScopes     = "read:user"

	DefaultCopilotBaseURL = "https://api.githubcopilot.com"
	CopilotVersion        = "0.26.7"
	APIUserAgent          = "GitHubCopilotChat/" + CopilotVersion
	EditorPluginVersion   = "copilot-chat/" + CopilotVersion
	APIVersion            = "2025-04-01"
	DefaultVSCodeVersion  = "1.126.0"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type UserResponse struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type TokenResponse struct {
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int64  `json:"refresh_in"`
	Token     string `json:"token"`
}

type ModelsResponse struct {
	Data   []Model `json:"data"`
	Object string  `json:"object"`
}

type UsageResponse struct {
	AccessTypeSKU         string                    `json:"access_type_sku"`
	AnalyticsTrackingID   string                    `json:"analytics_tracking_id"`
	AssignedDate          string                    `json:"assigned_date"`
	CanSignupForLimited   bool                      `json:"can_signup_for_limited"`
	ChatEnabled           bool                      `json:"chat_enabled"`
	CopilotPlan           string                    `json:"copilot_plan"`
	OrganizationLoginList []any                     `json:"organization_login_list"`
	OrganizationList      []any                     `json:"organization_list"`
	QuotaResetDate        string                    `json:"quota_reset_date"`
	QuotaSnapshots        map[string]UsageQuotaSnap `json:"quota_snapshots"`
}

type UsageQuotaSnap struct {
	Entitlement      float64 `json:"entitlement"`
	OverageCount     float64 `json:"overage_count"`
	OveragePermitted bool    `json:"overage_permitted"`
	PercentRemaining float64 `json:"percent_remaining"`
	QuotaID          string  `json:"quota_id"`
	QuotaRemaining   float64 `json:"quota_remaining"`
	Remaining        float64 `json:"remaining"`
	Unlimited        bool    `json:"unlimited"`
}

type Model struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Object       string            `json:"object"`
	Preview      bool              `json:"preview"`
	Vendor       string            `json:"vendor"`
	Version      string            `json:"version"`
	Capabilities ModelCapabilities `json:"capabilities"`
}

type ModelCapabilities struct {
	Type   string      `json:"type"`
	Family string      `json:"family"`
	Limits ModelLimits `json:"limits"`
}

type ModelLimits struct {
	MaxContextWindowTokens int `json:"max_context_window_tokens"`
	MaxOutputTokens        int `json:"max_output_tokens"`
	MaxPromptTokens        int `json:"max_prompt_tokens"`
}

func RequestDeviceCode(ctx context.Context, client *http.Client) (*DeviceCodeResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	payload := map[string]string{
		"client_id": GitHubClientID,
		"scope":     GitHubScopes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GitHubBaseURL+"/login/device/code", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setJSONHeaders(req.Header)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github device code request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out DeviceCodeResponse
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.DeviceCode == "" || out.UserCode == "" || out.VerificationURI == "" {
		return nil, fmt.Errorf("github device code response missing required fields")
	}
	return &out, nil
}

func PollAccessToken(ctx context.Context, client *http.Client, deviceCode *DeviceCodeResponse) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if deviceCode == nil || strings.TrimSpace(deviceCode.DeviceCode) == "" {
		return "", fmt.Errorf("github device code is required")
	}
	interval := time.Duration(deviceCode.Interval+1) * time.Second
	if interval <= 0 {
		interval = 6 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)
	if deviceCode.ExpiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("github device authentication timed out")
		}
		payload := map[string]string{
			"client_id":   GitHubClientID,
			"device_code": deviceCode.DeviceCode,
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, GitHubBaseURL+"/login/oauth/access_token", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		setJSONHeaders(req.Header)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		var parsed AccessTokenResponse
		_ = json.Unmarshal(data, &parsed)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.TrimSpace(parsed.AccessToken) != "" {
			return strings.TrimSpace(parsed.AccessToken), nil
		}
		if parsed.Error != "" && parsed.Error != "authorization_pending" && parsed.Error != "slow_down" {
			if parsed.Description != "" {
				return "", fmt.Errorf("github device authentication failed: %s: %s", parsed.Error, parsed.Description)
			}
			return "", fmt.Errorf("github device authentication failed: %s", parsed.Error)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
}

func FetchUser(ctx context.Context, client *http.Client, githubToken string) (*UserResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIBaseURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	setGitHubHeaders(req.Header, githubToken, DefaultVSCodeVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github user request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out UserResponse
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func FetchCopilotToken(ctx context.Context, client *http.Client, githubToken, vscodeVersion string) (*TokenResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIBaseURL+"/copilot_internal/v2/token", nil)
	if err != nil {
		return nil, err
	}
	setGitHubHeaders(req.Header, githubToken, vscodeVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("copilot token request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out TokenResponse
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Token) == "" {
		return nil, fmt.Errorf("copilot token response missing token")
	}
	return &out, nil
}

func FetchModels(ctx context.Context, client *http.Client, baseURL, token, vscodeVersion string) (*ModelsResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultCopilotBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	SetCopilotHeaders(req.Header, token, vscodeVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("copilot models request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out ModelsResponse
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func FetchUsage(ctx context.Context, client *http.Client, githubToken, vscodeVersion string) (*UsageResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubAPIBaseURL+"/copilot_internal/user", nil)
	if err != nil {
		return nil, err
	}
	setGitHubHeaders(req.Header, githubToken, vscodeVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("copilot usage request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out UsageResponse
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func SetCopilotHeaders(header http.Header, token, vscodeVersion string) {
	if header == nil {
		return
	}
	if strings.TrimSpace(vscodeVersion) == "" {
		vscodeVersion = DefaultVSCodeVersion
	}
	header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	header.Set("Content-Type", "application/json")
	header.Set("Copilot-Integration-Id", "vscode-chat")
	header.Set("Editor-Version", "vscode/"+vscodeVersion)
	header.Set("Editor-Plugin-Version", EditorPluginVersion)
	header.Set("User-Agent", APIUserAgent)
	header.Set("OpenAI-Intent", "conversation-panel")
	header.Set("X-GitHub-Api-Version", APIVersion)
	header.Set("X-Request-Id", uuid.NewString())
	header.Set("X-VSCode-User-Agent-Library-Version", "electron-fetch")
}

func AccountHash(input string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(input)))
	return hex.EncodeToString(sum[:])[:8]
}

func setJSONHeaders(header http.Header) {
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
}

func setGitHubHeaders(header http.Header, githubToken, vscodeVersion string) {
	setJSONHeaders(header)
	if strings.TrimSpace(vscodeVersion) == "" {
		vscodeVersion = DefaultVSCodeVersion
	}
	header.Set("Authorization", "token "+strings.TrimSpace(githubToken))
	header.Set("Editor-Version", "vscode/"+vscodeVersion)
	header.Set("Editor-Plugin-Version", EditorPluginVersion)
	header.Set("User-Agent", APIUserAgent)
	header.Set("X-GitHub-Api-Version", APIVersion)
	header.Set("X-VSCode-User-Agent-Library-Version", "electron-fetch")
}
