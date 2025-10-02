// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"log/slog"
	"net/url"
	"os"
	"strconv"

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
	NatsURL            string
	Port               string
	SkipEtagValidation bool
	LFXEnvironment     string
	ProjectLogoBaseURL string
	EmailConfig        emailConfig
}

// emailConfig holds all email-related configuration
type emailConfig struct {
	Enabled      bool
	SMTPHost     string
	SMTPPort     int
	SMTPFrom     string
	SMTPUsername string
	SMTPPassword string
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
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	skipEtagValidation := false
	skipEtagValidationStr := os.Getenv("SKIP_ETAG_VALIDATION")
	if skipEtagValidationStr == "true" {
		skipEtagValidation = true
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

	return environment{
		NatsURL:            natsURL,
		Port:               port,
		SkipEtagValidation: skipEtagValidation,
		LFXEnvironment:     lfxEnvironment,
		ProjectLogoBaseURL: projectLogoBaseURL,
		EmailConfig:        parseEmailConfig(),
	}
}

// parseEmailConfig parses all email-related environment variables
func parseEmailConfig() emailConfig {
	enabled := true
	enabledStr := os.Getenv("EMAIL_ENABLED")
	if enabledStr == "false" {
		enabled = false
	}

	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 1025 // Default for Mailpit
	portStr := os.Getenv("SMTP_PORT")
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "noreply@lfx.linuxfoundation.org"
	}

	return emailConfig{
		Enabled:      enabled,
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPFrom:     from,
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
	}
}
