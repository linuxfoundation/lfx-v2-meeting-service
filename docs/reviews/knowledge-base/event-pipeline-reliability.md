<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Event pipeline reliability

Three patterns from the v1→v2 KV event pipeline, all mined from PR `#218` and
its follow-ups. They share one consequence: a transient infrastructure failure
is converted into **permanent** data loss, because the handler destroys the only
record it would have needed to recover.

PR `#218` is titled *"fix: remove stale FGA access when registrant username is
cleared on update"*. These findings are what stopped that fix from being
undone by a NATS blip.

None of the three is visible to `errcheck`: in every case the error **is**
assigned and **is** compared to `nil`.

---

## `kv-get-error-treated-as-absent`

**Rule:** Only `jetstream.ErrKeyNotFound` may be treated as a missing key. Any
other error from a KV read is a transient infrastructure failure and must return
the retry decision.

**Severity:** `high`

**Detect:** In `cmd/meeting-api/eventing/**`, a `v1MappingsKV` or `v1ObjectsKV`
`.Get` whose only success test is `err == nil`, with no
`else if !errors.Is(err, jetstream.ErrKeyNotFound) { return true }` arm, **where**
the branch decides `ActionCreated` vs `ActionUpdated`, recovers a username or ID
used to build an FGA payload, or gates a `member_remove`.

**Evidence:** `#218` comment `discussion_r3572666267`, on
`cmd/meeting-api/eventing/registrant_event_handler.go:212`: *"A transient
mapping lookup failure is currently treated as “not found.” In that case this
update is published as a create, the previous username is never revoked, and the
mapping is overwritten with the new username, permanently losing the information
needed to clean up the stale tuple. Only `jetstream.ErrKeyNotFound` should mean
create; retry other lookup errors."*

Fixed in `b590e66` *"fix: retry on transient KV and publish failures in
registrant handlers"*:

```go
+	} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
+		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error reading registrant mapping, will retry")
+		return true
 	}
```

Survives verbatim on
`origin/main:cmd/meeting-api/eventing/registrant_event_handler.go:206-211`. The
same shape was fixed on the delete path (`discussion_r3572666326`) and on both
participant handlers (`f6c341e`).

**Guards that satisfy it:** an `errors.Is(err, jetstream.ErrKeyNotFound)`
discrimination that returns the retry decision for every other error. This is
also the classification `docs/event-processing.md` documents.

**Live anchors** (context, not work items): the un-discriminated form remains on
`main` at `cmd/meeting-api/eventing/attachment_event_handler.go:146,346`,
`cmd/meeting-api/eventing/meeting_event_handler.go:653`,
`cmd/meeting-api/eventing/past_meeting_event_handler.go:252`,
`cmd/meeting-api/eventing/recording_event_handler.go:227`,
`cmd/meeting-api/eventing/registrant_event_handler.go:609`, and
`cmd/meeting-api/eventing/summary_event_handler.go:177`.

---

## `swallowed-failure-before-state-destroying-write`

**Rule:** When a publish or KV write fails, the handler must return the retry
decision before any later statement overwrites the mapping, xref or tombstone
that the failed step was the only record of.

**Severity:** `high`

**Detect:** In a handler returning `(retry bool)`, a failed
`h.publisher.Publish*` or `h.v1MappingsKV.Put` is logged without
`return isTransientError(err)` or `return true`, **and** a later statement in the
same function overwrites the mapping, xref or tombstone that the failed step was
the only record of — so `cmd/meeting-api/eventing/event_processor.go` ACKs the
message on the `false` return and the event is gone.

**Evidence:** `#218` comment `discussion_r3572525956`, on
`cmd/meeting-api/eventing/registrant_event_handler.go:245`: *"When this publish
fails, execution continues and stores the new username, permanently discarding the only record of which
tuple still needs removal. A transient NATS outage therefore reproduces the
stale-access bug. Return the retry decision here, as the delete handler already
does for `PublishAccessDelete`."*

Fixed in `b590e66` by adding `return isTransientError(err)` after the warn log,
with matching fixes on the mapping `Put` (`discussion_r3572526031`) and on both
participant tombstone writes (`f6c341e`). All survive on `origin/main`.

**Guards that satisfy it:** `return isTransientError(err)`
(`cmd/meeting-api/eventing/utils.go:56`) or `return true` on the failure path.
Note the deliberate exception: best-effort enrichment and invite sending log and
continue on purpose, and those are not this pattern — the pattern requires a
*later state-destroying write in the same function*.

**Live anchors** (context, not work items): the xref writes at
`cmd/meeting-api/eventing/participant_event_handler.go:256-258` and `663-665`
still log without returning — the same shape, never separately raised.

---

## `unsafe-mapping-value-encoding`

**Rule:** A KV mapping value must use an encoding that tolerates arbitrary field
contents, because the usernames this service handles contain the delimiter —
invite acceptance forwards identities such as `auth0|guest`.

**Severity:** `high`

**Detect:** A KV mapping value built with `fmt.Sprintf("%s|%s|…")` or parsed with
`strings.Split*(value, "|", n)` where a field can hold an LF SSO username
(`participantData.Username`, `registrantData.Username`, `v1Data["lf_sso"]`,
`Invite.AcceptedBy`); or a new meaning attached to the legacy `"1"` sentinel
without a `scripts/` backfill; or a change to `buildRegistrantMappingValue` or
`parseRegistrantMappingValue` that is not mirrored in
`scripts/reconcile_meeting_registrants/common.py:mapping_state`.

**Evidence — two distinct PRs, producer and consumer:**

- `#218` comment `discussion_r3572666364`: *"The delimiter is unsafe for
  usernames this service actually handles: invite acceptance forwards identity
  values such as `auth0|guest` to ITX (`registrant_event_handler_test.go:184-198`).
  This encoding becomes `uid|auth0|guest|meeting`, and `SplitN(..., 3)` decodes
  the username as `auth0`, so a later update/delete removes the wrong FGA
  principal and leaves `auth0|guest` stale. Use an encoding that permits
  arbitrary field contents, such as JSON."* Fixed in `43c95e6` — a
  `registrantMappingData` struct, `json.Marshal` in
  `buildRegistrantMappingValue`, and a JSON-first parse with legacy pipe
  fallback. Live on
  `origin/main:cmd/meeting-api/eventing/registrant_event_handler.go:655-684`, and the
  `auth0|guest` premise still holds
  (`cmd/meeting-api/eventing/registrant_event_handler_test.go:258,267,273,286`).
- `#220` comment `discussion_r3598000819`, on
  `scripts/reconcile_meeting_registrants/common.py:106`: *"Current live
  registrant mappings are not the literal `\"1\"`: the meeting event handler
  writes JSON via `buildRegistrantMappingValue` … Consequently every normal
  current mapping is reported as `unknown`, and `_validate_candidate_state`
  aborts the dry-run before a plan can be produced."* Fixed in `e933d4b` —
  `mapping_state` now decodes both the JSON and pipe forms with identity
  matching.

**Guards that satisfy it:** `json.Marshal`/`json.Unmarshal` of a named struct,
with the legacy form handled on read; and, for a producer-side change, a
matching update to the Python consumer's `mapping_state`.

**Why it is not tooling's job:** nothing here type-checks a KV value's
encoding, and the Python consumer is not linted or tested in CI at all.
