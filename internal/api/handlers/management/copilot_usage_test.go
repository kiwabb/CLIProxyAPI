package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	copilotauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/copilot"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestGetCopilotUsageByAuthIndex(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	originalFetch := fetchCopilotUsage
	defer func() { fetchCopilotUsage = originalFetch }()
	fetchCopilotUsage = func(_ context.Context, _ *http.Client, githubToken, vscodeVersion string) (*copilotauth.UsageResponse, error) {
		if githubToken != "github-token" {
			t.Fatalf("githubToken = %q, want github-token", githubToken)
		}
		if vscodeVersion != "1.99.0" {
			t.Fatalf("vscodeVersion = %q, want 1.99.0", vscodeVersion)
		}
		return &copilotauth.UsageResponse{
			CopilotPlan:    "individual",
			QuotaResetDate: "2026-07-01",
			QuotaSnapshots: map[string]copilotauth.UsageQuotaSnap{
				"chat": {Entitlement: 100, Remaining: 90, QuotaRemaining: 90, PercentRemaining: 90},
			},
		}, nil
	}

	manager := coreauth.NewManager(nil, nil, nil)
	auth := &coreauth.Auth{
		ID:       "copilot-auth-id",
		FileName: "copilot-user.json",
		Provider: copilotauth.Provider,
		Label:    "copilot-user",
		Metadata: map[string]any{
			"github_token":   "github-token",
			"vscode_version": "1.98.0",
		},
		Attributes: map[string]string{
			"vscode_version": "1.99.0",
		},
	}
	authIndex := auth.EnsureIndex()
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/copilot-usage?auth_index="+authIndex, nil)

	h.GetCopilotUsage(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload struct {
		Status    string `json:"status"`
		AuthIndex string `json:"auth_index"`
		Usage     struct {
			CopilotPlan    string `json:"copilot_plan"`
			QuotaResetDate string `json:"quota_reset_date"`
			QuotaSnapshots struct {
				Chat struct {
					Remaining int `json:"remaining"`
				} `json:"chat"`
			} `json:"quota_snapshots"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != "ok" || payload.AuthIndex != authIndex {
		t.Fatalf("unexpected response status/index: %+v", payload)
	}
	if payload.Usage.CopilotPlan != "individual" || payload.Usage.QuotaSnapshots.Chat.Remaining != 90 {
		t.Fatalf("unexpected usage: %+v", payload.Usage)
	}
}
