import asyncio
import os
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import AsyncMock, Mock, patch

from common import NotFoundError
import reconcile_meeting_registrants as reconcile


class FakeResponse:
    def __init__(self, payload=None, status_code=200):
        self.payload = payload or {}
        self.status_code = status_code

    def raise_for_status(self):
        if self.status_code >= 400:
            raise RuntimeError(f"http status {self.status_code}")

    def json(self):
        return self.payload


class FakeHTTPSession:
    def __init__(self, responses):
        self.responses = list(responses)
        self.posts = []
        self.deletes = []

    def post(self, url, **kwargs):
        self.posts.append((url, kwargs))
        return self.responses.pop(0)

    def delete(self, url, **kwargs):
        self.deletes.append((url, kwargs))
        return FakeResponse()


class FakeDynamo:
    def __init__(self, responses):
        self.responses = list(responses)
        self.requests = []

    def batch_get_item(self, **kwargs):
        self.requests.append(kwargs)
        return self.responses.pop(0)


class Entry:
    def __init__(self, value, revision):
        self.value = value
        self.revision = revision


class FakeKV:
    def __init__(self, entries=None):
        self.entries = dict(entries or {})
        self.updates = []

    async def get(self, key):
        if key not in self.entries:
            raise NotFoundError(key)
        return self.entries[key]

    async def update(self, key, value, last):
        current = self.entries.get(key)
        if current is None or current.revision != last:
            raise reconcile.RevisionConflictError(key)
        self.updates.append((key, value, last))
        self.entries[key] = Entry(value, last + 1)
        return last + 1


class FailingUpdateKV(FakeKV):
    def __init__(self, entries=None, fail_key=None):
        super().__init__(entries)
        self.fail_key = fail_key

    async def update(self, key, value, last):
        if key == self.fail_key:
            raise reconcile.RevisionConflictError(key)
        return await super().update(key, value, last)


class SequencedGetKV(FakeKV):
    def __init__(self, key, entries):
        super().__init__({key: entries[-1]})
        self.key = key
        self.sequence = list(entries)

    async def get(self, key):
        if key != self.key:
            return await super().get(key)
        if len(self.sequence) > 1:
            return self.sequence.pop(0)
        return self.sequence[0]


class FakeAuthority:
    def __init__(self, active):
        self.active = set(active)

    def classify(self, uids):
        return {
            uid: ("active" if uid in self.active else "stale")
            for uid in sorted(set(uids))
        }


class FakeDiscovery:
    def __init__(self, uids):
        self.uids = list(uids)

    def discover(self, meeting_id):
        return list(self.uids)

    def contains(self, meeting_id, uid):
        return uid in self.uids


def cli_args(*extra, meeting_id="987", table="registrants"):
    return [
        "--meeting-id",
        meeting_id,
        "--opensearch-url",
        "http://search",
        "--nats-url",
        "nats://localhost:4222",
        "--dynamodb-table",
        table,
        *extra,
    ]


ENVIRONMENT = reconcile.Environment("123", "us-east-1", "arn:table", "reg", "id", "S")


class ConfigTests(unittest.TestCase):
    def test_meeting_id_is_required(self):
        with self.assertRaises(SystemExit):
            reconcile.parse_args([])

    def test_dry_run_is_default(self):
        config = reconcile.parse_args(cli_args())
        self.assertEqual("dry-run", config.mode)

    def test_apply_and_restore_are_mutually_exclusive(self):
        with self.assertRaises(SystemExit):
            reconcile.parse_args(cli_args("--apply", "--restore", "uid-1"))

    def test_apply_requires_plan_and_confirmations(self):
        with self.assertRaises(SystemExit):
            reconcile.parse_args(cli_args("--apply"))

    def test_literal_credentials_are_not_cli_options(self):
        parser = reconcile.build_parser()
        option_strings = {
            option for action in parser._actions for option in action.option_strings
        }
        self.assertNotIn("--aws-access-key-id", option_strings)
        self.assertNotIn("--aws-secret-access-key", option_strings)
        self.assertNotIn("--aws-session-token", option_strings)


