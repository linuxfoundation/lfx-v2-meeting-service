# reconcile_meeting_registrants

Reconciles one meeting's indexed registrants against the authoritative DynamoDB
registrant table. It repairs rows that are absent from DynamoDB but remain live in
the `v1-objects` NATS KV bucket and OpenSearch.

Dry-run is the default. Apply writes only `_sdc_deleted_at` to exact NATS keys
using revision-conditional updates. Existing meeting-service event processing
then removes the corresponding OpenSearch document and applicable OpenFGA tuple.

## Scope and safety

- OpenSearch supplies the bounded candidate UID list for one meeting.
- Consistent DynamoDB `BatchGetItem` reads determine active versus stale.
- The tool never scans DynamoDB or NATS.
- The tool never writes DynamoDB, OpenSearch, OpenFGA, or ITX.
- A present DynamoDB row is always active; undocumented status fields are ignored.
- Apply requires a reviewed plan and exact environment/count confirmations.
- JSON and MessagePack NATS values retain their original encoding and fields.

The configured table must use one string partition key whose value is the
registrant UID. Any other key schema is rejected rather than guessed.

## Prerequisites

1. Python 3.11 or newer.
2. Reachable OpenSearch and NATS endpoints, normally through separately managed
   read/write access paths or port-forwards. The tool does not invoke `kubectl`.
3. AWS credentials from the standard credential chain, `--aws-profile`, or
   `--assume-role-arn`.
4. Least-privilege AWS permissions:
   - `sts:GetCallerIdentity`
   - `sts:AssumeRole` only when `--assume-role-arn` is used
   - `dynamodb:DescribeTable`
   - `dynamodb:BatchGetItem`
5. NATS read access to `v1-objects` and `v1-mappings`; apply/restore also require
   revision-conditional update access to `v1-objects`.

Install dependencies in an isolated environment:

```bash
cd scripts/reconcile_meeting_registrants
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
```

## Dry-run

```bash
.venv/bin/python reconcile_meeting_registrants.py \
  --meeting-id 98727043273 \
  --opensearch-url http://localhost:9200 \
  --nats-url nats://localhost:4222 \
  --dynamodb-table itx-zoom-meetings-registrants-v2 \
  --aws-region us-east-1 \
  --aws-profile production-readonly \
  --plan ./meeting-98727043273-plan.json
```

Dry-run creates a mode-`0600` plan and prints a redacted summary. Review:

- AWS account, region, table ARN, and discovered key schema
- sanitized OpenSearch URL/index and NATS URL identities
- every candidate UID and DynamoDB classification
- NATS revisions, mapping states, encodings, and payload digests
- active, stale, and candidate totals

The plan contains no raw DynamoDB/NATS values, credentials, names, email
addresses, or usernames.

## Apply

Use the exact account, table ARN, meeting ID, and stale count from the reviewed
plan:

```bash
.venv/bin/python reconcile_meeting_registrants.py \
  --meeting-id 98727043273 \
  --opensearch-url http://localhost:9200 \
  --nats-url nats://localhost:4222 \
  --dynamodb-table itx-zoom-meetings-registrants-v2 \
  --aws-region us-east-1 \
  --aws-profile production-readonly \
  --plan ./meeting-98727043273-plan.json \
  --apply \
  --confirm-meeting-id 98727043273 \
  --confirm-aws-account 123456789012 \
  --confirm-table-arn arn:aws:dynamodb:us-east-1:123456789012:table/example \
  --confirm-stale-count 8
```

Before the first write, apply repeats candidate discovery, consistent DynamoDB
classification, and all NATS reads. Any changed UID, classification, digest,
mapping, revision, AWS environment, or OpenSearch/NATS target invalidates the
plan. After writes, the tool waits within the configured verification deadline
for stale UIDs to leave OpenSearch and their mappings to become tombstoned.

## Restore

Restore is an optional emergency rollback, not part of the normal reconciliation
workflow. Use it only to reverse an incorrect soft-delete after confirming that
DynamoDB again contains the exact registrant; the standard workflow ends after
dry-run, apply, and downstream verification.

Restore one UID only after DynamoDB contains that exact row:

```bash
.venv/bin/python reconcile_meeting_registrants.py \
  --meeting-id 98727043273 \
  --opensearch-url http://localhost:9200 \
  --nats-url nats://localhost:4222 \
  --dynamodb-table itx-zoom-meetings-registrants-v2 \
  --aws-region us-east-1 \
  --aws-profile production-readonly \
  --restore <registrant-uid> \
  --confirm-meeting-id 98727043273 \
  --confirm-aws-account 123456789012 \
  --confirm-table-arn arn:aws:dynamodb:us-east-1:123456789012:table/example
```

Restore requires an existing tombstoned mapping, removes only
`_sdc_deleted_at` with an expected revision, then waits for the same UID to
reappear in OpenSearch with a newer live mapping revision.

## Failure recovery

- Authentication, table-schema, pagination, decode, mapping, or incomplete
  DynamoDB-read failures stop before writes.
- Candidate, authority, digest, mapping, or revision drift requires a new
  dry-run plan.
- Partial apply stops on the first failure and reports completed and pending
  UIDs. Do not reuse the old plan; create and review a new dry-run plan.
- Verification timeout issues no compensating writes. Inspect the existing event
  pipeline before retrying.
- An already marked stale row is not rewritten, but remains included in
  downstream verification.

The command exits non-zero for every ambiguous, partial, or unverified result.
