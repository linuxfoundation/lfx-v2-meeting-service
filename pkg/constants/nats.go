// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// AuthEmailToUsernameSubject resolves a primary email address to an LFX username via the auth service.
const AuthEmailToUsernameSubject = "lfx.auth-service.email_to_username"

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