class OpenSearchTests(unittest.TestCase):
    def test_discovery_uses_exact_filters_and_deduplicates_pages(self):
        session = FakeHTTPSession(
            [
                FakeResponse(
                    {
                        "_scroll_id": "scroll-1",
                        "hits": {
                            "hits": [
                                {"_source": {"object_id": "uid-2"}},
                                {"_source": {"object_id": "uid-1"}},
                            ]
                        },
                    }
                ),
                FakeResponse(
                    {
                        "_scroll_id": "scroll-2",
                        "hits": {"hits": [{"_source": {"object_id": "uid-1"}}]},
                    }
                ),
                FakeResponse({"_scroll_id": "scroll-2", "hits": {"hits": []}}),
            ]
        )
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        self.assertEqual(["uid-1", "uid-2"], client.discover("meeting-1"))
        query = session.posts[0][1]["json"]["query"]["bool"]["filter"]
        self.assertEqual(
            [
                {"term": {"object_type": "v1_meeting_registrant"}},
                {"term": {"data.meeting_id": "meeting-1"}},
            ],
            query,
        )
        self.assertEqual(1, len(session.deletes))

    def test_malformed_hit_fails_closed_and_clears_scroll(self):
        session = FakeHTTPSession(
            [
                FakeResponse(
                    {
                        "_scroll_id": "scroll-1",
                        "hits": {"hits": [{"_source": {}}]},
                    }
                )
            ]
        )
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        with self.assertRaises(reconcile.ReconciliationError):
            client.discover("meeting-1")
        self.assertEqual(1, len(session.deletes))

    def test_contains_issues_one_scoped_query_without_scrolling(self):
        session = FakeHTTPSession(
            [FakeResponse({"hits": {"hits": [{"_source": {"object_id": "uid-1"}}]}})]
        )
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        self.assertTrue(client.contains("meeting-1", "uid-1"))
        self.assertEqual(1, len(session.posts))
        query = session.posts[0][1]["json"]["query"]["bool"]["filter"]
        self.assertEqual(
            [
                {"term": {"object_type": "v1_meeting_registrant"}},
                {"term": {"data.meeting_id": "meeting-1"}},
                {"term": {"object_id": "uid-1"}},
            ],
            query,
        )
        self.assertEqual(0, len(session.deletes))

    def test_contains_returns_false_without_a_hit(self):
        session = FakeHTTPSession([FakeResponse({"hits": {"hits": []}})])
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        self.assertFalse(client.contains("meeting-1", "uid-1"))

    def test_empty_result_returns_no_candidates(self):
        session = FakeHTTPSession(
            [FakeResponse({"_scroll_id": "scroll-1", "hits": {"hits": []}})]
        )
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        self.assertEqual([], client.discover("meeting-1"))

    def test_scroll_cleanup_failure_fails_closed(self):
        session = FakeHTTPSession(
            [FakeResponse({"_scroll_id": "scroll-1", "hits": {"hits": []}})]
        )
        session.delete = Mock(return_value=FakeResponse(status_code=500))
        client = reconcile.OpenSearchDiscovery(session, "http://search", "resources", 5)
        with self.assertRaises(reconcile.ReconciliationError):
            client.discover("meeting-1")


