#!/usr/bin/env python3
"""Reconcile indexed meeting registrants against authoritative DynamoDB rows."""

import argparse
import asyncio
import json
import os
import sys
import time
from dataclasses import asdict, dataclass, replace
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Awaitable, Callable, Iterable, Protocol

try:
    import boto3
except ImportError:  # pragma: no cover - exercised by the dependency check
    boto3 = None

try:
    import requests
except ImportError:  # pragma: no cover - exercised by the dependency check
    requests = None

from common import (
    DELETED_FIELD,
    KVStore,
    PlanDriftError,
    ReconciliationError,
    RevisionConflictError,
    VerificationError,
    decode_payload,
    encode_payload,
    get_entry,
    mapping_key,
    mapping_state,
    open_nats,
    payload_digest,
    registrant_key,
)

PLAN_VERSION = 1


class Discovery(Protocol):
    def discover(self, meeting_id: str) -> list[str]: ...

    def contains(self, meeting_id: str, uid: str) -> bool: ...


class Authority(Protocol):
    def classify(self, uids: Iterable[str]) -> dict[str, str]: ...


class HTTPResponse(Protocol):
    def raise_for_status(self) -> None: ...

    def json(self) -> Any: ...


class HTTPSession(Protocol):
    def post(self, url: str, **kwargs: Any) -> HTTPResponse: ...

    def delete(self, url: str, **kwargs: Any) -> HTTPResponse: ...


@dataclass(frozen=True)
class Config:
    meeting_id: str
    mode: str
    restore_uid: str | None
    opensearch_url: str
    opensearch_index: str
    nats_url: str
    dynamodb_table: str
    aws_region: str
    aws_profile: str | None
    assume_role_arn: str | None
    plan_path: Path
    confirm_meeting_id: str | None
    confirm_aws_account: str | None
    confirm_table_arn: str | None
    confirm_stale_count: int | None
    request_timeout: float
    verify_timeout: float
    verify_interval: float


@dataclass(frozen=True)
class Environment:
    account: str
    region: str
    table_arn: str
    table_name: str
    key_name: str
    key_type: str


@dataclass(frozen=True)
class CandidateSnapshot:
    uid: str
    classification: str
    revision: int
    encoding: str
    digest: str
    mapping_state: str
    mapping_revision: int
    deleted: bool


@dataclass(frozen=True)
class Plan:
    version: int
    meeting_id: str
    environment: Environment
    candidates: list[CandidateSnapshot]


@dataclass(frozen=True)
class Confirmations:
    meeting_id: str
    aws_account: str
    table_arn: str
    stale_count: int


@dataclass(frozen=True)
class RestoreConfirmations:
    meeting_id: str
    aws_account: str
    table_arn: str


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Reconcile one meeting's indexed registrants with DynamoDB."
    )
    parser.add_argument("--meeting-id", required=True)
    parser.add_argument("--opensearch-url", required=True)
    parser.add_argument("--opensearch-index", default="resources")
    parser.add_argument("--nats-url", required=True)
    parser.add_argument("--dynamodb-table", required=True)
    parser.add_argument("--aws-region", default="us-east-1")
    parser.add_argument("--aws-profile")
    parser.add_argument("--assume-role-arn")
    parser.add_argument("--plan", default="reconciliation-plan.json")
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--apply", action="store_true")
    modes.add_argument("--restore", metavar="REGISTRANT_UID")
    parser.add_argument("--confirm-meeting-id")
    parser.add_argument("--confirm-aws-account")
    parser.add_argument("--confirm-table-arn")
    parser.add_argument("--confirm-stale-count", type=int)
    parser.add_argument("--request-timeout", type=float, default=15.0)
    parser.add_argument("--verify-timeout", type=float, default=60.0)
    parser.add_argument("--verify-interval", type=float, default=2.0)
    return parser


