// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/oauth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

const tokenExpiryLeeway = 60 * time.Second

// auth0TokenSource implements oauth2.TokenSource using Auth0 SDK with private key.
type auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
}

// Token implements the oauth2.TokenSource interface.
func (a *auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	body := oauth.LoginWithClientCredentialsRequest{
		Audience: a.audience,
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, body, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}

	return token.WithExtra(map[string]any{
		"scope": tokenSet.Scope,
	}), nil
}

func newAuthenticatedHTTPClient(config Config) *http.Client {
	ctx := context.Background()

	if config.PrivateKey == "" {
		panic("ITX_CLIENT_PRIVATE_KEY is required but not set")
	}

	otelClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   config.Timeout,
	}

	authConfig, err := authentication.New(
		ctx,
		config.Auth0Domain,
		authentication.WithClientID(config.ClientID),
		authentication.WithClientAssertion(config.PrivateKey, "RS256"),
		authentication.WithClient(otelClient),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create Auth0 client: %v (ensure ITX_CLIENT_PRIVATE_KEY contains a valid RSA private key in PEM format)", err))
	}

	tokenSource := &auth0TokenSource{
		ctx:        ctx,
		authConfig: authConfig,
		audience:   config.Audience,
	}
	reuseTokenSource := oauth2.ReuseTokenSource(nil, tokenSource)

	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)
	httpClient.Timeout = config.Timeout

	return httpClient
}
