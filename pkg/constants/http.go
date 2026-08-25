// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

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

// contextUsername is the type for the username context key
type contextUsername string

// UsernameContextID is the context ID for the username
const UsernameContextID contextUsername = "username"

// contextEmail is the type for the email context key
type contextEmail string

// EmailContextID is the context ID for the JWT-claimed email of the authenticated principal
const EmailContextID contextEmail = "email"
