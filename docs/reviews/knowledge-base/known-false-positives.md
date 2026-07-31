<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Known false positives — the floor

**Apply this file last, after every other match.** These are not "lower
priority" — they are findings this repository's own history shows to be wrong,
already enforced elsewhere, or reliably ignored.

**This file is one revision's classification, not the suppression decision.** A
reviewer reads it at the supplied base *and* target revisions and drops a
candidate only when **both** waive it. Coverage present at only one revision does
not suppress — that is what stops a change from waiving a finding about itself,
and what keeps a withdrawn waiver from still suppressing. An entry here means
"this revision waives it", not "this candidate is dropped".

Each entry records why, so the decision can be re-audited rather than re-argued.

---

## Already enforced by tooling

### Missing license header on a new Go file — never a finding

Enforced three times over: the blocking
`.github/workflows/license-header-check.yaml`, `Makefile:151 license-check`
(invoked by `make check`), and `scripts/hooks/pre-commit`. Raised in `#216`
(`discussion_r3555676139`) and fixed in `8d7dfb4`, but the PR-side
`copilot-code-reviewer` skill already names it a non-finding verbatim. Pure
noise.

### Outdated third-party dependency versions — age alone is never a finding

Dependabot owns routine version currency, and `govulncheck` runs in CI, so
*"this dependency is behind latest"* is noise here.

**This waives staleness only.** It does not waive a change that introduces a
dependency version with a known vulnerability, or one that is unsupported or
end-of-life. Those are findings on their own terms — the guard is the absence of
a *newly introduced* problem, not the presence of Dependabot.

---

## Rejected on outcome — the team will not pay for these

### Generic *add tests for this* — never a finding

The single most important rejection here, because raw recurrence made it look
like the strongest pattern in the sample.

On PR `#218` it was raised **five times** (`discussion_r3572526003`,
`r3572632317`, `r3572666407`, `r3572717279`, `r3572975914`). The developer added
roughly 1,000 lines of tests (`43c95e6`, `09b2b66`) and then **deleted them
before merge** in `03a32cd9`:

> "Tests added significant review overhead (~1k lines) for a small bug fix.
> Removing to keep the PR focused."

On `origin/main` there is no
`cmd/meeting-api/eventing/participant_event_handler_test.go`, and
`cmd/meeting-api/eventing/registrant_event_handler_test.go` carries only two
update cases.

**The promotion gate is durable change, not recurrence.** High recurrence with a
reverted outcome means the finding class costs more than this team will pay;
promoting it manufactures a reviewer that gets skimmed and ignored.

This does **not** mean tests never matter. A narrowly scoped test finding tied to
a named contract or security consequence is still legitimate — but it belongs to
the sibling `repo_code` reviewer under `CLAUDE.md`'s testing guidance, not here.

### Unrelated dependency bundling in a scoped PR — never a finding

The worst outcome record in the sample. Raised on `#222`
(`discussion_r3640551641`), `#226` (`r3659463059`) and `#143`
(`r3075813933`). `#222` was **argued down and merged with the dependency bumps
intact** (`golang.org/x/sync v0.21.0` and friends are on `main`); `#226`'s
dependencies dissolved via a rebase rather than a split; `#143`'s comment was
posted 26 seconds *after* the force-push that already fixed it. Three of the four
were driven by CI `govulncheck` failing, so a pattern here would fight the build
gate.

---

## Covered elsewhere — not this reviewer's lane

### Generic in-file *doc comment contradicts adjacent code* — never a finding here

Strong evidence (13 comments across 6 PRs, 12 fixed), but it restates the
PR-side `meeting-service-code-review` skill's **Code truthfulness** section
almost verbatim and has no mechanical detect beyond *read the comment*. Only the
cross-file half survives, and it lives in
[`contract-and-config-drift.md`](contract-and-config-drift.md).

### Textbook Go gotchas with no repo-specific content — never a finding here

For example the deferred stale scroll ID (`#224` `discussion_r3646862112` →
`caf3897`), which recurred twice. It belongs to the central `general` reviewer.

*(Provenance note for maintainers: that comment's claim that the issue "also
appears on line 238" was a reviewer hallucination — line 238 there is the NATS
publish loop. Verify a cited line before trusting a mined comment.)*

### Generic script hygiene, single occurrence — never a finding

Pin `requirements.txt` (`#220` `discussion_r3615563615`), reject non-finite
argparse floats (`r3597959536`), wrap AWS client construction (`r3600991394`),
defensively type malformed JSON (`r3598219226`), throttle progress logging
(`#218` `r3574076391`). Each was fixed once, none needs repo knowledge, and all
are better addressed by adding `ruff`/`mypy`/`bandit` and a `pytest` job than by
a reviewer pattern.

---

## Contradicted by the merged contract

### `HistoryCheckRelation: "auditor"` should be `host` — never a finding

Raised twice on `#224` (`discussion_r3647155804`, `r3647155834`) with **no fix**,
which already fails the evidence rule. More importantly it **contradicts the
merged contract**: `auditor` is the value on all 10+ publishers in
`internal/infrastructure/eventing/nats_publisher.go` and matches
`docs/indexer-contract.md`. Promoting it would manufacture false positives on
correct code.

The underlying authorization question — whether an auditor reading a prior
version of a host-credentials document defeats the host-only boundary — is a real
open question about the indexer contract, deliberately **quarantined** and routed
to humans. It is not KB material and this reviewer must not raise it.

---

## Deliberate placeholders

### Obvious test credentials — never a finding

This repo's tests intentionally use transparent placeholders such as
`"test-user-token"` and `[]byte("test-secret")`. Judge whether a value would
actually authenticate somewhere, not whether the variable is named like a token.
A *real* credential in a diff remains a finding anywhere it appears.

---

## Other quarantined questions (not KB material)

Raised in review, never resolved, and explicitly out of scope for this reviewer:

- **Backfill CAS versus the source object** (`#218` `discussion_r3574683177`, no
  fix): the mapping-revision CAS does not protect the v1 object read that
  precedes it. The remedy is operational — quiesce event processing — and is
  documented nowhere.
- **Live documentation drift on `origin/main`**: `OTEL_TRACES_SAMPLE_RATIO` in
  `README.md:458` and `docs/tracing.md:15,124,127,130` (read by no code since
  `#206`), and the stale `member_remove` scope in `docs/fga-contract.md:28` plus
  its Triggers table. These are recorded as live anchors in the relevant
  category files for context; fixing them is a separate ticket, and a patch that
  does not touch them must never be faulted for them.
