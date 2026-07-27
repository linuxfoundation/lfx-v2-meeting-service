---
name: meeting-service-code-review
description: >
  How to judge the implementation of an lfx-v2-meeting-service pull request:
  the line-level grounding method, the quality dimensions, the traps specific to
  this Goa service's two surfaces (the ITX proxy and the v1 KV event pipeline),
  and the security anchors for meeting join credentials, visibility, and PII.
  Use on every PR that changes code, however small.
---

<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Meeting Service Code Review

The `/copilot-code-reviewer` skill owns the reviewer's scope and signal
discipline; this skill owns the line-level method.

A diff alone is not enough. For each non-trivial hunk, read the **whole changed
function**, not just the diff lines, and read the layer either side of it: for a
proxy change, the Goa method in `design/`, the API handler, the service, and the
ITX client method it ends up calling; for an event-pipeline change, the KV key
that routes to the handler and the message the handler ultimately publishes.
Then grep for **sibling implementations** of the same pattern — this codebase is
deliberately repetitive, so the neighboring handler or client method is usually
the reference answer, and comparing against it is the fastest way to see what a
hunk omits. Divergence is a lead, not a verdict: raise it when the sibling shows
the change gets something wrong — a dropped error classification, a missing
redaction, an ack where the neighbor retries, a field the contract expects —
and let it go when the code simply reads differently and still behaves
correctly.

## The house standards

Read the parts relevant to the diff before judging, every run, and name the
documented source in any standards finding:

- **`docs/indexer-contract.md`** and **`docs/fga-contract.md`** — declared
  authoritative for every message this service sends to the indexer and to
  fga-sync, and both state they must be updated in the same PR as any change to
  message construction. No linter enforces that, so a diff that changes a
  published field, tag, reference, or subject without touching the matching
  contract doc is a finding. So is a doc edit that no longer matches the code.
- **`docs/api-contracts/`** — one file per ITX resource family, giving both the
  proxy-side and the ITX-side schema and, for most operations, the permission
  the caller must already hold. Check a proxy change against the relevant file
  before assuming a field name, an optionality, or a status code.
- **`docs/event-processing.md`** — the KV event architecture, the key routing,
  the transformation patterns, and the configuration knobs.
- **`docs/itx-proxy-implementation.md`** and **`docs/tracing.md`** — the layered
  request flow and the OpenTelemetry configuration surface.
- **`CLAUDE.md`** and **`README.md`** — architecture overview, the make targets,
  and the environment-variable contract. Useful for intent; confirm any specific
  they give against the code, because both have drifted in places.

Enforcement runs in both directions: code that violates a documented standard is
a finding, and where this change leaves a documented standard behind, the doc
needs updating in the same PR. Pre-existing drift the change does not touch is
not a finding.

## Quality dimensions

Run these on the changed code, scaled to the size of the change:

- **Correctness.** Does it do what it claims? Watch nil-pointer dereferences on
  the optional fields that pervade both the Goa payloads and the ITX models,
  type assertions on the `map[string]any` decoded from KV records, silently
  dropped errors, `context` not threaded through to the outbound call, and
  goroutines started without a way to observe their failure.
- **Error handling.** Errors are classified with the semantic types in
  `internal/domain/errors.go`, and the API layer maps that classification onto
  the HTTP status. A new failure path that returns a bare `error`, or that
  classifies a caller mistake as internal (or an internal fault as a validation
  error), produces the wrong status and the wrong client behavior. Nothing
  should be swallowed, and no upstream ITX body or internal detail should be
  passed through to the caller verbatim.
- **Tests.** New or changed behavior needs tests that assert real behavior
  rather than that a mock was called. Converters, occurrence calculation, KV
  routing, and retry decisions are cheap to test and expensive to get wrong;
  missing tests on a contract-bearing or security-sensitive path are worth a
  finding when you can name the contract or security consequence left unguarded.
- **Concurrency.** `make test` enables the race detector locally, but the CI
  test step does not, so do not assume a data race would have been caught before
  review. Shared state written from a handler, an unbounded fan-out over KV
  records, and a goroutine outliving the request context are all worth reading
  carefully.
