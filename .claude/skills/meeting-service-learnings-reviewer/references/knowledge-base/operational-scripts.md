<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Operational scripts (`scripts/**`)

`scripts/**` is this repo's **uncovered high-risk surface**, and these two
patterns are the clearest reason the KB exists at all:

- Neither `.github/skills/copilot-code-reviewer/SKILL.md` nor
  `.github/skills/meeting-service-code-review/SKILL.md` mentions `scripts/`
  anywhere.
- Three PRs in a 30-day window (`#218`, `#220`, `#224`) added one-shot
  operational scripts, and **every one drew serious findings**.
- The work is irreversible: the `host_key` scrub permanently destroys the only
  copy of a meeting's join PIN.
- Go under `scripts/**` builds through the root `go.mod`, but the configured
  linters do not detect either semantic failure below. Python under `scripts/**`
  is linted and tested by **nothing** — MegaLinter runs the Go flavor, and no
  workflow or Makefile target runs `pytest`, leaving
  `scripts/reconcile_meeting_registrants/` (a 1,497-line production data-repair
  tool with 69 tests) unchecked.

---

## `scripts-destructive-step-ungated-on-publish-success`

**Rule:** A destructive step in `scripts/**` must not run until the producing
work is verified complete and flushed — zero failures, zero skips, and, for
core-NATS publishes, an explicit flush or drain.

**Severity:** `critical`

**Detect:** In `scripts/**`, a destructive operation (`_update_by_query`,
`_delete_by_query`, KV `Delete`/`Purge`, a field-removal, or a soft-delete write)
reachable without (a) a check that the producing phase's `failed` **and**
`skipped` counters are both zero, and (b) — when the producer used core-NATS
`nc.Publish` — an intervening `nc.Flush`, `nc.FlushTimeout` or `nc.Drain`.

**Evidence:** `#224` comment `discussion_r3646887342`, on
`scripts/backfill_meeting_host_credentials/main.go:215`: *"This destructive query
runs even when `processPage` recorded failed/skipped publishes, and
`nats.Conn.Publish` only queues a core-NATS message—it does not confirm that the
indexer stored the credentials document. An unavailable subscriber or per-record
failure can therefore be followed by deleting every source `host_key`, making
those credentials unrecoverable."*

Fixed across two commits: `caf3897` added
`if failed > 0 || skipped > 0 { … return 1 }` and changed `nc.Close()` to
`nc.Drain()`; `e944495` added `nc.FlushTimeout(30 * time.Second)` before the
scrub. Both live on `origin/main:197-209`.

**Scope limit — do not overreach.** The reviewer's stronger ask, that the script
verify the indexer actually *stored* the documents, was **not implemented**. A
flush confirms only that the NATS server received the messages. Requiring
end-to-end storage verification is outside this pattern.

Second distinct PR: `#220` `discussion_r3598000902` — a restore wrote without a
tombstoned precondition; fixed in `e933d4b` plus `77c4612` (revision-gated
verify).

**Guards that satisfy it:** a combined `failed`/`skipped` zero check before the
destructive step, plus `nc.FlushTimeout`/`nc.Flush`/`nc.Drain` when the producer
used core NATS; or a revision-gated precondition on the write.

---

## `scripts-false-success-exit-and-unvalidated-bounds`

**Rule:** A `scripts/**` entrypoint must not report success for work it did not
do: a numeric bound that can disable the work loop must be validated, and the
exit path must account for skipped records, not only failed ones.

**Severity:** `high`

**Detect:** Either (a) a numeric CLI flag bounding a worker pool, page size or
batch size is used without a `< 1` / `<= 0` guard before the work loop; or (b)
the terminal exit path returns 0 while a `skipped` or `notFound` counter is
non-zero and only `failed` is checked.

**Evidence — two distinct PRs, both arms fixed:**

- `#218` (`discussion_r3573751709`, `discussion_r3574076352`): `-workers=0`
  starts no goroutines and exits 0. Fixed in `f6c341e`:
  `+ if *workers < 1 { slog.Error("workers must be >= 1", …); os.Exit(1) }` —
  live at `scripts/backfill_participant_mappings/main.go:107`.
- `#224` (`discussion_r3647155852`): *"OpenSearch accepts `size: 0`, so
  `-update -delete -page-size 0` returns a positive total with no hits; the loop
  publishes nothing, records no failures, and then scrubs every source host
  key."* Fixed in `a093bd0`: `+ if *pageSize <= 0 { … os.Exit(1) }` — live at
  line 123.

Note how arm (a) compounds arm (b) and the destructive pattern above: a
zero bound produces no failures, which satisfies a `failed`-only exit check,
which then permits the scrub.

**Guards that satisfy it:** an explicit bounds guard before the work loop, and an
exit path whose success condition includes the `skipped`/`notFound` counters.

**Live anchors** (context, not work items): two scripts on `origin/main` still
exit 0 with `skipped > 0`, checking only `failed` —
`scripts/reindex_meetings/main.go:208-211` and
`scripts/backfill_meeting_host_credentials/main.go:230-233`. Copilot raised
exactly this in `discussion_r3659460671`, **ten minutes after the PR merged**, so
it was never actioned.

**Why it is not tooling's job:** the form is generic, but the `scripts/**`
scoping, the three-script recurrence, and the destructive downstream step are
what make it a real pattern here rather than a lint rule.
