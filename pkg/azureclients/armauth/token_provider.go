package armauth

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/go-logr/logr"
)

// TokenProvider is the track1 token provider wrapper for track2 implementation.
type TokenProvider struct {
	logger     logr.Logger
	credential azcore.TokenCredential
	timeout    time.Duration
	scope      string
}

func NewTokenProvider(
	logger logr.Logger,
	credential azcore.TokenCredential,
	scope string,
) (*TokenProvider, error) {
	return &TokenProvider{
		logger:     logger,
		credential: credential,
		timeout:    10 * time.Second,
		scope:      scope,
	}, nil
}

func (p *TokenProvider) OAuthToken() string {
	p.logger.V(4).Info("Fetching OAuth token")
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	token, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{p.scope},
	})
	if err != nil {
		p.logger.Error(err, "Failed to fetch OAuth token")
		return ""
	}
	p.logger.V(4).Info("Fetched OAuth token successfully", "token", token.Token)
	return token.Token
}
