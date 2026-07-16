"""NATS KV and payload helpers adapted from existing migration scripts."""

import hashlib
import json
from typing import Any, Protocol

import msgpack
import nats

REGISTRANT_PREFIX = "itx-zoom-meetings-registrants-v2."
MAPPING_PREFIX = "v1_meeting_registrants."
DELETED_FIELD = "_sdc_deleted_at"
LIVE_MAPPING = b"1"
TOMBSTONE_MAPPING = b"!del"


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
        raise ReconciliationError(f"NATS KV read failed for {key}") from error


def mapping_state(value: Any) -> str:
    raw = value.encode() if isinstance(value, str) else bytes(value)
    if raw == LIVE_MAPPING:
        return "live"
    if raw == TOMBSTONE_MAPPING:
        return "tombstoned"
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


async def open_nats(
    nats_url: str, connect_timeout: float
) -> tuple[Any, NATSKV, NATSKV]:
    try:
        connection = await nats.connect(
            servers=[nats_url], connect_timeout=connect_timeout
        )
        jetstream = connection.jetstream()
        values = NATSKV(await jetstream.key_value("v1-objects"))
        mappings = NATSKV(await jetstream.key_value("v1-mappings"))
        return connection, values, mappings
    except Exception as error:
        raise ReconciliationError("NATS connection failed") from error
