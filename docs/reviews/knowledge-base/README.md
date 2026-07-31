<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Empirical review knowledge base — lfx-v2-meeting-service

Patterns extracted from **real GitHub Copilot review comments on this
repository that a developer actually fixed**. This is the rulebook for the
`repo_learnings` role of this repo's local pre-PR review; the brain that loads it is
[`.claude/skills/meeting-service-learnings-reviewer/SKILL.md`](../../../.claude/skills/meeting-service-learnings-reviewer/SKILL.md),
which carries the review method and points here. This directory is the single
copy of the empirical KB — the skill does not duplicate it.

It is deliberately *not* a general review checklist. Correctness, security and
test quality in the abstract belong to the central `general` reviewer, and this
repo's written conventions and contracts belong to the sibling
`meeting-service-code-reviewer`. What lives here is only what this repo's own
review history proves, mechanically, about itself.

## Evidence window

Mined from PRs created or merged on or after **2026-06-29** — `#205`–`#228`
plus `#143` — against `origin/main` at **`4ce62f6`**. 142 Copilot inline
comments across 14 PRs (`#206`, `#208`, `#210`, `#212`, `#216`, `#217`, `#218`,
`#220`, `#221`, `#222`, `#224`, `#225`, `#226`, `#143`) were read in full.

Every quote, count and line anchor in these entries was **re-verified against
`origin/main` at `4bb31d0`** (after the `#229`/`#230` dead-code cleanups), and
all of them hold. When re-auditing, re-derive the figures rather than trusting
them — several figures here were wrong on first authoring precisely because they
were inherited rather than re-counted.

### Corrections on record

Kept visible so the next maintainer can see what was wrong and how it was found,
rather than re-litigating it:

- `recordAndMapHTTPError` call sites: **33 → 42**. The 33 was inherited and was
  never true at any commit; 42 has held since `dc70a88`. Found by this repo's own
  local review cycle reviewing the commit that introduced this KB.
- `reconcile_meeting_registrants`: **"a 1,497-line tool with 69 tests" → 1,376
  lines of tool code and 72 tests**. The 1,497 figure matches no commit in that
  file's history.
- `.gitleaks.toml`: **"a four-line allowlist" → no detection rules at all**, one
  `[allowlist]`, one `paths` entry, in a 9-line file. The substance was always
  right; only the descriptor was wrong.

**A failed check can mean the probe is wrong, not the claim.** The test count
above first came back as 32 from `grep -c '^def test_\|    def test_'`, which
silently missed 40 `async def test_` definitions. Correcting the KB down to 32
would have destroyed a true claim on the strength of a broken one-liner. Inspect
the source and validate the extractor before revising or removing any entry —
and never delete a pattern solely because a first-pass script returned zero.

Reviewer identity was verified rather than assumed: inline comments by login
`Copilot` (user id `175728472`, type `Bot`, app
`copilot-pull-request-reviewer`). `coderabbitai[bot]`,
`github-license-compliance[bot]` and human reviewers were excluded.

## The promotion gate

A candidate becomes an entry here only when all of these hold:

1. **A developer fixed it, on a merged PR.** A finding that was argued down,
   reverted, or silently ignored is rejection evidence, not a pattern. Recurrence
   alone is not enough — check the present-day state of `main` and look for a
   revert before crediting a fix.

   **This gate applies to every count in this KB, including any produced by a
   widened or re-run probe.** Broadening a search does not relax it: matches on
   open or unmerged PRs, and matches whose fix never landed, are never counted
   toward promotion. Where an entry has more than one detect tier, publish a
   separate reproducible count per tier and say which gate each passed —
   never one total across an unstated boundary.
2. **It is mechanically detectable.** The entry must state a `detect` condition
   a reviewer can evaluate against a diff, not a theme it can nod along to.
3. **No tooling already catches it.** This repo has no `.golangci.yml`, so
   `golangci-lint` runs defaults only (errcheck, gosimple, govet, ineffassign,
   staticcheck, unused) with no `gosec`; MegaLinter runs the **Go flavor**, so
   Python under `scripts/**` is linted and tested by nothing; `make verify`
   checks `design/`↔`gen/` drift only; `.gitleaks.toml` carries **no detection
   rules at all** — a single `[allowlist]` whose only entry is
   `paths = ["^gen/.*"]` — so it matches committed literals, never runtime
   dataflow. A pattern a linter or CI job already blocks would be noise.
4. **It adds something the PR-side review skills do not already state.**
   `.github/skills/**` covers contract drift, PII exposure, the retry/ack
   contract and code truthfulness qualitatively. An entry earns its place by
   adding a mechanical detect condition, a named guard helper, an uncovered
   surface, or a live anchor — not by restating a theme.

Two consequences worth stating plainly, because both are counter-intuitive:

- **`scripts/**` is this repo's uncovered high-risk surface.** Neither
  `.github/skills` file mentions it, its Python has no CI at all, and the
  operational scripts there do irreversible production work. Two of the eight
  entries here are scoped to it.
- **A repeatedly-raised finding can be evidence *against* a pattern.** See
  [`known-false-positives.md`](known-false-positives.md).

## Entry format

Each entry carries:

- **Pattern id** — the stable slug a finding names when it cites this entry.
- **Rule** — a single quotable sentence. This is what a finding quotes verbatim
  when citing the entry.
- **Severity** — the ceiling for a finding on this pattern, `Critical` or
  `Important`.
- **Detect** — the condition evaluated against the change under review, which a
  finding restates verbatim when citing the entry.
- **Evidence** — the originating comment, the fixing commit, and the state of
  `main` today.
- **Guards that satisfy it** — the helper or shape whose presence means the
  pattern does not fire.
- **Live anchors** — where the shape exists on `main` today. These are context
  that keeps the pattern honest, **not** work items: a pre-existing instance the
  change under review does not touch is never a finding.

## Maintaining this KB

Add an entry only from mined evidence that clears the gate above, and record the
comment, the fixing commit, and the present-day state so the next maintainer can
re-audit it. Remove an entry when its evidence stops holding — a repo where the
tooling grows should shed patterns, and entries retired by new linting are a
success, not a loss. KB removals are human-gated: they change what the reviewer
will and will not raise.
