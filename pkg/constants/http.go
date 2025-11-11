// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

import (
	"fmt"
	"net/url"
)

// Constants for the HTTP request headers
const (
	// AuthorizationHeader is the header name for the authorization
	AuthorizationHeader string = "authorization"

	// RequestIDHeader is the header name for the request ID
	RequestIDHeader string = "X-REQUEST-ID"

	// EtagHeader is the header name for the ETag
	EtagHeader string = "ETag"

	// XOnBehalfOfHeader is the header name for the on behalf of principal
	XOnBehalfOfHeader string = "x-on-behalf-of"
)

// contextRequestID is the type for the request ID context key
type contextRequestID string

// RequestIDContextID is the context ID for the request ID
const RequestIDContextID contextRequestID = "X-REQUEST-ID"

// contextAuthorization is the type for the authorization context key
type contextAuthorization string

// AuthorizationContextID is the context ID for the authorization
const AuthorizationContextID contextAuthorization = "authorization"

// contextPrincipal is the type for the principal context key
type contextPrincipal string

// PrincipalContextID is the context ID for the principal
const PrincipalContextID contextPrincipal = "x-on-behalf-of"

type contextEtag string

// ETagContextID is the context ID for the ETag
const ETagContextID contextEtag = "etag"

// LFX app domain constants
const (
	// LFXDomainDev is the development domain
	LFXDomainDev = "app.dev.lfx.dev"
	// LFXDomainStaging is the staging domain
	LFXDomainStaging = "app.staging.lfx.dev"
	// LFXDomainProd is the production domain
	LFXDomainProd = "app.lfx.dev"
)

// GetLFXAppDomain returns the appropriate LFX app domain based on the environment
// Environment should be one of: "dev", "staging", "prod"
func GetLFXAppDomain(environment string) string {
	switch environment {
	case "dev":
		return LFXDomainDev
	case "staging":
		return LFXDomainStaging
	case "prod":
		return LFXDomainProd
	default:
		// Default to production domain if environment is not one of the expected values
		return LFXDomainProd
	}
}

// LfxURLGenerator generates LFX app URLs with environment-specific domains or custom app origins
type LfxURLGenerator struct {
	environment     string
	customAppOrigin string
}

// NewLfxURLGenerator creates a new LfxURLGenerator with the given environment and optional custom app origin
func NewLfxURLGenerator(environment, customAppOrigin string) *LfxURLGenerator {
	return &LfxURLGenerator{
		environment:     environment,
		customAppOrigin: customAppOrigin,
	}
}

// GenerateMeetingURL generates the LFX app meeting URL with the given meeting UID and password
func (g *LfxURLGenerator) GenerateMeetingURL(meetingUID, password string) string {
	if g.customAppOrigin != "" {
		return fmt.Sprintf("%s/meetings/%s?password=%s", g.customAppOrigin, meetingUID, url.QueryEscape(password))
	}
	domain := GetLFXAppDomain(g.environment)
	return fmt.Sprintf("https://%s/meetings/%s?password=%s", domain, meetingUID, url.QueryEscape(password))
}

// GenerateMeetingDetailsURL generates the LFX app project meetings page URL with the given project slug and meeting UID
func (g *LfxURLGenerator) GenerateMeetingDetailsURL(projectSlug, meetingUID string) string {
	if g.customAppOrigin != "" {
		return fmt.Sprintf("%s/project/%s/meetings#meeting-%s", g.customAppOrigin, projectSlug, meetingUID)
	}
	domain := GetLFXAppDomain(g.environment)
	return fmt.Sprintf("https://%s/project/%s/meetings#meeting-%s", domain, projectSlug, meetingUID)
}
