---
paths:
  - "cmd/meeting-api/service/itx_*_converters.go"
  - "pkg/models/itx/**"
  - "internal/service/itx/**"
---
<!-- Copyright The Linux Foundation and each contributor to LFX. -->
<!-- SPDX-License-Identifier: MIT -->

# ITX converters

One judgement call that the code and the docs do not spell out. It refines
`CLAUDE.md` (`### Pointer Conversion Helpers`) and
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
`false` must reach ITX, the root cause is the ITX model field type, so the
fix starts there: make the field a pointer (as `Approved *bool` in
`pkg/models/itx/past_meeting_summaries.go`) and then update the converter or
service assignment to carry the payload's pointer through unchanged (as
`itx_past_meeting_summary_converters.go` does for `Approved`) instead of
dereferencing it with `utils.BoolValue` and the like. A helper-only or
converter-only fix cannot do it: the field type discards the value before it
is serialized.
