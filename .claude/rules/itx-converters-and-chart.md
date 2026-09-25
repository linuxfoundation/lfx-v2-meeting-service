---
paths:
  - "cmd/meeting-api/service/itx_*_converters.go"
  - "cmd/meeting-api/service/itx_*_converters_test.go"
  - "pkg/models/itx/**"
  - "pkg/utils/ptr.go"
  - "charts/lfx-v2-meeting-service/**"
  - "cmd/meeting-api/config.go"
---
<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# ITX converters, pointer helpers and the chart

Repo-specific calibrations for changes to the ITX converters, the ITX models,
the pointer helpers and the Helm chart. They refine the rules in `CLAUDE.md`
(`### Pointer Conversion Helpers`, `## Environment Variables`, `### Testing Strategy`)
and apply to reviewers and authors alike.

## Pointer helpers: the real names

The only pointer helpers are in `pkg/utils/ptr.go`: `utils.StringPtrOmitEmpty`,
`utils.IntPtrOmitZero`, `utils.Int64PtrOmitZero`, `utils.BoolPtr` and
`utils.BoolPtrOmitFalse`. **No `ptrIfNotZero` / `ptrIfNotEmpty` / `ptrIfTrue`
symbol exists in this repo**; never write, or ask for, code that uses one.

| Helper | Semantics |
|---|---|
| `utils.StringPtrOmitEmpty` | pointer unless the string is empty |
| `utils.IntPtrOmitZero`, `utils.Int64PtrOmitZero` | pointer unless the value is zero |
| `utils.BoolPtr` | **always** a pointer, so a deliberate `false` survives |
| `utils.BoolPtrOmitFalse` | pointer only when true |

## Judge the two conversion directions separately

**ITX → Goa (response conversion).** This is where the helpers are used —
inside `ConvertITX…ToGoa` functions. Choose an always-present pointer
(`utils.BoolPtr`) or an omit-zero one (`utils.BoolPtrOmitFalse`,
`utils.StringPtrOmitEmpty`, `utils.IntPtrOmitZero`) according to what the
proxy response contract (`docs/api-contracts/itx-*.md`) promises the client.
Taking the address directly (`&resp.Field`) carries **always-present**
semantics and is correct only where the contract wants that; for a string or
an int it is the only always-present form, because `BoolPtr` is the sole
always-present helper and takes a `bool`. Address-taking is never equivalent
to an omit-zero helper: `BoolPtrOmitFalse(false)`, `StringPtrOmitEmpty("")`
and `IntPtrOmitZero(0)` are all `nil`, so `&resp.Flag` emits a `false` the
contract may not want emitted. Judge each field against its contract; do not
treat address-taking as always fine or always wrong.

**Goa → ITX (outbound serialization).** The loss condition lives in the **ITX
model field type and JSON tag**, not in a helper. A non-pointer field with
`omitempty` — for example `Host bool \`json:"host,omitempty"\`` in
`pkg/models/itx/meeting_registrants.go` — silently drops a deliberate `false`,
and no helper fixes that. When an explicit zero, empty string or `false` must
reach ITX, the fix is a **pointer field in the ITX model** (as `Approved *bool`
in `pkg/models/itx/past_meeting_summaries.go` already is). Dereferencing an
optional Goa value into a non-pointer `omitempty` field is the outbound defect
to look for.

## Environment variables: code, chart and docs

`CLAUDE.md` `## Environment Variables` and `README.md` document the variables
the service reads; `charts/lfx-v2-meeting-service/` is how a cluster sets them.
Two **directed** checks: a chart that renders a name no service code reads, or
code reading a name that has **neither a dedicated chart knob nor
documentation**. A code-only variable that *is* documented is fine.

Do not claim the chart "cannot set" a variable: `values.yaml` exposes
`app.extraEnv` as an arbitrary list and `templates/deployment.yaml` injects it
verbatim, so any name can be set through the chart. The defect is a missing
first-class chart setting or missing documentation, not an impossibility. Some
variables are deliberately absent from the chart because
`cmd/meeting-api/config.go` derives their defaults; check there before calling
an unplumbed variable a problem.

## Tests on converters and service logic

A missing test is a finding **only** when it leaves a nameable contract or
security consequence unguarded — a converter mapping an unset value, an
occurrence calculation, a KV routing decision, a retry classification. A
generic "add tests for this" is not a finding; this repo has removed such tests
as review overhead before.