class DynamoDBTests(unittest.TestCase):
    def test_environment_discovers_identity_and_hash_key(self):
        sts = Mock()
        sts.get_caller_identity.return_value = {
            "Account": "123456789012",
            "Arn": "arn:aws:iam::123456789012:role/reconcile",
        }
        dynamo = Mock()
        dynamo.describe_table.return_value = {
            "Table": {
                "TableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/reg",
                "KeySchema": [{"AttributeName": "id", "KeyType": "HASH"}],
                "AttributeDefinitions": [{"AttributeName": "id", "AttributeType": "S"}],
            }
        }
        authority = reconcile.DynamoAuthority(
            sts, dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        environment = authority.discover_environment()
        self.assertEqual("123456789012", environment.account)
        self.assertEqual("id", environment.key_name)
        self.assertEqual({"id": {"S": "uid-1"}}, authority.build_key("uid-1"))

    def test_composite_key_is_rejected_without_scan(self):
        sts = Mock()
        sts.get_caller_identity.return_value = {"Account": "123"}
        dynamo = Mock()
        dynamo.describe_table.return_value = {
            "Table": {
                "TableArn": "arn:table",
                "KeySchema": [
                    {"AttributeName": "meeting_id", "KeyType": "HASH"},
                    {"AttributeName": "id", "KeyType": "RANGE"},
                ],
                "AttributeDefinitions": [
                    {"AttributeName": "meeting_id", "AttributeType": "S"},
                    {"AttributeName": "id", "AttributeType": "S"},
                ],
            }
        }
        authority = reconcile.DynamoAuthority(
            sts, dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        with self.assertRaises(reconcile.ReconciliationError):
            authority.discover_environment()
        dynamo.scan.assert_not_called()

    def test_classification_retries_unprocessed_keys(self):
        dynamo = FakeDynamo(
            [
                {
                    "Responses": {"reg": [{"id": {"S": "uid-1"}}]},
                    "UnprocessedKeys": {
                        "reg": {
                            "Keys": [{"id": {"S": "uid-2"}}],
                            "ConsistentRead": True,
                        }
                    },
                },
                {"Responses": {"reg": []}, "UnprocessedKeys": {}},
            ]
        )
        authority = reconcile.DynamoAuthority(
            Mock(), dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        authority.environment = reconcile.Environment(
            account="123",
            region="us-east-1",
            table_arn="arn:table",
            table_name="reg",
            key_name="id",
            key_type="S",
        )
        result = authority.classify(["uid-2", "uid-1", "uid-1"])
        self.assertEqual({"uid-1": "active", "uid-2": "stale"}, result)
        self.assertTrue(dynamo.requests[0]["RequestItems"]["reg"]["ConsistentRead"])

    def test_unresolved_keys_fail_closed(self):
        request = {
            "reg": {
                "Keys": [{"id": {"S": "uid-1"}}],
                "ConsistentRead": True,
            }
        }
        dynamo = FakeDynamo(
            [{"Responses": {"reg": []}, "UnprocessedKeys": request} for _ in range(4)]
        )
        authority = reconcile.DynamoAuthority(
            Mock(),
            dynamo,
            "reg",
            "us-east-1",
            sleep=lambda _: None,
            max_retries=3,
        )
        authority.environment = ENVIRONMENT
        with self.assertRaises(reconcile.ReconciliationError):
            authority.classify(["uid-1"])

    def test_malformed_batch_response_cannot_be_classified_as_absent(self):
        malformed_responses = (
            {"UnprocessedKeys": {}},
            {"Responses": {}, "UnprocessedKeys": {}},
            {"Responses": {"reg": []}, "UnprocessedKeys": []},
        )
        for response in malformed_responses:
            authority = reconcile.DynamoAuthority(
                Mock(),
                FakeDynamo([response]),
                "reg",
                "us-east-1",
                sleep=lambda _: None,
            )
            authority.environment = ENVIRONMENT
            with self.subTest(response=response):
                with self.assertRaises(reconcile.ReconciliationError):
                    authority.classify(["uid-1"])

    def test_foreign_unprocessed_table_fails_closed(self):
        dynamo = FakeDynamo(
            [
                {
                    "Responses": {"reg": []},
                    "UnprocessedKeys": {"other": {"Keys": [{"id": {"S": "uid-1"}}]}},
                },
                {"Responses": {"reg": []}, "UnprocessedKeys": {}},
            ]
        )
        authority = reconcile.DynamoAuthority(
            Mock(), dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        authority.environment = ENVIRONMENT
        with self.assertRaises(reconcile.ReconciliationError):
            authority.classify(["uid-1"])

    def test_classification_batches_one_hundred_keys_without_scan(self):
        uids = [f"uid-{number:03d}" for number in range(101)]
        dynamo = FakeDynamo(
            [
                {"Responses": {"reg": []}, "UnprocessedKeys": {}},
                {"Responses": {"reg": []}, "UnprocessedKeys": {}},
            ]
        )
        authority = reconcile.DynamoAuthority(
            Mock(), dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        authority.environment = ENVIRONMENT
        result = authority.classify(uids)
        self.assertEqual(101, len(result))
        self.assertEqual(2, len(dynamo.requests))
        self.assertTrue(all(value == "stale" for value in result.values()))

    def test_present_item_is_active_even_with_deleted_status_field(self):
        dynamo = FakeDynamo(
            [
                {
                    "Responses": {
                        "reg": [
                            {
                                "id": {"S": "uid-1"},
                                "status": {"S": "deleted"},
                            }
                        ]
                    },
                    "UnprocessedKeys": {},
                }
            ]
        )
        authority = reconcile.DynamoAuthority(
            Mock(), dynamo, "reg", "us-east-1", sleep=lambda _: None
        )
        authority.environment = ENVIRONMENT
        self.assertEqual({"uid-1": "active"}, authority.classify(["uid-1"]))

    def test_aws_client_builder_uses_profile_and_assumed_role(self):
        config = reconcile.parse_args(
            cli_args(
                "--aws-profile",
                "readonly",
                "--assume-role-arn",
                "arn:aws:iam::123:role/reconcile",
            )
        )
        source_session = Mock()
        assumed_session = Mock()
        fake_boto = Mock()
        fake_boto.Session.side_effect = [source_session, assumed_session]
        source_session.client.return_value.assume_role.return_value = {
            "Credentials": {
                "AccessKeyId": "redacted",
                "SecretAccessKey": "redacted",
                "SessionToken": "redacted",
            }
        }
        with patch.object(reconcile, "boto3", fake_boto):
            sts, dynamo = reconcile.build_aws_clients(config)
        self.assertEqual(assumed_session.client("sts"), sts)
        self.assertEqual(assumed_session.client("dynamodb"), dynamo)
        fake_boto.Session.assert_any_call(
            profile_name="readonly", region_name="us-east-1"
        )

    def test_sts_or_table_failure_does_not_expose_dependency_details(self):
        sts = Mock()
        sts.get_caller_identity.side_effect = RuntimeError(
            "secret-token dependency body"
        )
        authority = reconcile.DynamoAuthority(
            sts, Mock(), "reg", "us-east-1", sleep=lambda _: None
        )
        with self.assertRaises(reconcile.ReconciliationError) as caught:
            authority.discover_environment()
        self.assertNotIn("secret-token", str(caught.exception))

    def test_table_arn_must_match_sts_account_region_and_name(self):
        sts = Mock()
        sts.get_caller_identity.return_value = {"Account": "123"}
        base_table = {
            "KeySchema": [{"AttributeName": "id", "KeyType": "HASH"}],
            "AttributeDefinitions": [{"AttributeName": "id", "AttributeType": "S"}],
        }
        invalid_arns = (
            "arn:aws:dynamodb:us-east-1:999:table/reg",
            "arn:aws:dynamodb:us-west-2:123:table/reg",
            "arn:aws:s3:us-east-1:123:table/reg",
            "arn:aws:dynamodb:us-east-1:123:table/other",
        )
        for table_arn in invalid_arns:
            dynamo = Mock()
            dynamo.describe_table.return_value = {
                "Table": {**base_table, "TableArn": table_arn}
            }
            authority = reconcile.DynamoAuthority(
                sts, dynamo, "reg", "us-east-1", sleep=lambda _: None
            )
            with self.subTest(table_arn=table_arn):
                with self.assertRaises(reconcile.ReconciliationError):
                    authority.discover_environment()


class PayloadAndPlanTests(unittest.IsolatedAsyncioTestCase):
    async def test_json_and_messagepack_round_trip(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        for encoding in ("json", "msgpack"):
            raw = reconcile.encode_payload(payload, encoding)
            decoded, detected = reconcile.decode_payload(raw)
            self.assertEqual(payload, decoded)
            self.assertEqual(encoding, detected)

    async def test_snapshot_records_revision_digest_and_mapping(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(payload, "json"), 7
                )
            }
        )
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        snapshot = await reconcile.snapshot_candidate(
            values, mappings, "meeting-1", "uid-1"
        )
        self.assertEqual(7, snapshot.revision)
        self.assertEqual("live", snapshot.mapping_state)
        self.assertEqual(64, len(snapshot.digest))

    async def test_mismatched_payload_fails_closed(self):
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(
                        {"meeting_id": "other", "registrant_id": "uid-1"},
                        "json",
                    ),
                    7,
                )
            }
        )
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with self.assertRaises(reconcile.ReconciliationError):
            await reconcile.snapshot_candidate(values, mappings, "meeting-1", "uid-1")

    async def test_plan_is_deterministic_redacted_and_private(self):
        environment = ENVIRONMENT
        plan = reconcile.Plan(
            version=1,
            meeting_id="meeting-1",
            environment=environment,
            candidates=[
                reconcile.CandidateSnapshot(
                    "uid-1", "stale", 7, "json", "abc", "live", 3, False
                )
            ],
        )
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "plan.json"
            reconcile.write_plan(plan, path)
            raw = path.read_text()
            self.assertEqual(plan, reconcile.read_plan(path))
            self.assertNotIn("email", raw)
            self.assertNotIn("token", raw)
            self.assertEqual(0o600, os.stat(path).st_mode & 0o777)

    async def test_unknown_mapping_prevents_applicable_plan(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(payload, "json"), 7
                )
            }
        )
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"unexpected", 3)})
        with self.assertRaises(reconcile.ReconciliationError):
            await reconcile.build_plan(
                "meeting-1",
                ENVIRONMENT,
                FakeDiscovery(["uid-1"]),
                FakeAuthority([]),
                values,
                mappings,
            )

    async def test_missing_or_tombstoned_mapping_prevents_plan(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(payload, "json"), 7
                )
            }
        )
        for mappings in (
            FakeKV({}),
            FakeKV({reconcile.mapping_key("uid-1"): Entry(b"!del", 3)}),
        ):
            with self.subTest(entries=mappings.entries):
                with self.assertRaises(reconcile.ReconciliationError):
                    await reconcile.build_plan(
                        "meeting-1",
                        ENVIRONMENT,
                        FakeDiscovery(["uid-1"]),
                        FakeAuthority([]),
                        values,
                        mappings,
                    )

    async def test_stale_marked_tombstoned_candidate_is_valid_completed_state(self):
        payload = {
            "meeting_id": "meeting-1",
            "registrant_id": "uid-1",
            "_sdc_deleted_at": "2026-07-16T00:00:00Z",
        }
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(payload, "json"), 7
                )
            }
        )
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"!del", 3)})
        plan = await reconcile.build_plan(
            "meeting-1",
            ENVIRONMENT,
            FakeDiscovery(["uid-1"]),
            FakeAuthority([]),
            values,
            mappings,
        )
        self.assertTrue(plan.candidates[0].deleted)
        self.assertEqual("tombstoned", plan.candidates[0].mapping_state)

    async def test_active_soft_deleted_candidate_is_ambiguous(self):
        payload = {
            "meeting_id": "meeting-1",
            "registrant_id": "uid-1",
            "_sdc_deleted_at": "2026-07-16T00:00:00Z",
        }
        values = FakeKV(
            {
                reconcile.registrant_key("uid-1"): Entry(
                    reconcile.encode_payload(payload, "json"), 7
                )
            }
        )
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with self.assertRaises(reconcile.ReconciliationError):
            await reconcile.build_plan(
                "meeting-1",
                ENVIRONMENT,
                FakeDiscovery(["uid-1"]),
                FakeAuthority(["uid-1"]),
                values,
                mappings,
            )


