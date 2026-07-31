<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# Contract and configuration drift

Two patterns about a change landing in one arm of a multi-arm contract and not
the others. Both were fixed repeatedly by developers here, and neither is
detectable by any tool this repo runs.

---

## `contract-doc-not-updated-with-message-or-api-shape-change`

**Rule:** A change to what this service publishes or exposes — an indexer or
FGA message field, subject, access-check relation, indexed object, or an ITX
request/response shape — must update the matching contract document in the same
diff, and must update that document's summary and Triggers tables, not only its
detail section.

**Severity:** `Important`

**Detect — core arms (1–3), OR'd**, each a same-diff co-change check. (1) The diff
touches `member_put`/`member_remove`/`update_access`/`delete_access` publication
in `cmd/meeting-api/eventing/**` or `internal/infrastructure/eventing/**` and has
no `docs/fga-contract.md` hunk. (2) The diff changes an indexer object, subject,
schema or access-check relation (`internal/domain/models/event_models.go`,
`internal/infrastructure/eventing/nats_publisher.go`) and has no
`docs/indexer-contract.md` hunk. A `docs/event-processing.md` payload-example
hunk does **not** substitute for it — the rule names the contract document, and
the evidenced fix (`caf3897`) updated `docs/indexer-contract.md`. When the change
also alters a documented payload example, both files must be updated.
(3) The diff changes a client-visible or ITX-outbound shape and has no matching
`docs/api-contracts/itx-*.md` or `docs/itx-proxy-implementation.md` hunk.
**Define this structurally, not by a path list**: it fires wherever an
`itx.*Request` literal is constructed or populated, an `itx.*` model field is
assigned, or a client-visible response is built — because a change that starts or
stops populating an already-defined field alters the wire contract regardless of
which file it sits in. Known sites, as **examples not an exhaustive set**:
`design/*.go`, `pkg/models/itx/**`, `internal/service/itx/**`,
`cmd/meeting-api/service/itx_*_converters.go`, and API handlers that build
requests inline such as `cmd/meeting-api/api_itx_meetings.go`. A path-list
detector will keep going stale as new construction sites appear.

**Detect — variant arm (4), dependent:** shares the **trigger** of arms 1–3 — a
diff that changes published or exposed shape — and covers the case those arms do
not, because the matching contract doc **was** touched: it fires when that doc
was updated only in a detail section, leaving its own summary/subject table or
Triggers table describing the old behaviour. It is never a standalone detector —
a diff that changes no published shape does not reach it, so a docs-only diff
cannot fire it.

The dependency is on that shared trigger, **not** on a core arm having produced a
finding: arms 1–3 fire only when the contract doc is *absent* from the diff,
which is the exact opposite of this arm's precondition, so the two are mutually
exclusive by construction.

The scoping is deliberate. The arm has no developer-fixed evidence of its own
(see the counts below), and this KB's promotion gate does not admit an
unevidenced standalone detector. Binding it to the same trigger keeps every
finding it produces anchored to the well-evidenced pattern it refines.

**Evidence — counts are reported per tier, and both apply the same eligibility
gate: only findings on merged PRs that a developer actually fixed are counted.**

- **Core arms (1–3): 10 qualifying findings across three distinct PRs** —
  `#218`, `#224`, `#226` — every one fixed and the fix still present on
  `origin/main`. Reproduce by listing Copilot inline comments on those PRs whose
  fixing commit touches a `docs/` contract file.
- **Variant arm (4): 0 qualifying findings.** It carries **no recurrence
  evidence at all** and is not part of the count above. It rests solely on the
  live anchor below — a partial update visible on `origin/main` today. Treat it
  as the weaker arm accordingly: it earns a finding only when the diff plainly
  leaves a summary or Triggers table contradicting the detail section it just
  changed.

- `#224` comment `discussion_r3646862161`, on
  `internal/domain/models/event_models.go:426`: *"The repository's indexer
  contract is now stale: `docs/indexer-contract.md:59` still advertises
  `host_key` on `v1_meeting`, and it does not document this new object, subject,
  schema, or `host` access check. Update that contract in this PR so consumers do
  not continue relying on the exposed field."* Fixed in `caf3897` — the
  `host_key` row was removed from `v1_meeting` and a `## V1 Meeting Host
  Credentials` section added with subject `lfx.index.v1_meeting_host_credentials`
  and `access_check_relation: host`. On `main` today `host_key` appears in
  `docs/indexer-contract.md` only inside that new section.
- `#218` `discussion_r3572632336` → `43c95e6` rewrote the `fga-contract`
  `member_remove` section.
- `#226` `discussion_r3659606053` → `685ec40` added `## Audit stamping` to 9 of
  the 10 `docs/api-contracts/*.md` files.

**Guards that satisfy it:** a hunk in the matching contract doc in the same
diff, covering both the detail section and the summary/Triggers tables.

**Why it is not tooling's job:** `make verify` asserts `design/`↔`gen/` drift
only. Nothing in CI compares code to a hand-written contract doc.

**Live anchors** (context, not work items): `docs/fga-contract.md:28` still reads
*"sent on registrant delete and on full participant deletes"* although
`member_remove` is now also emitted on registrant update and on participant
username change; the Triggers table has a row for participant username change
(line 195) but none for registrant update. This is exactly the partial-update
failure mode the fourth detect arm catches.

---

## `env-var-contract-split-across-chart-code-docs`

**Rule:** An environment variable is a contract between the Helm chart and the Go
code that reads it; a diff that changes one arm without the other leaves either a
chart rendering a name no code reads, or a variable the service depends on with
no first-class chart setting and no documentation.

