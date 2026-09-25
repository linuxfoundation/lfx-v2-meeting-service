---
paths:
  - "cmd/meeting-api/service/itx_*_converters.go"
  - "pkg/models/itx/**"
  - "charts/lfx-v2-meeting-service/**"
  - "cmd/meeting-api/config.go"
---
<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# ITX converters and the chart

Two judgement calls that the code and the docs do not spell out. They refine
`CLAUDE.md` (`### Pointer Conversion Helpers`, `## Environment Variables`) and
`docs/itx-proxy-implementation.md` (`### 3. Pointer Conversion Helpers`).

## Judge the two conversion directions separately

**ITX → Goa (response conversion).** Choose an always-present pointer
(`utils.BoolPtr`, or `&resp.Field` for a string or an int — `BoolPtr` is the
only always-present helper) or an omit-zero one (`utils.BoolPtrOmitFalse`,
`utils.StringPtrOmitEmpty`, `utils.IntPtrOmitZero`) according to what the
proxy response contract in `docs/api-contracts/itx-*.md` promises the client
for that field. Address-taking is never equivalent to an omit-zero helper: the
omit-zero helpers return `nil` at the zero value, so `&resp.Flag` emits a
`false` the contract may want omitted. Judge each field against its contract;
address-taking is neither always fine nor always wrong.

**Goa → ITX (outbound serialization).** No helper applies. The loss condition
is the **ITX model field type and JSON tag**: a non-pointer field with
`omitempty` (for example `Host bool` in `pkg/models/itx/meeting_registrants.go`)
silently drops a deliberate `false`. When an explicit zero, empty string or
`false` must reach ITX, the fix is a pointer field in the ITX model (as
`Approved *bool` in `pkg/models/itx/past_meeting_summaries.go`), not a
converter change.

## Environment variables: what the chart can and cannot do

Never claim the chart "cannot set" a variable. `values.yaml` exposes
`app.extraEnv` as an arbitrary list and `templates/deployment.yaml` injects it
verbatim, so any name is settable through the chart. The defect, when there is
one, is a variable the code reads that has neither a first-class chart setting
nor documentation — a maintainability gap, not a runtime break. Before raising
it, check `cmd/meeting-api/config.go`: some variables are deliberately absent
from the chart because their defaults are derived there.
