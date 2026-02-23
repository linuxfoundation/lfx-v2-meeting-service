// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"log/slog"
	"net/url"
	"os"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// flags are the command line flags for the meeting service.
type flags struct {
	Debug bool
	Port  string
	Bind  string
}

// environment are the environment variables for the meeting service.
type environment struct {
	Port               string
	LFXEnvironment     string
	ProjectLogoBaseURL string
	LFXAppOrigin       string
	ITXConfig          itxConfig
	IDMappingDisabled  bool
}

// itxConfig holds ITX proxy configuration
type itxConfig struct {
	BaseURL     string
	ClientID    string
	PrivateKey  string
	Auth0Domain string
	Audience    string
}

// parseFlags parses command line flags for the meeting service
func parseFlags(defaultPort string) flags {
	var debug = flag.Bool("d", false, "enable debug logging")
	var port = flag.String("p", defaultPort, "listen port")
	var bind = flag.String("bind", "*", "interface to bind on")

	flag.Usage = func() {
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()

	// Based on the debug flag, set the log level environment variable used by [log.InitStructureLogConfig]
	if *debug {
		err := os.Setenv("LOG_LEVEL", "debug")
		if err != nil {
			slog.With(logging.ErrKey, err).Error("error setting log level")
			os.Exit(1)
		}
	}

	return flags{
		Debug: *debug,
		Port:  *port,
		Bind:  *bind,
	}
}

// parseEnv parses environment variables for the meeting service
func parseEnv() environment {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	lfxEnvironmentRaw := os.Getenv("LFX_ENVIRONMENT")
	var lfxEnvironment string
	switch lfxEnvironmentRaw {
	case "dev", "development":
		lfxEnvironment = "dev"
	case "staging", "stg", "stage":
		lfxEnvironment = "staging"
	case "prod", "production":
		lfxEnvironment = "prod"
	default:
		lfxEnvironment = "prod" // Default to production
	}

	projectLogoBaseURL := os.Getenv("PROJECT_LOGO_BASE_URL")
	if projectLogoBaseURL != "" {
		// Validate that the provided URL is valid
		if _, err := url.Parse(projectLogoBaseURL); err != nil {
			slog.With(logging.ErrKey, err, "url", projectLogoBaseURL).Error("invalid PROJECT_LOGO_BASE_URL provided, using default")
			projectLogoBaseURL = ""
		}
	}

	if projectLogoBaseURL == "" {
		// Default to the existing S3 bucket pattern
		projectLogoBaseURL = "https://lfx-one-project-logos-png-" + lfxEnvironment + ".s3.us-west-2.amazonaws.com"
	}

	lfxAppOrigin := os.Getenv("LFX_APP_ORIGIN")

	idMappingDisabled := os.Getenv("ID_MAPPING_DISABLED") == "true"

	return environment{
		Port:               port,
		LFXEnvironment:     lfxEnvironment,
		ProjectLogoBaseURL: projectLogoBaseURL,
		LFXAppOrigin:       lfxAppOrigin,
		ITXConfig:          parseITXConfig(),
		IDMappingDisabled:  idMappingDisabled,
	}
}

// parseITXConfig parses ITX proxy configuration from environment variables
func parseITXConfig() itxConfig {
	clientID := os.Getenv("ITX_CLIENT_ID")
	if clientID == "" {
		slog.Error("ITX_CLIENT_ID environment variable is required but not set")
		os.Exit(1)
	}

	privateKey := os.Getenv("ITX_CLIENT_PRIVATE_KEY")
	if privateKey == "" {
		slog.Error("ITX_CLIENT_PRIVATE_KEY environment variable is required but not set")
		os.Exit(1)
	}

	baseURL := os.Getenv("ITX_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.dev.itx.linuxfoundation.org"
	}

	auth0Domain := os.Getenv("ITX_AUTH0_DOMAIN")
	if auth0Domain == "" {
		auth0Domain = "linuxfoundation-dev.auth0.com"
	}

	audience := os.Getenv("ITX_AUDIENCE")
	if audience == "" {
		audience = "https://api.dev.itx.linuxfoundation.org/"
	}

	return itxConfig{
		BaseURL:     baseURL,
		ClientID:    clientID,
		PrivateKey:  privateKey,
		Auth0Domain: auth0Domain,
		Audience:    audience,
	}
}
