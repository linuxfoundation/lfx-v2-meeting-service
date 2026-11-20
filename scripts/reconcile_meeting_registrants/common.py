# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

"""Source NATS KV, verified Postgres state, and payload helpers."""

import asyncio
import hashlib
import json
import os
from dataclasses import dataclass
from typing import Any, Protocol
from urllib.parse import urlsplit

import asyncpg
import msgpack
import nats

REGISTRANT_PREFIX = "itx-zoom-meetings-registrants-v2."
MAPPING_PREFIX = "v1_meeting_registrants."
DELETED_FIELD = "_sdc_deleted_at"
LIVE_MAPPING = b"1"
TOMBSTONE_MAPPING = b"!del"
DATABASE_TIMEOUT = 5.0


class ReconciliationError(RuntimeError):
    """A safe, operator-facing reconciliation failure."""


class NotFoundError(ReconciliationError):
    """An expected KV key was not found."""


class RevisionConflictError(ReconciliationError):
    """A KV revision changed concurrently."""


class PlanDriftError(ReconciliationError):
    """Current state differs from the reviewed plan."""


class VerificationError(ReconciliationError):
    """Downstream state did not converge before the deadline."""


class KVEntry(Protocol):
    value: bytes
    revision: int


class KVStore(Protocol):
    async def get(self, key: str) -> KVEntry: ...

    async def update(self, key: str, value: bytes, last: int) -> int: ...


def registrant_key(uid: str) -> str:
    return f"{REGISTRANT_PREFIX}{uid}"


def mapping_key(uid: str) -> str:
    return f"{MAPPING_PREFIX}{uid}"


def encode_payload(payload: dict[str, Any], encoding: str) -> bytes:
    if encoding == "json":
        return json.dumps(payload, sort_keys=True, separators=(",", ":")).encode(
            "utf-8"
        )
    if encoding == "msgpack":
        return msgpack.packb(payload, use_bin_type=True)
    raise ReconciliationError(f"unsupported payload encoding: {encoding}")


def decode_payload(raw: bytes) -> tuple[dict[str, Any], str]:
    try:
        payload = json.loads(raw)
        if isinstance(payload, dict):
            return payload, "json"
    except (UnicodeDecodeError, json.JSONDecodeError):
        pass
    try:
        payload = msgpack.unpackb(raw, raw=False, strict_map_key=False)
        if isinstance(payload, dict):
            return payload, "msgpack"
    except (ValueError, TypeError, msgpack.ExtraData):
        pass
    raise ReconciliationError("registrant payload is neither JSON nor MessagePack")


