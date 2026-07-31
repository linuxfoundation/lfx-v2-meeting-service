<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Sensitive data exposure

One pattern, and the strongest evidence in the whole KB: eight distinct findings
across three PRs over 20 days, **all eight fixed**, median turnaround about seven
minutes, with three commit messages naming the privacy rationale explicitly.

The reviews did not just get lines changed — they built the guard helpers this
repo now uses. That is why this entry is worth having even though
`.github/skills/**` states the PII theme qualitatively: the value here is the
**named helpers** and the exact status-only span form.

---

## `sensitive-identity-data-in-logs-errors-and-telemetry`

**Rule:** An email address, SFID, username, host key, or an upstream response
body must not reach a **log attribute or an OpenTelemetry span** without passing
through this repo's redaction helpers; spans carry status only.

**The error returned to the caller is out of scope and must stay that way.** The
evidenced fix's whole shape is that the detailed error still reaches the caller
while telemetry is reduced to a status — `mapHTTPError`
(`internal/infrastructure/proxy/client.go:1126-1151`) deliberately derives
`DomainError.Message` from the upstream body and returns it through
`domain.New*Error`. Flagging that would contradict the fix this entry is built
on. Only an error value that is **also logged or recorded on a span** unredacted
is in scope, and then the finding is the logging or the span, not the return.

**Severity:** `Important`

**Detect:** An email, SFID, username, `host_key`, raw `respBody`, `*url.Error`,
`itx.User` or `itx.CreatedUpdatedBy` value reaching a `slog` attribute
(including via `logging.ErrKey`) or `span.RecordError`/`span.SetStatus`, without
`redaction.Redact` or `redaction.RedactEmail`, without
`requestJSONForLog`/`responseJSONForLog`, without a `LogValue()` on the type, or
— for spans — without being exactly `fmt.Errorf("HTTP %d", statusCode)`.

**`LogValue()` satisfies a `slog` attribute and nothing else.** It governs `slog`
serialization only; it does not redact a value passed to
`span.RecordError`/`span.SetStatus` or built into an error message. Do not accept
it as a guard outside a `slog` attribute — spans need the status-only form, and
an error that is logged needs explicit redaction.

An `fmt.Errorf`/`fmt.Sprintf`/`domain.New*Error` message counts **only when that
error value is then logged or recorded on a span** without redaction. An error
constructed and returned to the caller is not a match on its own.

**Evidence:** distinct PRs `#210`, `#217`, `#226`; 8 findings, 8 fixed.

Sample — `#217` comment `discussion_r3561789468`, on
`internal/infrastructure/userservice/client.go:296`: *"`domainErr.Error()` is
derived directly from the response's `Message`/`error` fields and even falls back
to a raw body snippet. This file explicitly notes that user-service response
bodies contain email addresses (lines 269–270), so `RecordError(domainErr)`
exports those identifiers to Datadog on 5xx responses. Record a sanitized
status-only error while continuing to return the detailed domain error to the
caller."*

Fixed in `2cc508e`:

```go
- trace.SpanFromContext(ctx).RecordError(domainErr)
+ trace.SpanFromContext(ctx).RecordError(fmt.Errorf("HTTP %d", resp.StatusCode))
```

Live on `origin/main`, and the same status-only form now guards all **42** call
sites of `c.recordAndMapHTTPError(` in
`internal/infrastructure/proxy/client.go` (counted at `4ce62f6`, still 42 at
`4bb31d0`; the helper is defined at line 1157 and has had 42 call sites since it
was introduced in `dc70a88`). Note the shape of the fix: the detailed error still goes to the
caller; only the telemetry is reduced to a status.

**Guards that satisfy it — infrastructure these reviews created:**

- `pkg/redaction` — `Redact`, `RedactEmail` (predates the window).
- `internal/infrastructure/proxy/logredact.go` — `requestJSONForLog` (6 call
  sites), `responseJSONForLog` (12), and
  `auditFieldsForResponseRedaction = [created_by, updated_by, modified_by, file_uploaded_by]`.
- `LogValue() slog.Value` on `itx.User` — satisfies `slog` attributes only.
- For spans: exactly `fmt.Errorf("HTTP %d", statusCode)`.
- The whole `internal/infrastructure/transport/` package was deleted as part of
  this work.

**Why it is not tooling's job:** gitleaks matches committed literals, not runtime
dataflow; there is no `gosec` and no taint analysis. The only automated guard is
this repo's own `internal/infrastructure/proxy/client_otel_test.go` assertion
that `exception.message == "HTTP 500"` — itself written because of these
reviews.

**Not this pattern:** the deliberate placeholder credentials in tests
(`"test-user-token"`, `[]byte("test-secret")`) are fine. See
[`known-false-positives.md`](known-false-positives.md).
