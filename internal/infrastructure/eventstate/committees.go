// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"errors"
)

func validParent(kind string) error {
	if kind != "meeting" && kind != "past_meeting" {
		return errors.New("unsupported committee association parent")
	}
	return nil
}

func (s *Store) UpsertCommitteeAssociation(ctx context.Context, kind, parentID, associationID, committeeID string, filters []string) error {
	if err := validParent(kind); err != nil {
		return err
	}
	value, err := json.Marshal(map[string]committeeAssociation{associationID: {CommitteeID: committeeID, Filters: filters}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err = s.pool.Exec(ctx, `INSERT INTO meeting_committee_associations(parent_kind,parent_id,associations) VALUES($1,$2,$3)
	ON CONFLICT(parent_kind,parent_id) DO UPDATE SET associations =
	(CASE WHEN meeting_committee_associations.deleted THEN '{}'::jsonb ELSE coalesce(nullif(meeting_committee_associations.associations,'null'::jsonb),'{}'::jsonb) END) || EXCLUDED.associations,
	deleted=false,version=nextval('meeting_event_state_revision'),updated_at=now()`, kind, parentID, value)
	return databaseError(err)
}

func (s *Store) RemoveCommitteeAssociation(ctx context.Context, kind, parentID, associationID string) error {
	if err := validParent(kind); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, `UPDATE meeting_committee_associations SET associations=coalesce(nullif(associations,'null'::jsonb),'{}'::jsonb)-$3::text,
	version=nextval('meeting_event_state_revision'),updated_at=now() WHERE parent_kind=$1 AND parent_id=$2 AND NOT deleted`, kind, parentID, associationID)
	return databaseError(err)
}