def parse_args(argv: list[str] | None = None) -> Config:
    parser = build_parser()
    args = parser.parse_args(argv)
    mode = "apply" if args.apply else "restore" if args.restore else "dry-run"
    _validate_args(parser, args, mode)
    return Config(
        meeting_id=args.meeting_id,
        mode=mode,
        restore_uid=args.restore,
        opensearch_url=args.opensearch_url.rstrip("/"),
        opensearch_index=args.opensearch_index,
        nats_url=args.nats_url,
        dynamodb_table=args.dynamodb_table,
        aws_region=args.aws_region,
        aws_profile=args.aws_profile,
        assume_role_arn=args.assume_role_arn,
        plan_path=Path(args.plan),
        confirm_meeting_id=args.confirm_meeting_id,
        confirm_aws_account=args.confirm_aws_account,
        confirm_table_arn=args.confirm_table_arn,
        confirm_stale_count=args.confirm_stale_count,
        request_timeout=args.request_timeout,
        verify_timeout=args.verify_timeout,
        verify_interval=args.verify_interval,
    )


def _validate_args(
    parser: argparse.ArgumentParser, args: argparse.Namespace, mode: str
) -> None:
    if args.request_timeout <= 0 or args.verify_timeout < 0:
        parser.error("timeouts must be positive")
    if args.verify_interval < 0:
        parser.error("verify interval must not be negative")
    confirmations = (
        args.confirm_meeting_id,
        args.confirm_aws_account,
        args.confirm_table_arn,
    )
    if mode in {"apply", "restore"} and any(value is None for value in confirmations):
        parser.error("write modes require meeting, account, and table confirmations")
    if mode == "apply" and args.confirm_stale_count is None:
        parser.error("apply requires --confirm-stale-count")


class OpenSearchDiscovery:
    def __init__(
        self,
        session: HTTPSession,
        base_url: str,
        index: str,
        timeout: float,
    ) -> None:
        self.session = session
        self.base_url = base_url.rstrip("/")
        self.index = index
        self.timeout = timeout

    def discover(self, meeting_id: str) -> list[str]:
        scroll_id: str | None = None
        found: set[str] = set()
        try:
            payload = self._first_page(meeting_id)
            while True:
                scroll_id = _required_string(payload, "_scroll_id")
                hits = self._hits(payload)
                if not hits:
                    return sorted(found)
                found.update(self._uids(hits))
                payload = self._next_page(scroll_id)
        finally:
            if scroll_id is not None:
                self._clear_scroll(scroll_id)

    def contains(self, meeting_id: str, uid: str) -> bool:
        body = {
            "size": 1,
            "_source": ["object_id"],
            "query": {
                "bool": {
                    "filter": [
                        *self._meeting_filter(meeting_id),
                        {"term": {"object_id": uid}},
                    ]
                }
            },
        }
        payload = self._post(f"{self.base_url}/{self.index}/_search", json=body)
        return bool(self._hits(payload))

    def _first_page(self, meeting_id: str) -> dict[str, Any]:
        body = {
            "size": 500,
            "_source": ["object_id"],
            "query": {"bool": {"filter": self._meeting_filter(meeting_id)}},
        }
        return self._post(
            f"{self.base_url}/{self.index}/_search",
            params={"scroll": "1m"},
            json=body,
        )

    @staticmethod
    def _meeting_filter(meeting_id: str) -> list[dict[str, Any]]:
        return [
            {"term": {"object_type": "v1_meeting_registrant"}},
            {"term": {"data.meeting_id": meeting_id}},
        ]

    def _next_page(self, scroll_id: str) -> dict[str, Any]:
        return self._post(
            f"{self.base_url}/_search/scroll",
            json={"scroll": "1m", "scroll_id": scroll_id},
        )

    def _post(self, url: str, **kwargs: Any) -> dict[str, Any]:
        try:
            response = self.session.post(url, timeout=self.timeout, **kwargs)
            response.raise_for_status()
            payload = response.json()
        except Exception as error:
            raise ReconciliationError("OpenSearch request failed") from error
        if not isinstance(payload, dict):
            raise ReconciliationError("OpenSearch returned malformed JSON")
        return payload

    @staticmethod
    def _hits(payload: dict[str, Any]) -> list[dict[str, Any]]:
        hits = payload.get("hits", {}).get("hits")
        if not isinstance(hits, list):
            raise ReconciliationError("OpenSearch response has malformed hits")
        return hits

    @staticmethod
    def _uids(hits: Iterable[dict[str, Any]]) -> list[str]:
        uids = []
        for hit in hits:
            source = hit.get("_source") if isinstance(hit, dict) else None
            uid = source.get("object_id") if isinstance(source, dict) else None
            if not isinstance(uid, str) or not uid:
                raise ReconciliationError("OpenSearch hit lacks object_id")
            uids.append(uid)
        return uids

    def _clear_scroll(self, scroll_id: str) -> None:
        try:
            response = self.session.delete(
                f"{self.base_url}/_search/scroll",
                timeout=self.timeout,
                json={"scroll_id": [scroll_id]},
            )
            response.raise_for_status()
        except Exception as error:
            raise ReconciliationError("OpenSearch scroll cleanup failed") from error


