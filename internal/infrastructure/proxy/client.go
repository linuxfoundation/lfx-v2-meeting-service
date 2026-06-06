// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

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

// Client implements domain.ITXProxyClient.
type Client struct {
	httpClient *http.Client
	config     Config
}

// NewClient creates a new ITX proxy client with OAuth2 M2M authentication using private key.
func NewClient(config Config) *Client {
	return &Client{
		httpClient: newAuthenticatedHTTPClient(config),
		config:     config,
	}
}
