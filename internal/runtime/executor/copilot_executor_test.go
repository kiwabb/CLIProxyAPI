package executor

import (
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestCopilotExecutorConfigReadsMetadataFallback(t *testing.T) {
	executor := NewCopilotExecutor(nil)
	auth := &cliproxyauth.Auth{
		Metadata: map[string]any{
			"account_type":   "business",
			"vscode_version": "1.99.0",
		},
	}

	if got := executor.baseURL(auth); got != "https://api.business.githubcopilot.com" {
		t.Fatalf("baseURL() = %q, want business base URL", got)
	}
	if got := executor.vscodeVersion(auth); got != "1.99.0" {
		t.Fatalf("vscodeVersion() = %q, want metadata vscode_version", got)
	}
}

func TestCopilotExecutorConfigPrefersAttributes(t *testing.T) {
	executor := NewCopilotExecutor(nil)
	auth := &cliproxyauth.Auth{
		Attributes: map[string]string{
			"base_url":       "https://attr.example.test/",
			"vscode_version": "1.100.0",
		},
		Metadata: map[string]any{
			"base_url":       "https://metadata.example.test",
			"vscode_version": "1.99.0",
		},
	}

	if got := executor.baseURL(auth); got != "https://attr.example.test" {
		t.Fatalf("baseURL() = %q, want attributes base_url", got)
	}
	if got := executor.vscodeVersion(auth); got != "1.100.0" {
		t.Fatalf("vscodeVersion() = %q, want attributes vscode_version", got)
	}
}