def payload_digest(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


async def get_entry(store: KVStore, key: str) -> KVEntry:
    try:
        return await store.get(key)
    except NotFoundError:
        raise
    except Exception as error:
        if error.__class__.__name__ in {"KeyNotFoundError", "NotFoundError"}:
            raise NotFoundError(key) from error
        raise ReconciliationError(f"state/source read failed for {key}") from error


def mapping_state(
    value: Any, expected_uid: str | None = None, expected_meeting_id: str | None = None
) -> str:
    raw = value.encode() if isinstance(value, str) else bytes(value)
    if raw == LIVE_MAPPING:
        return "live"
    if raw == TOMBSTONE_MAPPING:
        return "tombstoned"
    try:
        decoded = raw.decode("utf-8")
    except UnicodeDecodeError:
        return "unknown"
    try:
        data = json.loads(decoded)
    except json.JSONDecodeError:
        parts = decoded.split("|", 2)
        if len(parts) == 3 and parts[0] and parts[2]:
            if expected_uid is not None and parts[0] != expected_uid:
                return "unknown"
            if expected_meeting_id is not None and parts[2] != expected_meeting_id:
                return "unknown"
            return "live"
        return "unknown"
    if (
        isinstance(data, dict)
        and isinstance(data.get("uid"), str)
        and data["uid"]
        and isinstance(data.get("username"), str)
        and isinstance(data.get("meeting_id"), str)
        and data["meeting_id"]
    ):
        if expected_uid is not None and data["uid"] != expected_uid:
            return "unknown"
        if (
            expected_meeting_id is not None
            and data["meeting_id"] != expected_meeting_id
        ):
            return "unknown"
        return "live"
    return "unknown"


class NATSKV:
    def __init__(self, store: Any) -> None:
        self.store = store

    async def get(self, key: str) -> Any:
        try:
            return await self.store.get(key)
        except Exception as error:
            if error.__class__.__name__ in {"KeyNotFoundError", "NotFoundError"}:
                raise NotFoundError(key) from error
            raise ReconciliationError(f"NATS KV read failed for {key}") from error

    async def update(self, key: str, value: bytes, last: int) -> int:
        try:
            return await self.store.update(key, value, last=last)
        except Exception as error:
            if error.__class__.__name__ in {
                "KeyWrongLastSequenceError",
                "BadRequestError",
            }:
                raise RevisionConflictError(key) from error
            raise ReconciliationError(f"NATS KV update failed for {key}") from error


@dataclass(frozen=True)
class PostgresEntry:
    value: bytes
    revision: int


class PostgresMappings:
    """Read-only meeting-owned state; PG revisions never address source KV."""

    def __init__(self, pool: Any, identity: str) -> None:
        self.pool = pool
        self.identity = identity

    @classmethod
    async def open(cls, dsn: str, timeout: float) -> "PostgresMappings":
        if not dsn:
            raise ReconciliationError("MEETING_STATE_DATABASE_URL is required")
        pool = None
        try:
            endpoint = urlsplit(dsn)
            if endpoint.scheme not in {"postgres", "postgresql"} or not endpoint.hostname:
                raise ReconciliationError("Postgres state requires a database URL")
            pool = await asyncpg.create_pool(
                dsn=dsn,
                min_size=1,
                max_size=4,
                timeout=min(timeout, DATABASE_TIMEOUT),
                command_timeout=DATABASE_TIMEOUT,
                server_settings={"application_name": "meeting-registrant-reconcile"},
            )
            async with pool.acquire(timeout=DATABASE_TIMEOUT) as connection:
                async with connection.transaction(readonly=True):
                    await cls._require_ready(connection)
                    metadata = await connection.fetchrow(
                        "SELECT current_database() AS database, current_schema() AS schema",
                        timeout=DATABASE_TIMEOUT,
                    )
            # Bind plans to the actual database/schema and stable endpoint,
            # without retaining credentials, role names, or DSN query values.
            identity = hashlib.sha256(
                json.dumps(
                    [endpoint.hostname.lower(), endpoint.port or 5432,
                     metadata["database"], metadata["schema"]],
                    separators=(",", ":"),
                ).encode("utf-8")
            ).hexdigest()
            return cls(pool, identity)
        except Exception as error:
            if pool is not None:
                pool.terminate()
            if isinstance(error, ReconciliationError):
                raise
            raise ReconciliationError("Postgres state connection failed") from None

    @staticmethod
    async def _require_ready(connection: Any) -> None:
        if not await connection.fetchval(
            "SELECT EXISTS (SELECT 1 FROM meeting_event_state_cutover)",
            timeout=DATABASE_TIMEOUT,
        ):
            raise ReconciliationError("meeting Postgres cutover is not verified")

    async def get(self, key: str) -> PostgresEntry:
        if not key.startswith(MAPPING_PREFIX) or not key[len(MAPPING_PREFIX):]:
            raise ReconciliationError("unsupported Postgres state key")
        try:
            async with self.pool.acquire(timeout=DATABASE_TIMEOUT) as connection:
                async with connection.transaction(readonly=True):
                    await self._require_ready(connection)
                    row = await connection.fetchrow(
                        "SELECT state, source_uid, username, parent_meeting_id, "
                        "principal_authoritative, version FROM meeting_resource_sync_state "
                        "WHERE resource_kind='v1_meeting_registrants' AND source_id=$1",
                        key[len(MAPPING_PREFIX):],
                        timeout=DATABASE_TIMEOUT,
                    )
            if row is None:
                raise NotFoundError(key)
            state = row["state"]
            if state == "empty":
                value = b""
            elif state == "processed":
                value = LIVE_MAPPING
            elif state == "deleted":
                value = TOMBSTONE_MAPPING
            elif state == "legacy":
                value = row["source_uid"].encode("utf-8")
            elif state == "principal":
                principal = {
                    "uid": row["source_uid"], "username": row["username"],
                    "meeting_id": row["parent_meeting_id"],
                }
                if row["principal_authoritative"]:
                    principal["principal_authoritative"] = True
                value = json.dumps(principal, separators=(",", ":")).encode("utf-8")
            else:
                raise ReconciliationError("unrecognized Postgres processing state")
            return PostgresEntry(value, row["version"])
        except ReconciliationError:
            raise
        except Exception:
            raise ReconciliationError("Postgres mapping read failed") from None

    async def update(self, key: str, value: bytes, last: int) -> int:
        raise ReconciliationError("Postgres processing state is read-only in this tool")

    async def close(self) -> None:
        try:
            await asyncio.wait_for(self.pool.close(), timeout=DATABASE_TIMEOUT)
        except TimeoutError:
            self.pool.terminate()


class ReconciliationConnections:
    """Close both stores after a completed or failed reconciliation run."""

    def __init__(self, connection: Any, mappings: PostgresMappings) -> None:
        self.connection = connection
        self.mappings = mappings

    async def close(self) -> None:
        try:
            await self.mappings.close()
        finally:
            await self.connection.close()


async def open_nats(
    nats_url: str, connect_timeout: float
) -> tuple[ReconciliationConnections, NATSKV, PostgresMappings]:
    connection = None
    try:
        connection = await nats.connect(
            servers=[nats_url], connect_timeout=connect_timeout
        )
        jetstream = connection.jetstream()
        values = NATSKV(await jetstream.key_value("v1-objects"))
        mappings = await PostgresMappings.open(
            os.environ.get("MEETING_STATE_DATABASE_URL", ""), connect_timeout
        )
        return ReconciliationConnections(connection, mappings), values, mappings
    except Exception as error:
        if connection is not None:
            await connection.close()
        if isinstance(error, ReconciliationError):
            raise
        raise ReconciliationError("reconciliation store connection failed") from None