This sentence deliberately states only the chart↔code arm, because it is the
text a finding quotes verbatim and it must assert nothing the detect condition
below does not test. The documentation obligation is real but conditional, so it
lives in the detect and guard prose rather than in the quoted rule — do not
"restore" it here.

**Severity:** `Important` — for the `chart − code` direction because dead
configuration surface misleads operators, and for the `code − chart` direction
because a variable reachable only through `app.extraEnv` is undiscoverable to
anyone reading the chart or the docs. Neither direction is a runtime break;
`app.extraEnv` keeps every name settable. Do not raise either as breakage.

**Detect — chart↔code arm (the evidenced one).** Fully scriptable. Build the
chart-side set from **both** places the chart names variables, then compare it
against `os.Getenv\("([A-Z0-9_]+)"\)` extracted from the **service** Go code
only — `cmd/**`, `internal/**` and `pkg/**`, excluding `*_test.go`. Do not scan
`**/*.go`: `internal/logging/logging_test.go` reads `LOG_LEVEL`/`LOG_ADD_SOURCE`
to save and restore them, and the three standalone programs under `scripts/**`
read their own operator-supplied variables (`OPENSEARCH_URL`, `NATS_URL`) that
the service chart is not meant to render. Including either produces phantom
"code reads what the chart cannot set" findings.

1. Literals in the templates — `- name: [A-Z0-9_]+` under `charts/**/templates/`.
   At `9cc00c9` this yields only the 11 `OTEL_*` names.
2. **Keys under `app.environment` in `charts/**/values.yaml`**, which
   `deployment.yaml:41-42` renders through
   `{{- range $name, $config := .Values.app.environment }}` / `- name: {{ $name }}`.
   This is where `LOG_LEVEL`, `JWKS_URL`, `JWT_AUDIENCE`, `LFX_ENVIRONMENT` and
   the rest of the service's real configuration surface live.

   **`app.extraEnv` is not a key source and must not be extracted as one.** It
   defaults to `[]` and `deployment.yaml:50-52` injects it verbatim with
   `{{- toYaml . }}`. It is a **wildcard escape hatch**: an operator can set any
   variable through it without that name appearing anywhere in `values.yaml`.

A literals-only extractor misses essentially every variable documented in
`CLAUDE.md`'s Environment Variables section, so step 2 is not optional.

These are **two directed checks, not one symmetric difference** — they have
different conditions and must not be collapsed. Flag either, when **introduced by
the diff**:

- **`chartNames − codeNames`** — the chart renders a name no service code reads.
  Dead configuration surface.
- **`codeNames − dedicatedChartNames − documentedNames`** — code reads a name
  that has neither a first-class chart setting nor documentation. The
  documentation condition applies to **this direction only**; a code-only
  variable that *is* documented is not a finding.

**Never phrase the second half as "the chart cannot set it".** Because
`app.extraEnv` is a wildcard, that claim is false for every variable: any name
can be set through the chart. The defect is the missing first-class setting and
the missing documentation, which is a real maintainability problem — not an
impossibility. A finding that asserts impossibility is wrong on its face and
will be rejected.

Before reporting, check `cmd/meeting-api/config.go`: some variables are
deliberately absent from the chart because their defaults are derived there.

**Docs arm — not independently finding-bearing.** Documentation naming a
variable (`README.md`, `docs/`) is checked **only when the same diff already
matched the chart↔code arm for that variable**, in which case the stale doc is
part of the one finding. A doc that merely drifted from code is *not* a finding
on its own: the originating review raised exactly that
(`discussion_r3507937384`) and it was never actioned, so promoting it would
manufacture a reviewer this team has already shown it will not act on. Such
drift belongs in a ticket, and the live instance is recorded as an anchor below.

**Severity note:** the failure is silent. In the originating case Helm-based
sampling configuration stopped working while both the chart and the code
remained internally valid.

**Evidence — 1 qualifying finding (merged and developer-fixed), on `#206`.** The
same review's *docs* arm (`discussion_r3507937384`) was never actioned, so it is
**not** counted toward promotion; it appears below only as a live anchor. This
entry qualifies on the chart arm plus the cross-arm shape, not on a recurrence
total spanning fixed and unfixed matches.

`#206` comment `discussion_r3507937313`, on
`charts/lfx-v2-meeting-service/templates/deployment.yaml:92`: *"The chart now
renders OTEL_TRACES_SAMPLER/OTEL_TRACES_SAMPLER_ARG, but the application code
reads OTEL_TRACES_SAMPLE_RATIO and configures the sampler via
TraceIDRatioBased(cfg.TracesSampleRatio) (see pkg/utils/otel.go:112-126,
302-308). With this change, Helm-based sampling configuration will stop working
and tracing will default to 1.0 sampling."* Fixed in `135393e`
*"fix(otel): replace TracesSampleRatio with sampler env vars"* — the developer
moved the **code** to the chart's contract. Live on
`origin/main:pkg/utils/otel.go:116-117`.

**Guards that satisfy it:** the diff updates every arm the variable appears in,
or the variable's default is derived in `cmd/meeting-api/config.go` and the
chart's absence is intentional.

**Why it is not tooling's job:** `helm lint` and `yamllint` do not know what the
binary reads, and `KUBERNETES_KUBECONFORM`/`KUBERNETES_KUBESCAPE` are in
`DISABLE_LINTERS`.

**Live anchors** (context, not work items): the docs arm of the same review
(`discussion_r3507937384`) was never actioned — `README.md:458` and
`docs/tracing.md:15,124,127,130` still document `OTEL_TRACES_SAMPLE_RATIO`,
which no code has read since `#206`. `charts/…/values.yaml:88` also
reintroduced the invented term `parent_sampler_arg` (`67c9361`) that `a6d07fc`
had removed at the reviewer's request.