class DynamoAuthority:
    def __init__(
        self,
        sts_client: Any,
        dynamodb_client: Any,
        table_name: str,
        region: str,
        sleep: Callable[[float], None] = time.sleep,
        max_retries: int = 3,
    ) -> None:
        self.sts = sts_client
        self.dynamodb = dynamodb_client
        self.table_name = table_name
        self.region = region
        self.sleep = sleep
        self.max_retries = max_retries
        self.environment: Environment | None = None

    def discover_environment(self) -> Environment:
        try:
            identity = self.sts.get_caller_identity()
            table = self.dynamodb.describe_table(TableName=self.table_name)["Table"]
            environment = self._parse_environment(identity, table)
        except ReconciliationError:
            raise
        except Exception as error:
            raise ReconciliationError(
                "AWS identity or DynamoDB table discovery failed"
            ) from error
        self.environment = environment
        return environment

    def _parse_environment(
        self, identity: dict[str, Any], table: dict[str, Any]
    ) -> Environment:
        schema = table.get("KeySchema")
        definitions = table.get("AttributeDefinitions")
        if not isinstance(schema, list) or len(schema) != 1:
            raise ReconciliationError("DynamoDB table key schema is unsupported")
        key_name = schema[0].get("AttributeName")
        key_type = _attribute_type(definitions, key_name)
        account = identity.get("Account")
        table_arn = table.get("TableArn")
        if schema[0].get("KeyType") != "HASH" or key_type != "S":
            raise ReconciliationError("DynamoDB table key schema is unsupported")
        if not all(isinstance(value, str) and value for value in (account, table_arn)):
            raise ReconciliationError("AWS environment identity is malformed")
        self._validate_table_arn(table_arn, account)
        return Environment(
            account, self.region, table_arn, self.table_name, key_name, key_type
        )

    def _validate_table_arn(self, table_arn: str, account: str) -> None:
        parts = table_arn.split(":", 5)
        expected_resource = f"table/{self.table_name}"
        if (
            len(parts) != 6
            or parts[0] != "arn"
            or parts[2] != "dynamodb"
            or parts[3] != self.region
            or parts[4] != account
            or parts[5] != expected_resource
        ):
            raise ReconciliationError(
                "DynamoDB table ARN does not match the AWS environment"
            )

    def build_key(self, uid: str) -> dict[str, dict[str, str]]:
        if self.environment is None:
            raise ReconciliationError("DynamoDB environment is not initialized")
        return {self.environment.key_name: {self.environment.key_type: uid}}

    def classify(self, uids: Iterable[str]) -> dict[str, str]:
        unique = sorted(set(uids))
        if self.environment is None:
            raise ReconciliationError("DynamoDB environment is not initialized")
        active: set[str] = set()
        for offset in range(0, len(unique), 100):
            active.update(self._read_batch(unique[offset : offset + 100]))
        return {uid: ("active" if uid in active else "stale") for uid in unique}

    def _read_batch(self, uids: list[str]) -> set[str]:
        request = {
            self.table_name: {
                "Keys": [self.build_key(uid) for uid in uids],
                "ConsistentRead": True,
            }
        }
        active: set[str] = set()
        for attempt in range(self.max_retries + 1):
            response = self._batch_get(request)
            active.update(self._response_uids(response))
            request = response.get("UnprocessedKeys") or {}
            if not request:
                return active
            if attempt < self.max_retries:
                self.sleep(min(0.1 * (2**attempt), 1.0))
        raise ReconciliationError("DynamoDB left candidate keys unresolved")

    def _batch_get(self, request: dict[str, Any]) -> dict[str, Any]:
        try:
            response = self.dynamodb.batch_get_item(RequestItems=request)
        except Exception as error:
            raise ReconciliationError("DynamoDB batch read failed") from error
        if not isinstance(response, dict):
            raise ReconciliationError("DynamoDB returned malformed batch data")
        responses = response.get("Responses")
        unprocessed = response.get("UnprocessedKeys")
        if (
            not isinstance(responses, dict)
            or set(responses) != {self.table_name}
            or not isinstance(responses[self.table_name], list)
            or not isinstance(unprocessed, dict)
        ):
            raise ReconciliationError("DynamoDB returned incomplete batch data")
        self._validate_unprocessed_keys(request, unprocessed)
        return response

    def _validate_unprocessed_keys(
        self, request: dict[str, Any], unprocessed: dict[str, Any]
    ) -> None:
        if set(request) != {self.table_name} or not set(unprocessed) <= {
            self.table_name
        }:
            raise ReconciliationError("DynamoDB returned foreign unprocessed keys")
        if self.table_name not in unprocessed:
            return
        pending = unprocessed[self.table_name]
        if not isinstance(pending, dict) or not isinstance(pending.get("Keys"), list):
            raise ReconciliationError("DynamoDB returned malformed unprocessed keys")
        requested = {
            json.dumps(key, sort_keys=True) for key in request[self.table_name]["Keys"]
        }
        returned = {json.dumps(key, sort_keys=True) for key in pending["Keys"]}
        if not returned <= requested:
            raise ReconciliationError("DynamoDB returned unexpected unprocessed keys")

    def _response_uids(self, response: dict[str, Any]) -> set[str]:
        if self.environment is None:
            raise ReconciliationError("DynamoDB environment is not initialized")
        items = response["Responses"][self.table_name]
        result = set()
        for item in items:
            value = item.get(self.environment.key_name, {}).get(
                self.environment.key_type
            )
            if not isinstance(value, str) or not value:
                raise ReconciliationError("DynamoDB returned a malformed key")
            result.add(value)
        return result