- **Readability and structure.** The change should read like the surrounding
  code; names should say what a thing is; duplicated logic that traps the next
  editor wants a shared helper.
- **Code truthfulness.** Comments, docs, and the PR description must match what
  the code does. A stale comment, a dead branch, or a contract doc describing a
  field the code no longer emits is a finding.

## Service specifics worth a second look

- **The design is the source; `gen/` is the output.** API shape, validation, and
  the security scheme all live in the Goa DSL under `design/`. A hand-edit to
  `gen/` will be silently overwritten, and a `design/` change whose regenerated
  code was never committed still passes CI while the released image compiles the
  committed `gen/`. Flag either.
- **Auth is declared, not coded, per endpoint.** Whether a route is
  authenticated is set in the Goa DSL, not in the handler: attaching the JWT
  security scheme is what causes the token to be verified and the principal to
  be placed on the context. A method that omits it is an unauthenticated
  endpoint, and nothing in the handler will make that obvious. Some routes are
  deliberately public and need no substitute guard — the `/livez` and `/readyz`
  health checks and the served OpenAPI documents are all unauthenticated on
  purpose. Raise the omission when an unauthenticated method reads or returns
  data a caller should not see, or mutates state, and nothing else — a signature
  check, the upstream gateway — stands in front of it. On a new endpoint that
  touches meeting data, this is the single highest-value thing to check.
- **The ITX wire models are lossy by design.** Fields in `pkg/models/itx/` are
  frequently non-pointer with `omitempty`, so a deliberate zero, empty string,
  or `false` is simply absent from the request. Updates go out as `PUT`s, some
  reusing the create request struct and some carrying a dedicated update shape,
  so check which one the endpoint under review uses before reasoning about
  unset semantics: a converter that maps an "unset" the wrong way can silently
  clear, or fail to clear, a value in ITX.
  Check new converter fields against the relevant `docs/api-contracts/` schema,
  and check that the pointer helpers in `pkg/utils/ptr.go` are used with the
  semantics their names imply.
- **The retry/ack contract in the event pipeline.** Every failure path in a KV
  handler is a choice between redelivering the message with backoff and
  acknowledging it, and it should be an explicit one. Acknowledging a transient
  failure drops the event permanently; redelivering a permanently malformed
  record burns redeliveries until the consumer's max-deliver limit. Check which
  one a new failure path selects and whether that matches the error it handles.
- **At-least-once means idempotent.** Redelivery, consumer restarts, and KV
  re-writes all replay events. Any externally visible effect a handler
  performs — publishing to the indexer or fga-sync, writing an ID mapping,
  sending an invite — must be safe to repeat. A handler that is only correct the
  first time is a finding.
- **ID mapping is optional and degrades to pass-through.** When mapping is
  disabled or unavailable the service uses a no-op mapper that returns the input
  unchanged. Code that assumes a mapped v2 UUID is always a UUID, or that treats
  an unmapped identity result as an error, will behave differently in the two
  configurations.
- **Changed constants are behavior changes.** Timeouts, retry and backoff
  values, meeting duration and early-join caps, NATS subjects and queue groups,
  KV bucket and stream names, and the Helm chart's default values all change
  runtime behavior even when the code still compiles. Ask whether the change is
  stated and intentional and what its blast radius is; an unexplained constant
  change is a finding.
- **Deployment surface.** A new environment variable or configuration knob has
  to reach the running service through the chart under
  `charts/lfx-v2-meeting-service/`; config read in Go but never plumbed through
  the chart is a change that works locally and not in a cluster.

## Security anchors

These describe the boundaries that make a diff security-relevant here; verify
the concrete mechanism in the code each time rather than assuming a guard
exists. Report only high-confidence, concretely reachable findings, name the
file and function, and say what an attacker controls.

