package copilot

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSetCopilotHeaders(t *testing.T) {
	header := http.Header{}

	SetCopilotHeaders(header, "copilot-token", "1.99.0")

	if got := header.Get("Authorization"); got != "Bearer copilot-token" {
		t.Fatalf("Authorization = %q, want Bearer token", got)
	}
	if got := header.Get("Editor-Version"); got != "vscode/1.99.0" {
		t.Fatalf("Editor-Version = %q, want vscode/1.99.0", got)
	}
	if got := header.Get("Copilot-Integration-Id"); got != "vscode-chat" {
		t.Fatalf("Copilot-Integration-Id = %q, want vscode-chat", got)
	}
	if got := header.Get("X-Request-Id"); got == "" {
		t.Fatal("X-Request-Id is empty")
	}
}

func TestAccountHashStableAndMasked(t *testing.T) {
	first := AccountHash("alice:123")
	second := AccountHash(" alice:123 ")

	if first != second {
		t.Fatalf("AccountHash should trim input: %q != %q", first, second)
	}
	if len(first) != 8 {
		t.Fatalf("AccountHash length = %d, want 8", len(first))
	}
}

func TestFetchUsage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != GitHubAPIBaseURL+"/copilot_internal/user" {
			t.Fatalf("request URL = %q, want Copilot usage URL", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "token github-token" {
			t.Fatalf("Authorization = %q, want GitHub token header", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"copilot_plan":"individual",
				"quota_reset_date":"2026-07-01",
				"quota_snapshots":{
					"chat":{"entitlement":100,"remaining":90,"quota_remaining":90,"percent_remaining":90,"unlimited":false}
				}
			}`)),
			Request: req,
		}, nil
	})}

	usage, err := FetchUsage(context.Background(), client, "github-token", "1.99.0")
	if err != nil {
		t.Fatalf("FetchUsage() error = %v", err)
	}
	if usage.CopilotPlan != "individual" {
		t.Fatalf("CopilotPlan = %q, want individual", usage.CopilotPlan)
	}
	if got := usage.QuotaSnapshots["chat"].Remaining; got != 90 {
		t.Fatalf("chat remaining = %v, want 90", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
