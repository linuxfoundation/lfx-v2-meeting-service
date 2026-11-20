# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

import json
import os
import time
import unittest
import tempfile
from contextlib import asynccontextmanager
from dataclasses import replace
from pathlib import Path
from unittest.mock import AsyncMock, Mock, patch
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

import asyncpg

from common import (
    NotFoundError, PostgresMappings, ReconciliationError, open_nats,
)
import reconcile_meeting_registrants as reconcile
from test_reconcile_meeting_registrants import FakeDiscovery, ENVIRONMENT, TARGETS, cli_args


class FakeConnection:
    def __init__(self, row=None, ready=True):
        self.row = row
        self.ready = ready
        self.readonly = []
        self.queries = []

    @asynccontextmanager
    async def transaction(self, readonly=False):
        self.readonly.append(readonly)
        yield self

    async def fetchval(self, query, **kwargs):
        self.queries.append((query, (), kwargs))
        return self.ready

    async def fetchrow(self, query, *args, **kwargs):
        self.queries.append((query, args, kwargs))
        return self.row


class FakePool:
    def __init__(self, connection):
        self.connection = connection

    @asynccontextmanager
    async def acquire(self, timeout):
        yield self.connection


class PostgresMappingTests(unittest.IsolatedAsyncioTestCase):
    async def test_legacy_kv_plans_cannot_compare_pg_revisions(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "plan.json"
            reconcile.write_plan(reconcile.Plan(2, "987", ENVIRONMENT, TARGETS, []), path)
            with self.assertRaisesRegex(ReconciliationError, "version is unsupported"):
                reconcile.read_plan(path)

    async def test_changed_pg_target_is_rejected_before_apply(self):
        config = replace(reconcile.parse_args(cli_args()), mode="apply")
        resources = Mock(close=AsyncMock())
        mappings = Mock(identity="different-pg-target")
        plan = Mock(targets=replace(reconcile.target_identity(config), state_database_identity="reviewed-pg-target"))
        authority = Mock(discover_environment=Mock(return_value=ENVIRONMENT))
        with patch.object(reconcile, "build_aws_clients", return_value=(None, None)), patch.object(
            reconcile, "DynamoAuthority", return_value=authority
        ), patch.object(reconcile, "OpenSearchDiscovery", return_value=FakeDiscovery([])), patch.object(
            reconcile, "read_plan", return_value=plan
        ), patch.object(reconcile, "open_nats", AsyncMock(return_value=(resources, Mock(), mappings))), patch.object(
            reconcile, "_run_apply", AsyncMock()
        ) as apply:
            with self.assertRaisesRegex(ReconciliationError, "runtime targets differ"):
                await reconcile.run(config)
        apply.assert_not_awaited()
        resources.close.assert_awaited_once()

    async def test_named_state_fields_match_runtime_encoding(self):
        for state, expected in {
            "empty": b"", "processed": b"1", "deleted": b"!del",
            "legacy": b"uid-1",
        }.items():
            with self.subTest(state=state):
                connection = FakeConnection({"state": state, "source_uid": "uid-1", "version": 41})
                store = PostgresMappings(FakePool(connection), "identity")
                entry = await store.get("v1_meeting_registrants.uid-1")
                self.assertEqual(entry.value, expected)
                self.assertEqual(entry.revision, 41)
                self.assertEqual(connection.readonly, [True])
                self.assertEqual(connection.queries[-1][1], ("uid-1",))

    async def test_principal_preserves_authority_and_parent(self):
        connection = FakeConnection({
            "state": "principal", "source_uid": "uid-1", "username": "test.user",
            "parent_meeting_id": "meeting-1", "principal_authoritative": True,
            "version": 42,
        })
        store = PostgresMappings(FakePool(connection), "identity")
        entry = await store.get("v1_meeting_registrants.uid-1")
        self.assertEqual(json.loads(entry.value), {
            "uid": "uid-1", "username": "test.user", "meeting_id": "meeting-1",
            "principal_authoritative": True,
        })

    async def test_unverified_state_and_missing_rows_fail_closed(self):
        for ready, error in [(False, ReconciliationError), (True, NotFoundError)]:
            with self.subTest(ready=ready):
                store = PostgresMappings(FakePool(FakeConnection(ready=ready)), "identity")
                with self.assertRaises(error):
                    await store.get("v1_meeting_registrants.uid-1")

    async def test_reader_never_updates_processing_state(self):
        store = PostgresMappings(FakePool(FakeConnection()), "identity")
        with self.assertRaisesRegex(ReconciliationError, "read-only"):
            await store.update("v1_meeting_registrants.uid-1", b"!del", 41)

    async def test_opener_reads_only_source_kv_and_closes_both(self):
        connection = Mock()
        connection.close = AsyncMock()
        jetstream = Mock()
        jetstream.key_value = AsyncMock(return_value=Mock())
        connection.jetstream.return_value = jetstream
        mappings = Mock(identity="pg-identity")
        mappings.close = AsyncMock()
        with patch("common.nats.connect", AsyncMock(return_value=connection)), patch(
            "common.PostgresMappings.open", AsyncMock(return_value=mappings)
        ):
            resources, _, returned = await open_nats("nats://localhost:4222", 5)
        jetstream.key_value.assert_awaited_once_with("v1-objects")
        self.assertIs(returned, mappings)
        await resources.close()
        mappings.close.assert_awaited_once()
        connection.close.assert_awaited_once()

    async def test_failed_state_open_closes_source_connection(self):
        connection = Mock()
        connection.close = AsyncMock()
        connection.jetstream.return_value.key_value = AsyncMock(return_value=Mock())
        with patch("common.nats.connect", AsyncMock(return_value=connection)), patch(
            "common.PostgresMappings.open", AsyncMock(side_effect=ReconciliationError("not verified"))
        ):
            with self.assertRaisesRegex(ReconciliationError, "not verified"):
                await open_nats("nats://localhost:4222", 5)
        connection.close.assert_awaited_once()


@unittest.skipUnless(os.environ.get("MEETING_STATE_TEST_DATABASE_URL"), "disposable PG URL required")
class PostgresVerificationTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.admin = await asyncpg.connect(os.environ["MEETING_STATE_TEST_DATABASE_URL"])
        self.schema = f"meeting_py_state_test_{time.time_ns()}"
        await self.admin.execute(f'CREATE SCHEMA "{self.schema}"')
        await self.admin.execute(f'SET search_path TO "{self.schema}"')
        schema_file = Path(__file__).resolve().parents[2] / "internal/infrastructure/eventstate/schema.sql"
        await self.admin.execute(schema_file.read_text())
        endpoint = urlsplit(os.environ["MEETING_STATE_TEST_DATABASE_URL"])
        query = dict(parse_qsl(endpoint.query))
        query["search_path"] = self.schema
        self.dsn = urlunsplit(endpoint._replace(query=urlencode(query)))
        self.store = None

    async def asyncTearDown(self):
        if self.store is not None:
            await self.store.close()
        await self.admin.execute(f'DROP SCHEMA "{self.schema}" CASCADE')
        await self.admin.close()

    async def test_verification_reads_pg_tombstones_and_pg_revisions(self):
        with self.assertRaisesRegex(ReconciliationError, "not verified"):
            await PostgresMappings.open(self.dsn, 5)
        await self.admin.execute(
            "INSERT INTO meeting_event_state_cutover(singleton,source_last_sequence) VALUES(true,999)"
        )
        await self.admin.execute(
            "INSERT INTO meeting_resource_sync_state(resource_kind,source_id,state,version) "
            "VALUES('v1_meeting_registrants','uid-1','deleted',41)"
        )
        self.store = await PostgresMappings.open(self.dsn, 5)
        self.assertTrue(self.store.identity)
        await reconcile.verify_apply("meeting-1", ["uid-1"], FakeDiscovery([]), self.store, 1, 0.01)
        await self.admin.execute(
            "UPDATE meeting_resource_sync_state SET state='principal',source_uid='uid-1',"
            "username='test.user',parent_meeting_id='meeting-1',principal_authoritative=true,version=42"
        )
        await reconcile.verify_restore("meeting-1", "uid-1", FakeDiscovery(["uid-1"]), self.store, 41, 1, 0.01)
        entry = await self.store.get("v1_meeting_registrants.uid-1")
        self.assertEqual(entry.revision, 42)
        self.assertTrue(json.loads(entry.value)["principal_authoritative"])


if __name__ == "__main__":
    unittest.main()
