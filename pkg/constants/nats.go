// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// AuthEmailToUsernameSubject resolves a primary email address to an LFX username via the auth service.
const AuthEmailToUsernameSubject = "lfx.auth-service.email_to_username"

// AuthUserMetadataSubject resolves a plain-text LFX username (or subject identifier or JWT) to the
// user's profile (name, given/family name, picture, etc.) via the auth service.
// Request: plain-text username. Reply: {"success":bool,"data":{...}} or {"success":false,"error":"..."}.
const AuthUserMetadataSubject = "lfx.auth-service.user_metadata.read"

// AuthUserEmailsSubject resolves a plain-text LFX username to the user's email records via the
// auth service. Request: {"user":{"auth_token":"<username>"}}.
// Reply: {"success":bool,"data":{"primary_email":string,"alternate_emails":[{"email":string,"verified":bool}]}}
// or {"success":false,"error":"..."}.
const AuthUserEmailsSubject = "lfx.auth-service.user_emails.read"

// PreferredEmailGetSubject is the NATS RPC subject for reading a user's preferred
// meeting-invite email. Request: {"token":"<user bearer token>"}. The user is resolved from
// the token (the RPC calls user-service as the user). Reply: {"email_id","email"}.
const PreferredEmailGetSubject = "lfx.meeting-service.preferred_email.get"

// PreferredEmailSetSubject is the NATS RPC subject for setting a user's preferred
// meeting-invite email. Request: {"token":"<user bearer token>","email":<string|null>,"email_id":<string|null>}.
// "email" (a verified address, resolved to its SFDC email-record ID) takes precedence over
// "email_id" when both are set; a null/empty selection or "primary" clears the override.
// Reply: {"email_id","email"}.
const PreferredEmailSetSubject = "lfx.meeting-service.preferred_email.set"

// PreferredEmailQueueGroup is the NATS queue group for the preferred-email responder,
// so multiple service replicas load-balance RPC requests.
const PreferredEmailQueueGroup = "meeting-service-preferred-email"
