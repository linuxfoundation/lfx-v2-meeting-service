-- Copyright The Linux Foundation and each contributor to LFX.
-- SPDX-License-Identifier: MIT

CREATE SEQUENCE IF NOT EXISTS meeting_event_state_revision;

CREATE TABLE IF NOT EXISTS meeting_event_state_cutover (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    source_last_sequence bigint NOT NULL,
    verified_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS meeting_resource_sync_state (
    resource_kind text NOT NULL,
    source_id text NOT NULL,
    state text NOT NULL CHECK (state IN ('empty', 'processed', 'deleted', 'legacy', 'principal')),
    source_uid text NOT NULL DEFAULT '',
    username text NOT NULL DEFAULT '',
    parent_meeting_id text NOT NULL DEFAULT '',
    principal_authoritative boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (resource_kind, source_id)
);

-- The map contains only association IDs, committee IDs, and voting filters,
-- not arbitrary KV payloads. It is one CAS-protected aggregate per parent.
CREATE TABLE IF NOT EXISTS meeting_committee_associations (
    parent_kind text NOT NULL,
    parent_id text NOT NULL,
    deleted boolean NOT NULL DEFAULT false,
    associations jsonb NOT NULL DEFAULT '{}'::jsonb,
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (parent_kind, parent_id)
);

CREATE TABLE IF NOT EXISTS meeting_summary_associations (
    past_meeting_id text PRIMARY KEY,
    summary_ids text[],
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS meeting_participant_lookup (
    participant_kind text NOT NULL CHECK (participant_kind IN ('invitee', 'attendee')),
    past_meeting_id text NOT NULL,
    username text NOT NULL,
    participant_id text NOT NULL,
    deleted boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (participant_kind, past_meeting_id, username)
);

CREATE TABLE IF NOT EXISTS meeting_invitation_guards (
    registrant_id text PRIMARY KEY,
    invite_id text NOT NULL,
    dispatch_pending boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Historical participant aliases are not new allocations. Both directions
-- preserve the old API's invitee/attendee addressing across the migration.
CREATE TABLE IF NOT EXISTS meeting_participant_aliases (
    alias_kind text NOT NULL CHECK (alias_kind IN ('invitee_id', 'attendee_id', 'participant_to_invitee_id', 'participant_to_attendee_id')),
    source_id text NOT NULL,
    target_id text NOT NULL,
    deleted boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT nextval('meeting_event_state_revision'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (alias_kind, source_id)
);