class ApplyRestoreTests(unittest.IsolatedAsyncioTestCase):
    def make_plan(self, digest, revision=7, deleted=False):
        return reconcile.Plan(
            version=1,
            meeting_id="meeting-1",
            environment=ENVIRONMENT,
            candidates=[
                reconcile.CandidateSnapshot(
                    "uid-1",
                    "stale",
                    revision,
                    "json",
                    digest,
                    "live",
                    3,
                    deleted,
                )
            ],
        )

    async def test_apply_adds_marker_with_expected_revision(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        plan = self.make_plan(reconcile.payload_digest(raw))
        completed = await reconcile.apply_plan(
            plan,
            reconcile.Confirmations("meeting-1", "123", "arn:table", 1),
            FakeDiscovery(["uid-1"]),
            FakeAuthority([]),
            values,
            mappings,
            now=lambda: datetime(2026, 7, 16, tzinfo=timezone.utc),
        )
        self.assertEqual(["uid-1"], completed)
        key, updated, revision = values.updates[0]
        decoded, encoding = reconcile.decode_payload(updated)
        self.assertEqual(reconcile.registrant_key("uid-1"), key)
        self.assertEqual(7, revision)
        self.assertEqual("json", encoding)
        self.assertEqual("2026-07-16T00:00:00Z", decoded["_sdc_deleted_at"])

    async def test_apply_rejects_recreated_dynamo_row_without_write(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with self.assertRaises(reconcile.PlanDriftError):
            await reconcile.apply_plan(
                self.make_plan(reconcile.payload_digest(raw)),
                reconcile.Confirmations("meeting-1", "123", "arn:table", 1),
                FakeDiscovery(["uid-1"]),
                FakeAuthority(["uid-1"]),
                values,
                mappings,
                now=lambda: datetime.now(timezone.utc),
            )
        self.assertEqual([], values.updates)

    async def test_apply_rejects_confirmation_mismatch_before_write(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with self.assertRaises(reconcile.PlanDriftError):
            await reconcile.apply_plan(
                self.make_plan(reconcile.payload_digest(raw)),
                reconcile.Confirmations("meeting-1", "wrong", "arn:table", 1),
                FakeDiscovery(["uid-1"]),
                FakeAuthority([]),
                values,
                mappings,
                now=lambda: datetime.now(timezone.utc),
            )
        self.assertEqual([], values.updates)

    async def test_already_deleted_stale_uid_is_returned_for_verification(self):
        payload = {
            "meeting_id": "meeting-1",
            "registrant_id": "uid-1",
            "_sdc_deleted_at": "2026-07-16T00:00:00Z",
        }
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        plan = self.make_plan(reconcile.payload_digest(raw), deleted=True)
        completed = await reconcile.apply_plan(
            plan,
            reconcile.Confirmations("meeting-1", "123", "arn:table", 1),
            FakeDiscovery(["uid-1"]),
            FakeAuthority([]),
            values,
            mappings,
            now=lambda: datetime.now(timezone.utc),
        )
        self.assertEqual(["uid-1"], completed)
        self.assertEqual([], values.updates)

    async def test_revision_conflict_reports_partial_apply_without_retry(self):
        payload1 = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        payload2 = {"meeting_id": "meeting-1", "registrant_id": "uid-2"}
        raw1 = reconcile.encode_payload(payload1, "json")
        raw2 = reconcile.encode_payload(payload2, "json")
        values = FailingUpdateKV(
            {
                reconcile.registrant_key("uid-1"): Entry(raw1, 7),
                reconcile.registrant_key("uid-2"): Entry(raw2, 8),
            },
            fail_key=reconcile.registrant_key("uid-2"),
        )
        mappings = FakeKV(
            {
                reconcile.mapping_key("uid-1"): Entry(b"1", 3),
                reconcile.mapping_key("uid-2"): Entry(b"1", 4),
            }
        )
        plan = reconcile.Plan(
            1,
            "meeting-1",
            ENVIRONMENT,
            [
                reconcile.CandidateSnapshot(
                    "uid-1",
                    "stale",
                    7,
                    "json",
                    reconcile.payload_digest(raw1),
                    "live",
                    3,
                    False,
                ),
                reconcile.CandidateSnapshot(
                    "uid-2",
                    "stale",
                    8,
                    "json",
                    reconcile.payload_digest(raw2),
                    "live",
                    4,
                    False,
                ),
            ],
        )
        with self.assertRaises(reconcile.ReconciliationError) as caught:
            await reconcile.apply_plan(
                plan,
                reconcile.Confirmations("meeting-1", "123", "arn:table", 2),
                FakeDiscovery(["uid-1", "uid-2"]),
                FakeAuthority([]),
                values,
                mappings,
                now=lambda: datetime.now(timezone.utc),
            )
        self.assertIn("completed=['uid-1']", str(caught.exception))
        self.assertEqual(1, len(values.updates))

    async def test_apply_rechecks_each_uid_immediately_before_write(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        authority = Mock()
        authority.classify.side_effect = [
            {"uid-1": "stale"},
            {"uid-1": "active"},
        ]
        with self.assertRaises(reconcile.PlanDriftError):
            await reconcile.apply_plan(
                self.make_plan(reconcile.payload_digest(raw)),
                reconcile.Confirmations("meeting-1", "123", "arn:table", 1),
                FakeDiscovery(["uid-1"]),
                authority,
                values,
                mappings,
                now=lambda: datetime.now(timezone.utc),
            )
        self.assertEqual([], values.updates)
        self.assertEqual(2, authority.classify.call_count)

    async def test_apply_rechecks_mapping_revision_before_write(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        key = reconcile.mapping_key("uid-1")
        mappings = SequencedGetKV(key, [Entry(b"1", 3), Entry(b"1", 4)])
        with self.assertRaises(reconcile.PlanDriftError):
            await reconcile.apply_plan(
                self.make_plan(reconcile.payload_digest(raw)),
                reconcile.Confirmations("meeting-1", "123", "arn:table", 1),
                FakeDiscovery(["uid-1"]),
                FakeAuthority([]),
                values,
                mappings,
                now=lambda: datetime.now(timezone.utc),
            )
        self.assertEqual([], values.updates)

    async def test_run_apply_binds_target_meeting_to_plan(self):
        payload = {"meeting_id": "meeting-1", "registrant_id": "uid-1"}
        raw = reconcile.encode_payload(payload, "json")
        plan = self.make_plan(reconcile.payload_digest(raw))
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 7)})
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "plan.json"
            reconcile.write_plan(plan, path)
            config = reconcile.parse_args(
                cli_args(
                    "--plan",
                    str(path),
                    "--apply",
                    "--confirm-meeting-id",
                    "meeting-1",
                    "--confirm-aws-account",
                    "123",
                    "--confirm-table-arn",
                    "arn:table",
                    "--confirm-stale-count",
                    "1",
                    meeting_id="different-meeting",
                    table="reg",
                )
            )
            with self.assertRaises(reconcile.PlanDriftError):
                await reconcile._run_apply(
                    config,
                    plan.environment,
                    FakeDiscovery(["uid-1"]),
                    FakeAuthority([]),
                    values,
                    mappings,
                )
        self.assertEqual([], values.updates)

    async def test_restore_requires_dynamo_presence_and_removes_only_marker(self):
        payload = {
            "meeting_id": "meeting-1",
            "registrant_id": "uid-1",
            "name": "preserved",
            "_sdc_deleted_at": "2026-07-16T00:00:00Z",
        }
        raw = reconcile.encode_payload(payload, "msgpack")
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 9)})
        await reconcile.restore_candidate(
            "meeting-1",
            "uid-1",
            ENVIRONMENT,
            reconcile.RestoreConfirmations("meeting-1", "123", "arn:table"),
            FakeAuthority(["uid-1"]),
            values,
        )
        decoded, encoding = reconcile.decode_payload(values.updates[0][1])
        self.assertEqual("msgpack", encoding)
        self.assertEqual("preserved", decoded["name"])
        self.assertNotIn("_sdc_deleted_at", decoded)

    async def test_restore_rejects_absent_authoritative_row(self):
        values = FakeKV({})
        with self.assertRaises(reconcile.ReconciliationError):
            await reconcile.restore_candidate(
                "meeting-1",
                "uid-1",
                ENVIRONMENT,
                reconcile.RestoreConfirmations("meeting-1", "123", "arn:table"),
                FakeAuthority([]),
                values,
            )
        self.assertEqual([], values.updates)

    async def test_restore_rejects_payload_without_marker(self):
        raw = reconcile.encode_payload(
            {"meeting_id": "meeting-1", "registrant_id": "uid-1"}, "json"
        )
        values = FakeKV({reconcile.registrant_key("uid-1"): Entry(raw, 9)})
        with self.assertRaises(reconcile.ReconciliationError):
            await reconcile.restore_candidate(
                "meeting-1",
                "uid-1",
                ENVIRONMENT,
                reconcile.RestoreConfirmations("meeting-1", "123", "arn:table"),
                FakeAuthority(["uid-1"]),
                values,
            )
        self.assertEqual([], values.updates)


