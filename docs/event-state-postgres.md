# Meeting-owned Postgres event state

The meeting API still proxies ITX business entities and preserves their IDs.
Project/committee translations still use the helper RPC. Only meeting event
processing state moves from the shared `v1-mappings` KV bucket into these tables:

| Table | Owned state |
| --- | --- |
| `meeting_resource_sync_state` | Processed, empty, deleted and legacy state; source UID, parent meeting ID, prior username and whether the principal is authoritative |
| `meeting_committee_associations` | Active/past meeting committee associations, keyed by source association IDs, with committee IDs and voting filters |
| `meeting_summary_associations` | Past meeting → summary source IDs, updated under CAS |
| `meeting_participant_lookup` | Invitee/attendee + past meeting + username → source participant ID, including deletion guards |
| `meeting_participant_aliases` | Historical invitee/attendee aliases and reverse lookup targets, preserved without allocating new IDs |
| `meeting_invitation_guards` | Registrant dispatch claim and returned invite ID |
| `meeting_event_state_cutover` | Verified offline migration checkpoint; required before runtime or repair starts |

This is not the helper's `v1_mappings` table. The adapter reconstructs the old
entry encoding at the event-handler boundary, but persists named domain columns
and structured association fields, not arbitrary KV keys and values. Historical
principal usernames are required by the existing FGA removal contract; no emails,
names, invite payloads, meeting passwords or business documents are copied.

## Configuration

`MEETING_STATE_DATABASE_URL` is required whenever meeting event processing is
enabled, for historical participant alias API reads, and for the participant repair/reindex tools. Load the DSN from an
approved Secret, with TLS verification for deployed databases. The Helm values
`eventStateDatabase.secretName` and `secretKey` wire an existing Secret's single
DSN key; alternatively use `app.environment.MEETING_STATE_DATABASE_URL.valueFrom`.
Do not use both sources or commit credentials. Database provisioning and live
environment values are separate operational work.

Each database operation has a five-second cap; each pool has at most ten
connections. Event processing and lazily opened participant API lookups use
separate pools; size database capacity for both pools across all replicas. The migration
schema transaction has a thirty-second cap and an advisory lock. Runtime does
not create an empty schema or fall back to KV when configuration, connectivity,
or cutover verification is missing. Source data still comes from `v1-objects`.

## Offline migration and deployment ordering

1. Provision a meeting-owned database/schema and Secret. Give the migration role
   schema/sequence permissions; the runtime role needs only table DML, sequence
   usage and read access to the cutover checkpoint. Restrict checkpoint writes
   and DDL to the migration role.
2. Stop and drain **all writers to the legacy mappings bucket**, including other
   wrappers, compatibility helper modes, and repair jobs. An elapsed quiet period
   is not a writer fence. Also stop the meeting consumer and maintenance tools.
   Retain source replication and its durable stream; check retention headroom.
3. Run `go run ./scripts/migrate_event_state` with `NATS_URL` and a Secret-loaded
   `MEETING_STATE_DATABASE_URL`. Dry-run validates owned key families and values,
   without applying schema or writing data. It uses request/reply sequence scans,
   not a bulk ephemeral consumer. It retains only owned current entries in memory;
   size the job for the measured meeting state population.
4. Review counts and exceptions. Live KV is the authority while legacy writers
   are fenced. Historical meeting rows copied into the helper's Postgres table
   are **not** silently used as the source or overwritten. Reconcile any intended
   differences or missing source history explicitly before proceeding.
5. Run the same command with `-apply`. It creates the schema, uses create-only
   imports, compares every imported entry semantically, rejects unmatched target
   rows, and requires unchanged source first/last sequence and message count.
   Application `!del` markers are retained; native DEL/PURGE remove entries from
   the source snapshot. Invalid owned data aborts; unrelated helper/service keys
   are not imported. Incomplete runs cannot enable runtime. A retry preserves
   agreeing partial rows and refuses conflicts rather than overwriting them.
6. Keep the writer fence in place through verification and deployment. Start the
   upgraded meeting service with the verified Postgres Secret. Restore other
   still-legacy writers only after the meeting migration is complete. No source
   deletion or writer configuration change is performed by the import command.
7. Verify dev first, then production: create/update/delete and restore events,
   committee changes, participant username changes and sibling merges, summary
   access refresh, recordings/attachments, and invitation replay. Confirm search
   documents and access tuples remain consistent. The PR does not itself perform
   these environment acceptance checks.

The importer refuses to run against a completed checkpoint because Postgres may
already contain newer state. Do not remove that checkpoint to force a rerun. A
source change, malformed state, or mismatched target requires investigation; the
command logs no sensitive values. Do not interpret a partial import as parity.

## Idempotency and rollback

PG revisions use a shared sequence, including across delete/recreate cycles.
Create-only invitation dispatch claims select a single worker before sending.
Lost responses, failed completion writes, and errors after dispatch retain a
pending claim: manual reconciliation is required, never automatic expiry or a
second send. This closes the former post-send marker race without claiming
exactly-once delivery from the remote invite service.

Before the upgraded consumer starts, rollback can discard the unused target
under explicit approval and resume the legacy version against unchanged KV.
After PG writes begin, KV is stale: **do not deploy the old image** without
stopping/draining processing and explicitly reconciling new PG state back to an
approved rollback store. There is no automatic dual-write or stale-read fallback.

`backfill_participant_mappings` now scans PG and CAS-updates PG revisions;
`reindex_meetings` reads PG participant cross-references while re-putting only
source `v1-objects` records. `reconcile_meeting_registrants` reads verified PG
state in read-only transactions and keeps source writes in NATS. It rejects old
KV-derived plans and binds new plans to the actual PG database/schema identity.
`migrate_event_state` is the sole explicit offline
legacy mappings reader in this repository. Remove historical KV/PG copies only
after verified owner cutovers and approved cleanup. Bucket retirement is gated
by the other services and helper/committee-tool dependencies in ops #1649.

## Local database tests

Set `MEETING_STATE_TEST_DATABASE_URL` to a **disposable local Postgres database**
and run `go test -race ./internal/infrastructure/eventstate/...`. Each database
test creates and drops only its own unique test schema. Without this variable,
database tests skip; codec tests and existing handler tests still run. The tests
exercise real SQL, CAS, delete/recreate safety, concurrent association writes and
invitation claims, plus synthetic stable/changed/conflicting migration sources.

Participant alias requests retain the mapper's configured timeout and any earlier
caller deadline. Lazy initialization is shared and bounded by that configured
timeout; waiters can cancel independently, and shutdown cancels/waits for pending
initialization before releasing its single pool. Migration message reads use the
leader-served API even when the source stream allows direct reads.