def _attribute_type(definitions: Any, key_name: Any) -> str:
    if not isinstance(definitions, list) or not isinstance(key_name, str):
        raise ReconciliationError("DynamoDB key metadata is malformed")
    for definition in definitions:
        if definition.get("AttributeName") == key_name:
            value = definition.get("AttributeType")
            if isinstance(value, str):
                return value
    raise ReconciliationError("DynamoDB key type is missing")


async def snapshot_candidate(
    values: KVStore, mappings: KVStore, meeting_id: str, uid: str
) -> CandidateSnapshot:
    value_entry = await get_entry(values, registrant_key(uid))
    mapping_entry = await get_entry(mappings, mapping_key(uid))
    payload, encoding = decode_payload(value_entry.value)
    _validate_identity(payload, meeting_id, uid)
    state = mapping_state(mapping_entry.value)
    return CandidateSnapshot(
        uid=uid,
        classification="",
        revision=value_entry.revision,
        encoding=encoding,
        digest=payload_digest(value_entry.value),
        mapping_state=state,
        mapping_revision=mapping_entry.revision,
        deleted=DELETED_FIELD in payload,
    )


def _validate_identity(payload: dict[str, Any], meeting_id: str, uid: str) -> None:
    if str(payload.get("meeting_id", "")) != meeting_id:
        raise ReconciliationError(f"payload meeting mismatch for {uid}")
    registrant_id = payload.get("registrant_id")
    if registrant_id not in (None, "", uid):
        raise ReconciliationError(f"payload registrant mismatch for {uid}")


def write_plan(plan: Plan, path: Path) -> None:
    encoded = json.dumps(
        asdict(plan), sort_keys=True, indent=2, separators=(",", ": ")
    ).encode("utf-8")
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, "wb", closefd=False) as stream:
            stream.write(encoded)
            stream.write(b"\n")
            stream.flush()
            os.fsync(stream.fileno())
    finally:
        os.close(descriptor)


