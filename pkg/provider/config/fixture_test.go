package config

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type DummyTokenCredential struct {
	token string
}

func NewDummyTokenCredential(token string) *DummyTokenCredential {
	return &DummyTokenCredential{
		token: token,
	}
}

func (d *DummyTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: d.token,
	}, nil
}
