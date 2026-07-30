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

**Severity:** `high`

**Detect:** Three OR'd arms, each a same-diff co-change check. (1) The diff
touches `member_put`/`member_remove`/`update_access`/`delete_access` publication
in `cmd/meeting-api/eventing/**` or `internal/infrastructure/eventing/**` and has
no `docs/fga-contract.md` hunk. (2) The diff changes an indexer object, subject,
schema or access-check relation (`internal/domain/models/event_models.go`,
`internal/infrastructure/eventing/nats_publisher.go`) and has no
`docs/indexer-contract.md` hunk and no `docs/event-processing.md` payload-example
hunk. (3) The diff changes a client-visible or ITX-outbound shape (`design/*.go`,
`pkg/models/itx/**`, field injection in `internal/service/itx/**`) and has no
matching `docs/api-contracts/itx-*.md` or `docs/itx-proxy-implementation.md`
hunk. A fourth arm fires when the doc *is* updated in the diff but only in a
detail section, leaving its own summary/subject table or Triggers table
describing the old behaviour.

**Evidence:** Raised 10× across three distinct PRs — `#218`, `#224`, `#226` —
and fixed every time.

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

**Rule:** An environment variable is a three-arm contract across the Helm chart,
the Go code that reads it, and the documentation that names it; a change to one
arm without the others silently breaks configuration that still appears to work.

**Severity:** `high`

**Detect:** Fully scriptable. Extract `- name: [A-Z0-9_]+` from
`charts/**/templates/`, `os.Getenv\("([A-Z0-9_]+)"\)` from `**/*.go`, and
env-var names from `README.md` and `docs/`; flag a non-empty symmetric
difference introduced by the diff. Before reporting, check
`cmd/meeting-api/config.go` — some variables are deliberately absent from the
chart because their defaults are derived there.

**Severity note:** the failure is silent. In the originating case Helm-based
sampling configuration stopped working while both the chart and the code
remained internally valid.

**Evidence:** `#206` comment `discussion_r3507937313`, on
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