def read_plan(path: Path) -> Plan:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        environment = Environment(**data["environment"])
        candidates = [
            CandidateSnapshot(**candidate) for candidate in data["candidates"]
        ]
        plan = Plan(data["version"], data["meeting_id"], environment, candidates)
    except (OSError, KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        raise ReconciliationError("plan file is malformed or unreadable") from error
    if plan.version != PLAN_VERSION:
        raise ReconciliationError("plan version is unsupported")
    return plan


async def build_plan(
    meeting_id: str,
    environment: Environment,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
) -> Plan:
    uids = discovery.discover(meeting_id)
    classifications = authority.classify(uids)
    candidates = []
    for uid in uids:
        snapshot = await snapshot_candidate(values, mappings, meeting_id, uid)
        classification = classifications[uid]
        _validate_candidate_state(snapshot, classification)
        candidates.append(replace(snapshot, classification=classification))
    return Plan(PLAN_VERSION, meeting_id, environment, candidates)


def _validate_candidate_state(snapshot: CandidateSnapshot, classification: str) -> None:
    valid_active = (
        classification == "active"
        and not snapshot.deleted
        and snapshot.mapping_state == "live"
    )
    valid_stale_pending = (
        classification == "stale"
        and not snapshot.deleted
        and snapshot.mapping_state == "live"
    )
    valid_stale_marked = (
        classification == "stale"
        and snapshot.deleted
        and snapshot.mapping_state in {"live", "tombstoned"}
    )
    if not (valid_active or valid_stale_pending or valid_stale_marked):
        raise ReconciliationError(
            f"indexed candidate has ambiguous state: {snapshot.uid}"
        )


async def apply_plan(
    plan: Plan,
    confirmations: Confirmations,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
    now: Callable[[], datetime],
) -> list[str]:
    _validate_apply_confirmations(plan, confirmations)
    snapshots = await _revalidate_plan(plan, discovery, authority, values, mappings)
    completed = []
    for snapshot in snapshots:
        if snapshot.classification != "stale":
            continue
        try:
            await _validate_before_write(snapshot, authority, mappings)
            if not snapshot.deleted:
                await _soft_delete(plan.meeting_id, snapshot, values, now())
        except Exception as error:
            if isinstance(error, PlanDriftError) and not completed:
                raise
            pending = [
                item.uid
                for item in snapshots
                if item.classification == "stale" and item.uid not in completed
            ]
            raise ReconciliationError(
                f"partial apply; completed={completed}; pending={pending}"
            ) from error
        completed.append(snapshot.uid)
    return completed


async def _validate_before_write(
    snapshot: CandidateSnapshot, authority: Authority, mappings: KVStore
) -> None:
    if authority.classify([snapshot.uid]).get(snapshot.uid) != "stale":
        raise PlanDriftError(
            f"DynamoDB contains candidate before write: {snapshot.uid}"
        )
    mapping = await get_entry(mappings, mapping_key(snapshot.uid))
    if (
        mapping.revision != snapshot.mapping_revision
        or mapping_state(mapping.value) != snapshot.mapping_state
    ):
        raise PlanDriftError(f"mapping changed before write: {snapshot.uid}")


def _require_confirmation_match(
    expected: tuple[Any, ...], actual: tuple[Any, ...], message: str
) -> None:
    if actual != expected:
        raise PlanDriftError(message)


def _validate_apply_confirmations(plan: Plan, confirmations: Confirmations) -> None:
    stale_count = sum(
        1 for candidate in plan.candidates if candidate.classification == "stale"
    )
    expected = (
        plan.meeting_id,
        plan.environment.account,
        plan.environment.table_arn,
        stale_count,
    )
    actual = (
        confirmations.meeting_id,
        confirmations.aws_account,
        confirmations.table_arn,
        confirmations.stale_count,
    )
    _require_confirmation_match(
        expected, actual, "apply confirmations do not match the plan"
    )


async def _revalidate_plan(
    plan: Plan,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
) -> list[CandidateSnapshot]:
    uids = discovery.discover(plan.meeting_id)
    if uids != sorted(candidate.uid for candidate in plan.candidates):
        raise PlanDriftError("indexed candidates changed after dry-run")
    classifications = authority.classify(uids)
    current = []
    for uid in uids:
        snapshot = await snapshot_candidate(values, mappings, plan.meeting_id, uid)
        current.append(replace(snapshot, classification=classifications[uid]))
    if current != plan.candidates:
        raise PlanDriftError("authoritative or NATS state changed after dry-run")
    return current


async def _soft_delete(
    meeting_id: str,
    snapshot: CandidateSnapshot,
    values: KVStore,
    marked_at: datetime,
) -> None:
    entry = await get_entry(values, registrant_key(snapshot.uid))
    if entry.revision != snapshot.revision:
        raise RevisionConflictError(snapshot.uid)
    payload, encoding = decode_payload(entry.value)
    _validate_identity(payload, meeting_id, snapshot.uid)
    payload[DELETED_FIELD] = _rfc3339(marked_at)
    try:
        await values.update(
            registrant_key(snapshot.uid),
            encode_payload(payload, encoding),
            last=snapshot.revision,
        )
    except Exception as error:
        if isinstance(error, RevisionConflictError):
            raise
        raise ReconciliationError(
            f"NATS KV update failed for {snapshot.uid}"
        ) from error


async def restore_candidate(
    meeting_id: str,
    uid: str,
    environment: Environment,
    confirmations: RestoreConfirmations,
    authority: Authority,
    values: KVStore,
) -> None:
    _require_confirmation_match(
        (meeting_id, environment.account, environment.table_arn),
        (confirmations.meeting_id, confirmations.aws_account, confirmations.table_arn),
        "restore confirmations do not match",
    )
    if authority.classify([uid]).get(uid) != "active":
        raise ReconciliationError("DynamoDB does not contain the restore UID")
    entry = await get_entry(values, registrant_key(uid))
    payload, encoding = decode_payload(entry.value)
    _validate_identity(payload, meeting_id, uid)
    if DELETED_FIELD not in payload:
        raise ReconciliationError("restore UID is not soft-deleted")
    del payload[DELETED_FIELD]
    try:
        await values.update(
            registrant_key(uid),
            encode_payload(payload, encoding),
            last=entry.revision,
        )
    except Exception as error:
        if isinstance(error, RevisionConflictError):
            raise
        raise ReconciliationError(f"NATS KV restore failed for {uid}") from error


async def verify_apply(
    meeting_id: str,
    uids: list[str],
    discovery: Discovery,
    mappings: KVStore,
    timeout: float,
    interval: float,
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> None:
    deadline = time.monotonic() + timeout
    while True:
        incomplete = []
        for uid in uids:
            if discovery.contains(meeting_id, uid):
                incomplete.append(uid)
                continue
            entry = await get_entry(mappings, mapping_key(uid))
            if mapping_state(entry.value) != "tombstoned":
                incomplete.append(uid)
        if not incomplete:
            return
        if time.monotonic() >= deadline:
            raise VerificationError(f"apply verification incomplete: {incomplete}")
        await sleep(interval)


async def verify_restore(
    meeting_id: str,
    uid: str,
    discovery: Discovery,
    mappings: KVStore,
    timeout: float,
    interval: float,
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> None:
    deadline = time.monotonic() + timeout
    while True:
        entry = await get_entry(mappings, mapping_key(uid))
        if discovery.contains(meeting_id, uid) and mapping_state(entry.value) == "live":
            return
        if time.monotonic() >= deadline:
            raise VerificationError(f"restore verification incomplete: {uid}")
        await sleep(interval)


def build_aws_clients(config: Config) -> tuple[Any, Any]:
    if boto3 is None:
        raise ReconciliationError("boto3 is not installed")
    session = boto3.Session(
        profile_name=config.aws_profile, region_name=config.aws_region
    )
    if config.assume_role_arn:
        session = _assume_role(session, config)
    return session.client("sts"), session.client("dynamodb")


def _assume_role(session: Any, config: Config) -> Any:
    try:
        response = session.client("sts").assume_role(
            RoleArn=config.assume_role_arn,
            RoleSessionName="meeting-registrant-reconciliation",
        )
        credentials = response["Credentials"]
        return boto3.Session(
            aws_access_key_id=credentials["AccessKeyId"],
            aws_secret_access_key=credentials["SecretAccessKey"],
            aws_session_token=credentials["SessionToken"],
            region_name=config.aws_region,
        )
    except Exception as error:
        raise ReconciliationError("AWS role assumption failed") from error


async def run(config: Config) -> dict[str, Any]:
    if requests is None:
        raise ReconciliationError("requests is not installed")
    sts, dynamodb = build_aws_clients(config)
    authority = DynamoAuthority(sts, dynamodb, config.dynamodb_table, config.aws_region)
    environment = authority.discover_environment()
    discovery = OpenSearchDiscovery(
        requests.Session(),
        config.opensearch_url,
        config.opensearch_index,
        config.request_timeout,
    )
    connection, values, mappings = await open_nats(
        config.nats_url, config.request_timeout
    )
    try:
        if config.mode == "dry-run":
            return await _run_dry(
                config, environment, discovery, authority, values, mappings
            )
        if config.mode == "apply":
            return await _run_apply(
                config, environment, discovery, authority, values, mappings
            )
        return await _run_restore(
            config, environment, discovery, authority, values, mappings
        )
    finally:
        await connection.close()


async def _run_dry(
    config: Config,
    environment: Environment,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
) -> dict[str, Any]:
    plan = await build_plan(
        config.meeting_id, environment, discovery, authority, values, mappings
    )
    write_plan(plan, config.plan_path)
    return _summary("dry-run", plan.candidates)


async def _run_apply(
    config: Config,
    environment: Environment,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
) -> dict[str, Any]:
    plan = read_plan(config.plan_path)
    if config.meeting_id != plan.meeting_id:
        raise PlanDriftError("target meeting differs from the reviewed plan")
    if environment != plan.environment:
        raise PlanDriftError("AWS environment differs from the plan")
    confirmations = Confirmations(
        config.confirm_meeting_id or "",
        config.confirm_aws_account or "",
        config.confirm_table_arn or "",
        config.confirm_stale_count if config.confirm_stale_count is not None else -1,
    )
    completed = await apply_plan(
        plan,
        confirmations,
        discovery,
        authority,
        values,
        mappings,
        now=lambda: datetime.now(timezone.utc),
    )
    await verify_apply(
        plan.meeting_id,
        completed,
        discovery,
        mappings,
        config.verify_timeout,
        config.verify_interval,
    )
    return {"mode": "apply", "meeting_id": plan.meeting_id, "completed": completed}


async def _run_restore(
    config: Config,
    environment: Environment,
    discovery: Discovery,
    authority: Authority,
    values: KVStore,
    mappings: KVStore,
) -> dict[str, Any]:
    uid = config.restore_uid or ""
    confirmations = RestoreConfirmations(
        config.confirm_meeting_id or "",
        config.confirm_aws_account or "",
        config.confirm_table_arn or "",
    )
    await restore_candidate(
        config.meeting_id, uid, environment, confirmations, authority, values
    )
    await verify_restore(
        config.meeting_id,
        uid,
        discovery,
        mappings,
        config.verify_timeout,
        config.verify_interval,
    )
    return {"mode": "restore", "meeting_id": config.meeting_id, "completed": [uid]}


def _summary(mode: str, candidates: list[CandidateSnapshot]) -> dict[str, Any]:
    active = [
        candidate.uid
        for candidate in candidates
        if candidate.classification == "active"
    ]
    stale = [
        candidate.uid for candidate in candidates if candidate.classification == "stale"
    ]
    return {
        "mode": mode,
        "candidate_count": len(candidates),
        "active_count": len(active),
        "stale_count": len(stale),
        "active_uids": active,
        "stale_uids": stale,
    }


def _required_string(payload: dict[str, Any], name: str) -> str:
    value = payload.get(name)
    if not isinstance(value, str) or not value:
        raise ReconciliationError(f"response lacks {name}")
    return value


def _rfc3339(value: datetime) -> str:
    return (
        value.astimezone(timezone.utc)
        .isoformat(timespec="seconds")
        .replace("+00:00", "Z")
    )


def main(argv: list[str] | None = None) -> int:
    try:
        result = asyncio.run(run(parse_args(argv)))
        print(json.dumps(result, sort_keys=True))
        return 0
    except ReconciliationError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
