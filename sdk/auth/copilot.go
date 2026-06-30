package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	copilotauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/copilot"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/browser"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

// CopilotAuthenticator implements GitHub device-code login for GitHub Copilot.
type CopilotAuthenticator struct{}

func NewCopilotAuthenticator() *CopilotAuthenticator { return &CopilotAuthenticator{} }

func (a *CopilotAuthenticator) Provider() string { return copilotauth.Provider }

func (a *CopilotAuthenticator) RefreshLead() *time.Duration { return nil }

func (a *CopilotAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*coreauth.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	httpClient := util.SetProxy(&cfg.SDKConfig, &http.Client{})
	deviceCode, err := copilotauth.RequestDeviceCode(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	fmt.Println("Starting GitHub Copilot device authentication...")
	fmt.Printf("GitHub device URL: %s\n", deviceCode.VerificationURI)
	fmt.Printf("GitHub device code: %s\n", deviceCode.UserCode)

	if !opts.NoBrowser {
		if !browser.IsAvailable() {
			log.Warn("No browser available; please open the GitHub device URL manually")
		} else if errOpen := browser.OpenURL(deviceCode.VerificationURI); errOpen != nil {
			log.Warnf("Failed to open browser automatically: %v", errOpen)
		}
	}

	githubToken, err := copilotauth.PollAccessToken(ctx, httpClient, deviceCode)
	if err != nil {
		return nil, err
	}

	user, err := copilotauth.FetchUser(ctx, httpClient, githubToken)
	if err != nil {
		return nil, err
	}
	login := strings.TrimSpace(user.Login)
	if login == "" {
		login = fmt.Sprintf("github-%d", user.ID)
	}
	accountHash := copilotauth.AccountHash(fmt.Sprintf("%s:%d", login, user.ID))
	fileName := fmt.Sprintf("copilot-%s.json", accountHash)

	fmt.Printf("GitHub Copilot authentication successful as %s\n", login)

	return &coreauth.Auth{
		ID:       fileName,
		Provider: a.Provider(),
		FileName: fileName,
		Label:    login,
		Metadata: map[string]any{
			"type":         a.Provider(),
			"email":        login,
			"login":        login,
			"github_id":    user.ID,
			"github_token": githubToken,
		},
		Attributes: map[string]string{
			"auth_kind": "oauth",
		},
	}, nil
}
