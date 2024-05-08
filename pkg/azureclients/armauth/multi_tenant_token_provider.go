package armauth

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/go-logr/logr"
)

// MultiTenantTokenProvider is the track1 multi-tenant token provider wrapper for track2 implementation.
type MultiTenantTokenProvider struct {
	logger               logr.Logger
	primaryCredential    azcore.TokenCredential
	auxiliaryCredentials []azcore.TokenCredential
	timeout              time.Duration
	scope                string
}

func NewMultiTenantTokenProvider(
	logger logr.Logger,
	primaryCredential azcore.TokenCredential,
	auxiliaryCredentials []azcore.TokenCredential,
) (*MultiTenantTokenProvider, error) {
	return &MultiTenantTokenProvider{
		logger:               logger,
		primaryCredential:    primaryCredential,
		auxiliaryCredentials: auxiliaryCredentials,
		timeout:              10 * time.Second,
	}, nil
}

func (p *MultiTenantTokenProvider) PrimaryOAuthToken() string {
	p.logger.V(4).Info("Fetching primary oauth token")
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	token, err := p.primaryCredential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{p.scope},
	})
	if err != nil {
		p.logger.Error(err, "Failed to fetch primary OAuth token")
		return ""
	}
	return token.Token
}

func (p *MultiTenantTokenProvider) AuxiliaryOAuthTokens() []string {
	p.logger.V(4).Info("Fetching auxiliary oauth token", "num-credentials", len(p.auxiliaryCredentials))
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	var tokens []string
	for _, cred := range p.auxiliaryCredentials {
		token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: []string{p.scope},
		})
		if err != nil {
			p.logger.Error(err, "Failed to fetch auxiliary OAuth token")
			return nil
		}

		tokens = append(tokens, token.Token)
	}

	return tokens
}
