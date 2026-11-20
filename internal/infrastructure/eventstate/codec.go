// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

type principal struct {
	UID           string `json:"uid"`
	Username      string `json:"username"`
	MeetingID     string `json:"meeting_id"`
	Authoritative bool   `json:"principal_authoritative,omitempty"`
}

type committeeAssociation struct {
	CommitteeID string   `json:"committee_id"`
	Filters     []string `json:"committee_filters,omitempty"`
}

// binding uses only constant table/column identifiers; all data is parameterized.
type binding struct {
	table   string
	keys    []string
	args    []any
	columns []string
}

var resourceKinds = map[string]bool{
	"v1_meetings": true, "v1_meeting_registrants": true,
	"v1_invite_responses": true, "v1_meeting_attachments": true,
	"v1_past_meetings": true, "v1_past_meeting_invitees": true,
	"v1_past_meeting_attendees": true, "v1_past_meeting_recordings": true,
	"v1_past_meeting_summaries": true, "v1_past_meeting_attachments": true,
}

func bind(key string) (binding, error) {
	if key == "" || strings.ContainsAny(key, " \t\r\n*>\\") {
		return binding{}, jetstream.ErrInvalidKey
	}
	if rest, ok := strings.CutPrefix(key, "past_meeting_participant."); ok {
		kind, id, ok := strings.Cut(rest, ".")
		if !ok || id == "" || (kind != "invitee_id" && kind != "attendee_id" && kind != "participant_to_invitee_id" && kind != "participant_to_attendee_id") {
			return binding{}, errors.New("unrecognized participant alias family")
		}
		return binding{"meeting_participant_aliases", []string{"alias_kind", "source_id"}, []any{kind, id}, []string{"target_id", "deleted"}}, nil
	}
	for prefix, kind := range map[string]string{
		"v1-mappings.meeting-mappings.":      "meeting",
		"v1-mappings.past-meeting-mappings.": "past_meeting",
	} {
		if id, ok := strings.CutPrefix(key, prefix); ok && id != "" {
			return binding{"meeting_committee_associations", []string{"parent_kind", "parent_id"}, []any{kind, id}, []string{"deleted", "associations"}}, nil
		}
	}
	if id, ok := strings.CutPrefix(key, "v1-mappings.past-meeting-summaries."); ok && id != "" {
		return binding{"meeting_summary_associations", []string{"past_meeting_id"}, []any{id}, []string{"summary_ids"}}, nil
	}
	if rest, ok := strings.CutPrefix(key, "v1_participant_by_meeting_user."); ok {
		parts := strings.SplitN(rest, ".", 3)
		if len(parts) != 3 || (parts[0] != "invitee" && parts[0] != "attendee") || parts[1] == "" || parts[2] == "" {
			return binding{}, jetstream.ErrInvalidKey
		}
		return binding{"meeting_participant_lookup", []string{"participant_kind", "past_meeting_id", "username"}, []any{parts[0], parts[1], parts[2]}, []string{"participant_id", "deleted"}}, nil
	}
	if id, ok := strings.CutPrefix(key, "v1_meeting_registrant_lfid_invite_sent."); ok && id != "" {
		return binding{"meeting_invitation_guards", []string{"registrant_id"}, []any{id}, []string{"invite_id", "dispatch_pending"}}, nil
	}
	kind, id, ok := strings.Cut(key, ".")
	if !ok || id == "" || !resourceKinds[kind] {
		return binding{}, errors.New("unrecognized meeting state family")
	}
	return binding{"meeting_resource_sync_state", []string{"resource_kind", "source_id"}, []any{kind, id}, []string{"state", "source_uid", "username", "parent_meeting_id", "principal_authoritative"}}, nil
}

func (b binding) decode(value []byte) ([]any, error) {
	s := string(value)
	switch b.table {
	case "meeting_resource_sync_state":
		state := "legacy"
		p := principal{UID: s}
		switch s {
		case "":
			state = "empty"
			p.UID = ""
		case "1":
			state = "processed"
			p.UID = ""
		case "!del":
			state = "deleted"
			p.UID = ""
		default:
			if strings.HasPrefix(s, "{") {
				if err := json.Unmarshal(value, &p); err != nil {
					return nil, err
				}
				state = "principal"
			} else if parts := strings.SplitN(s, "|", 3); len(parts) == 3 {
				p = principal{UID: parts[0], Username: parts[1], MeetingID: parts[2]}
				state = "principal"
			}
		}
		return []any{state, p.UID, p.Username, p.MeetingID, p.Authoritative}, nil
	case "meeting_committee_associations":
		if s == "!del" {
			return []any{true, []byte("{}")}, nil
		}
		var links map[string]committeeAssociation
		dec := json.NewDecoder(bytes.NewReader(value))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&links); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(links)
		return []any{false, encoded}, err
	case "meeting_summary_associations":
		var ids []string
		if err := json.Unmarshal(value, &ids); err != nil {
			return nil, err
		}
		return []any{ids}, nil
	case "meeting_participant_lookup", "meeting_participant_aliases":
		if s == "!del" {
			return []any{"", true}, nil
		}
		return []any{s, false}, nil
	case "meeting_invitation_guards":
		if s == "pending" {
			return []any{"", true}, nil
		}
		return []any{s, false}, nil
	default:
		return nil, errors.New("unsupported state table")
	}
}
