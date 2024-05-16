/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package armauth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"

	"sigs.k8s.io/cloud-provider-azure/pkg/azclient/utils"
	"sigs.k8s.io/cloud-provider-azure/pkg/azclient/vaultclient"
)

type SecretResourceID struct {
	SubscriptionID string
	ResourceGroup  string
	VaultName      string
	SecretName     string
}

func (s SecretResourceID) String() string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.KeyVault/vaults/%s/secrets/%s", s.SubscriptionID, s.ResourceGroup, s.VaultName, s.SecretName)
}

type KeyVaultCredential struct {
	secretClient     *azsecrets.Client
	vaultURI         string
	secretResourceID SecretResourceID

	mtx   sync.RWMutex
	token *azcore.AccessToken
}

type KeyVaultCredentialSecret struct {
	AccessToken string    `json:"access_token"`
	ExpiresOn   time.Time `json:"expires_on"`
}

func NewKeyVaultCredential(
	msiCredential azcore.TokenCredential,
	secretResourceID SecretResourceID,
) (*KeyVaultCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get KeyVault URI
	var vaultURI string
	{
		vaultCli, err := vaultclient.New(secretResourceID.SubscriptionID, msiCredential, utils.GetDefaultOption())
		if err != nil {
			return nil, fmt.Errorf("create KeyVault client: %w", err)
		}

		vault, err := vaultCli.Get(ctx, secretResourceID.ResourceGroup, secretResourceID.VaultName)
		if err != nil {
			return nil, fmt.Errorf("get vault %s: %w", secretResourceID.VaultName, err)
		}

		if vault.Properties == nil || vault.Properties.VaultURI == nil {
			return nil, fmt.Errorf("vault uri is nil")
		}
		vaultURI = *vault.Properties.VaultURI
	}

	cli, err := azsecrets.NewClient(vaultURI, msiCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("create secret client: %w", err)
	}

	rv := &KeyVaultCredential{
		secretClient:     cli,
		mtx:              sync.RWMutex{},
		secretResourceID: secretResourceID,
	}

	if err := rv.refreshToken(ctx); err != nil {
		return nil, fmt.Errorf("refresh token from %s: %w", secretResourceID, err)
	}

	return rv, nil
}

func (c *KeyVaultCredential) refreshToken(ctx context.Context) error {
	const (
		LatestVersion      = ""
		RefreshTokenOffset = 5 * time.Minute
	)

	{
		c.mtx.RLock()
		if c.token != nil && c.token.ExpiresOn.Add(RefreshTokenOffset).Before(time.Now()) {
			c.mtx.RUnlock()
			return nil
		}
		c.mtx.RUnlock()
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.token != nil && c.token.ExpiresOn.Add(RefreshTokenOffset).Before(time.Now()) {
		return nil
	}

	resp, err := c.secretClient.GetSecret(ctx, c.secretResourceID.SecretName, LatestVersion, nil)
	if err != nil {
		return err
	}
	if resp.Value == nil {
		return fmt.Errorf("secret value is nil")
	}

	var secret KeyVaultCredentialSecret
	if err := json.Unmarshal([]byte(*resp.Value), &secret); err != nil {
		return fmt.Errorf("unmarshal secret value `%s`: %w", *resp.Value, err)
	}

	c.token = &azcore.AccessToken{
		Token:     secret.AccessToken,
		ExpiresOn: secret.ExpiresOn,
	}

	return nil
}

func (c *KeyVaultCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if err := c.refreshToken(ctx); err != nil {
		return azcore.AccessToken{}, fmt.Errorf("refresh token: %w", err)
	}

	return *c.token, nil
}
