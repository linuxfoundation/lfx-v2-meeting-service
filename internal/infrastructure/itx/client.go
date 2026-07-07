// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"net/http"
	"time"
)

// Config holds ITX proxy configuration.
type Config struct {
	BaseURL     string
	ClientID    string
	PrivateKey  string // RSA private key in PEM format
	Auth0Domain string
	Audience    string
	Timeout     time.Duration
}

// Client implements domain.ITXProxyClient and exposes typed resource accessors.
type Client struct {
	httpClient *http.Client
	config     Config
}

// NewClient creates a new ITX client with OAuth2 M2M authentication using private key.
func NewClient(config Config) *Client {
	return &Client{
		httpClient: newAuthenticatedHTTPClient(config),
		config:     config,
	}
}

// NewClientWithHTTPClient creates a Client that uses the provided HTTP client.
// Intended for tests and tooling that do not need Auth0 M2M authentication.
func NewClientWithHTTPClient(config Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		config:     config,
	}
}