- **Meeting join credentials are data this service handles routinely.** Zoom
  passcodes, the host key, the join URL, and the join-page password flow through
  the ITX models, the event handlers, and the indexer payload — the indexed
  meeting record deliberately carries them. That makes it normal for
  them to appear in a diff and abnormal for them to appear anywhere new:
  in a log line, an error message, a trace attribute, a test fixture committed
  with a real value, or a response shape a caller was not already entitled to.
  `pkg/redaction` exists for the logging case; a new log or error that prints a
  passcode, host key, join URL, token, or raw email instead of a redacted form
  is a finding.
- **Visibility and access flags decide who can reach a meeting.** The fga-sync
  access config derives an object's public flag from the meeting's visibility,
  and the indexed record also carries visibility, the restricted flag, and the
  artifact-access levels for recordings, transcripts, and summaries. A
  transformation that defaults one of these to the permissive value, drops it so
  the downstream service falls back to a default, or inverts it, is an
  authorization defect expressed as a data bug — and it lands in OpenFGA, not
  here. Treat any diff touching those fields as security-relevant.
- **fga-sync messages are the access-control write path.** A missed
  `delete_access` or `member_remove` on a delete leaves tuples granting access
  to a resource that no longer exists; a wrong object type or UID writes tuples
  against the wrong object. Check delete and soft-delete paths as carefully as
  create paths, and check them against `docs/fga-contract.md`.
- **The ITX credentials are the service's, not the caller's.** Outbound calls
  carry the service's OAuth2 M2M token and a scope header, so ITX will honor any
  request this service chooses to make. Anything that lets a caller influence
  which object is acted on beyond what the route's own path and payload
  validation permits — an unvalidated identifier interpolated into an ITX path,
  a caller-supplied field used to select a different meeting or project, an
  identifier format looser than the upstream contract — is an authorization
  problem even though the local code "just proxies".
- **The M2M private key and the OAuth2 token.** The ITX client private key and
  the tokens minted from it must never reach a log, an error message, a trace
  attribute, or a test fixture, and must not be widened in the chart's
  configuration surface (for example, moved from a secret into plain values).
- **NATS request/reply surfaces are not behind the HTTP auth.** Subjects this
  service answers are reachable by anything on the NATS connection; where a
  request payload carries a user bearer token, that token *is* the
  authorization, and the handler must resolve the user from it rather than
  trusting any identifier supplied alongside it. A new subject that acts on a
  caller-named user without deriving that user from the token is a finding, as
  is one that echoes the token back or logs it.
- **PII in the pipeline.** Registrant and participant records carry names,
  email addresses, and identity references, and user enrichment adds more. Flag
  a new log line, error, span attribute, or published message that emits raw PII
  where the surrounding code redacts it, or that widens which downstream service
  receives it.
- **Committed secrets.** A *real* credential, private key, token, or connection
  string in the diff is a finding anywhere it appears — tests, fixtures, chart
  values, workflow files — even when the code path that reads it is dead. The
  tests here deliberately use obvious placeholders (`"test-user-token"`,
  `[]byte("test-secret")`); those are fine and flagging them is noise. Judge
  whether the value would actually authenticate somewhere, not whether the
  variable is called a token.

Do not raise generic hardening with no concrete vulnerability, denial of service
or "add rate limiting" on its own, outdated third-party dependency versions,
theoretical timing issues, or the observation that an identifier is guessable —
an authorization finding rests on a missing check, not on unguessability.

## Judgment calls

- **Point at the working pattern.** When the diff diverges, cite the sibling
  handler, converter, or client method that does it correctly rather than
  describing an abstract ideal.
- **Do not propose rewrites of a sound approach**, and do not suggest change for
  its own sake; working, readable code needs no improvement.
- **Know your limits.** Report what you can show is wrong. When a judgment rests
  on something you cannot see — the ITX API's actual behavior, the OpenFGA
  model, the indexer's tolerance for a missing field, a deployed configuration
  value — you cannot confirm the defect, so do not raise it. A conditional
  finding still costs the author a full investigation and is the kind of comment
  that teaches a team to skim reviews. Silence is the correct output.
