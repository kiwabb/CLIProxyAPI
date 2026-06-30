package cliproxy

import (
	"testing"

	copilotauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/copilot"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestCopilotModelsToModelInfo(t *testing.T) {
	models := copilotModelsToModelInfo([]copilotauth.Model{
		{
			ID:      "gpt-4.1",
			Name:    "GPT-4.1",
			Vendor:  "openai",
			Version: "gpt-4.1-2025-04-14",
			Capabilities: copilotauth.ModelCapabilities{
				Type: "chat",
				Limits: copilotauth.ModelLimits{
					MaxContextWindowTokens: 128000,
					MaxOutputTokens:        16000,
				},
			},
		},
		{ID: "   "},
	})

	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	got := models[0]
	if got.ID != "gpt-4.1" {
		t.Fatalf("ID = %q, want gpt-4.1", got.ID)
	}
	if got.ContextLength != 128000 {
		t.Fatalf("ContextLength = %d, want 128000", got.ContextLength)
	}
	if got.MaxCompletionTokens != 16000 {
		t.Fatalf("MaxCompletionTokens = %d, want 16000", got.MaxCompletionTokens)
	}
}

func TestDefaultCopilotModelsAvailable(t *testing.T) {
	models := defaultCopilotModels()
	if len(models) == 0 {
		t.Fatal("default Copilot models should not be empty")
	}
	for _, model := range models {
		if model.ID == "" {
			t.Fatal("default Copilot model has empty ID")
		}
	}
}

func TestCopilotAuthConfigReadsMetadataFallback(t *testing.T) {
	auth := &coreauth.Auth{
		Metadata: map[string]any{
			"base_url":       " https://copilot.example.test/ ",
			"account_type":   "enterprise",
			"vscode_version": "1.99.0",
		},
	}

	if got := copilotBaseURL(auth); got != "https://copilot.example.test" {
		t.Fatalf("copilotBaseURL() = %q, want metadata base_url", got)
	}
	if got := copilotVSCodeVersion(auth); got != "1.99.0" {
		t.Fatalf("copilotVSCodeVersion() = %q, want metadata vscode_version", got)
	}

	auth.Metadata["base_url"] = ""
	if got := copilotBaseURL(auth); got != "https://api.enterprise.githubcopilot.com" {
		t.Fatalf("copilotBaseURL() = %q, want enterprise base URL", got)
	}
}

func TestCopilotAuthConfigPrefersAttributes(t *testing.T) {
	auth := &coreauth.Auth{
		Attributes: map[string]string{
			"base_url":       "https://attr.example.test/",
			"vscode_version": "1.100.0",
		},
		Metadata: map[string]any{
			"base_url":       "https://metadata.example.test",
			"vscode_version": "1.99.0",
		},
	}

	if got := copilotBaseURL(auth); got != "https://attr.example.test" {
		t.Fatalf("copilotBaseURL() = %q, want attributes base_url", got)
	}
	if got := copilotVSCodeVersion(auth); got != "1.100.0" {
		t.Fatalf("copilotVSCodeVersion() = %q, want attributes vscode_version", got)
	}
}