class VerificationTests(unittest.IsolatedAsyncioTestCase):
    async def test_apply_verification_waits_for_absence_and_tombstone(self):
        discovery = FakeDiscovery([])
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"!del", 4)})
        await reconcile.verify_apply(
            "meeting-1",
            ["uid-1"],
            discovery,
            mappings,
            timeout=0.05,
            interval=0,
            sleep=lambda _: asyncio.sleep(0),
        )

    async def test_verification_timeout_makes_no_writes(self):
        discovery = FakeDiscovery(["uid-1"])
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 3)})
        with self.assertRaises(reconcile.VerificationError):
            await reconcile.verify_apply(
                "meeting-1",
                ["uid-1"],
                discovery,
                mappings,
                timeout=0,
                interval=0,
                sleep=lambda _: asyncio.sleep(0),
            )
        self.assertEqual([], mappings.updates)

    async def test_restore_verification_waits_for_index_and_live_mapping(self):
        discovery = FakeDiscovery(["uid-1"])
        mappings = FakeKV({reconcile.mapping_key("uid-1"): Entry(b"1", 4)})
        await reconcile.verify_restore(
            "meeting-1",
            "uid-1",
            discovery,
            mappings,
            timeout=0.05,
            interval=0,
        )

    async def test_mapping_dependency_failure_does_not_trigger_write(self):
        mappings = FakeKV({})
        with self.assertRaises(NotFoundError):
            await reconcile.verify_apply(
                "meeting-1",
                ["uid-1"],
                FakeDiscovery([]),
                mappings,
                timeout=0.05,
                interval=0,
            )
        self.assertEqual([], mappings.updates)


class OutputTests(unittest.TestCase):
    def test_main_prints_only_redacted_summary(self):
        result = {
            "mode": "dry-run",
            "candidate_count": 1,
            "active_count": 0,
            "stale_count": 1,
            "stale_uids": ["uid-1"],
        }
        with (
            patch.object(reconcile, "run", AsyncMock(return_value=result)),
            patch("builtins.print") as output,
        ):
            self.assertEqual(
                0,
                reconcile.main(cli_args()),
            )
        rendered = output.call_args.args[0]
        for secret in ("email", "username", "token", "AccessKeyId"):
            self.assertNotIn(secret, rendered)


if __name__ == "__main__":
    unittest.main()
