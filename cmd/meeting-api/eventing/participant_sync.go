// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// participantConvertFn is the shared signature of convertMapToInviteeParticipantData
// and convertMapToAttendeeParticipantData.
type participantConvertFn func(
	ctx context.Context,
	v1Data map[string]interface{},
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingParticipantEventData, error)

// participantUpdateConfig carries the side-specific behaviour for syncParticipantUpdate.
// Invitee and attendee sides share the same coordination logic; they differ only in the
// string prefixes and the three function fields.
type participantUpdateConfig struct {
	// ownXrefPrefix is the side label used in own KV xref keys ("invitee" or "attendee").
	ownXrefPrefix string
	// siblingXrefPrefix is the side label used in sibling KV xref keys ("attendee" or "invitee").
	siblingXrefPrefix string
	// mappingKeyPrefix is the prefix for the per-record mapping key
	// (e.g. "v1_past_meeting_invitees" or "v1_past_meeting_attendees").
	mappingKeyPrefix string
	// siblingObjectPrefix is the NATS KV object bucket prefix for the sibling record type
	// (e.g. "itx-zoom-past-meetings-attendees." or "itx-zoom-past-meetings-invitees.").
	siblingObjectPrefix string

	// convert decodes own v1Data into a PastMeetingParticipantEventData.
	convert participantConvertFn
	// siblingConvert decodes the sibling's v1Data for partial-update publishes.
	siblingConvert participantConvertFn
	// mergeSibling is called when the sibling's xref exists, allowing the own-side record
	// to reflect the sibling's presence (e.g. set IsAttended=true and copy attendee-only
	// fields). The siblingID is the raw value stored in the sibling's xref entry.
	mergeSibling func(ctx context.Context, self *models.PastMeetingParticipantEventData, siblingID string) error
	// setSiblingFlags adjusts IsInvited/IsAttended on the sibling record before publishing
	// a partial member_put when the username has changed.
	setSiblingFlags func(sibling *models.PastMeetingParticipantEventData)
}

// participantDeleteConfig carries the side-specific behaviour for syncParticipantDelete.
// The delete path does not need to decode own v1Data — only the surviving sibling's data.
type participantDeleteConfig struct {
	ownXrefPrefix       string
	siblingXrefPrefix   string
	mappingKeyPrefix    string
	siblingObjectPrefix string

	// siblingConvert decodes the surviving sibling's v1Data for partial-delete publishes.
	siblingConvert participantConvertFn
	// setSiblingFlags adjusts IsInvited/IsAttended on the sibling record before publishing
	// the partial-delete indexer update.
	setSiblingFlags func(sibling *models.PastMeetingParticipantEventData)
}

// syncParticipantUpdate is the shared implementation for handlePastMeetingInviteeUpdate and
// handlePastMeetingAttendeeUpdate. All side-specific behaviour is carried by cfg; the
// coordination logic (sibling merge, username-change revocation, mapping storage) is identical
// for both sides.
func (h *EventHandlers) syncParticipantUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	cfg participantUpdateConfig,
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_"+cfg.ownXrefPrefix)
	funcLogger.DebugContext(ctx, "processing past meeting participant update")

	// Decode own v1Data into a participant record.
	participantData, err := cfg.convert(ctx, v1Data, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to participant")
		return isTransientError(err)
	}

	// Validate required fields.
	if participantData.UID == "" || participantData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in participant data")
		return false
	}
	if participantData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping participant sync - parent project not found in mappings")
		return false
	}
	funcLogger = funcLogger.With("participant_uid", participantData.UID)
	funcLogger.InfoContext(ctx, "processing past meeting participant update")

	// Merge sibling — if the other side's xref exists, reflect its presence in our record
	// so a late-arriving update on one side doesn't reset a flag the other side already set.
	// Distinguish ErrKeyNotFound (sibling absent) from transient errors: a transient failure
	// here would publish an incorrect IsAttended/IsInvited flag and corrupt FGA relations.
	if participantData.Username != "" {
		siblingXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.siblingXrefPrefix, participantData.MeetingAndOccurrenceID, participantData.Username)
		entry, err := h.v1MappingsKV.Get(ctx, siblingXrefKey)
		if err != nil {
			if !isKVAbsenceError(err) {
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error reading sibling xref for merge, will retry")
				return true
			}
			// ErrKeyNotFound or ErrInvalidKey: no xref exists or can exist — skip merge.
		} else if !entryIsTombstoned(entry) {
			if err := cfg.mergeSibling(ctx, participantData, string(entry.Value())); err != nil {
				// mergeSibling only returns errors for retryable sibling-object reads
				// (non-ErrKeyNotFound from v1ObjectsKV.Get). Always retry: isTransientError
				// misses context.DeadlineExceeded and other errors whose messages don't
				// match its keyword list, which would silently ACK without publishing.
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error in mergeSibling, will retry")
				return true
			}
		}
	}

	// Determine created vs updated and recover the previously-stored username.
	// Distinguish ErrKeyNotFound (first-time create) from transient errors: a transient failure
	// here would make us treat this as a create, overwrite the mapping, and permanently lose the
	// old username — exactly the stale-access condition we are trying to prevent.
	mappingKey := fmt.Sprintf("%s.%s", cfg.mappingKeyPrefix, participantData.UID)
	indexerAction := indexerConstants.ActionCreated
	oldUsername := ""
	if entry, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
		_, oldUsername, _ = parseRegistrantMappingValue(string(entry.Value()))
	} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error reading participant mapping, will retry")
		return true
	}

	// Handle username change: revoke old access before publishing the new state.
	if indexerAction == indexerConstants.ActionUpdated && oldUsername != "" && oldUsername != participantData.Username {
		// Check if a sibling record still grants the old username access.
		oldSiblingXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.siblingXrefPrefix, participantData.MeetingAndOccurrenceID, oldUsername)
		siblingEntry, xrefErr := h.v1MappingsKV.Get(ctx, oldSiblingXrefKey)
		if xrefErr != nil && !isKVAbsenceError(xrefErr) {
			funcLogger.With(logging.ErrKey, xrefErr).WarnContext(ctx, "transient error checking sibling xref, will retry")
			return true
		}
		// ErrKeyNotFound or ErrInvalidKey both mean no sibling xref exists.
		siblingExists := xrefErr == nil && !entryIsTombstoned(siblingEntry)

		if siblingExists {
			// A sibling record still grants access — send a partial member_put so stale
			// own-side relations are cleared while sibling-side access is preserved.
			siblingID := string(siblingEntry.Value())
			siblingObjEntry, siblingErr := h.v1ObjectsKV.Get(ctx, cfg.siblingObjectPrefix+siblingID)
			if siblingErr != nil {
				if !errors.Is(siblingErr, jetstream.ErrKeyNotFound) {
					funcLogger.With(logging.ErrKey, siblingErr).WarnContext(ctx, "transient error fetching sibling for partial update, will retry")
					return true
				}
				siblingExists = false
			} else {
				siblingData, decErr := decodeData(siblingObjEntry.Value())
				if decErr != nil {
					// The sibling xref is active and the object was fetched successfully, so a
					// decode failure does not mean the sibling stopped granting access. Falling
					// through to member_remove would incorrectly revoke the surviving relation.
					// Retry instead, matching partialParticipantDelete.
					funcLogger.With(logging.ErrKey, decErr).WarnContext(ctx, "failed to decode sibling for partial update, will retry")
					return true
				} else if siblingParticipant, convErr := cfg.siblingConvert(ctx, siblingData, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger); convErr != nil {
					// Same reasoning: cannot safely issue member_remove when the sibling xref
					// is active, regardless of whether the conversion error is transient or
					// permanent. Always retry to preserve the sibling's FGA access.
					funcLogger.With(logging.ErrKey, convErr).WarnContext(ctx, "failed to convert sibling for partial update, will retry")
					return true
				} else {
					cfg.setSiblingFlags(siblingParticipant)
					if pubErr := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionUpdated), siblingParticipant); pubErr != nil {
						funcLogger.With(logging.ErrKey, pubErr).WarnContext(ctx, "failed to publish partial member_put for old username")
						return isTransientError(pubErr)
					}
				}
			}
		}
		if !siblingExists {
			// No sibling record survives — fully revoke the old username's access.
			payload, err := buildGenericMemberRemovePayload("v1_past_meeting", participantData.MeetingAndOccurrenceID, oldUsername)
			if err != nil {
				funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to build member remove payload for old username")
				return false
			}
			if err := h.publisher.PublishAccessDelete(ctx, fgaconstants.GenericMemberRemoveSubject, payload); err != nil {
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to publish FGA remove for old username")
				return isTransientError(err)
			}
		}
		// Tombstone the stale own-side cross-reference so sibling handlers don't match
		// the old username on subsequent events.
		oldOwnXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.ownXrefPrefix, participantData.MeetingAndOccurrenceID, oldUsername)
		if _, err := h.v1MappingsKV.Put(ctx, oldOwnXrefKey, []byte(tombstoneMarker)); err != nil {
			if errors.Is(err, jetstream.ErrInvalidKey) {
				// oldUsername contains invalid KV key characters; the xref can never have
				// existed, so the tombstone is a no-op — continue to publish the new event.
				funcLogger.WarnContext(ctx, "skipping tombstone for old xref: username contains invalid KV key characters")
			} else {
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error tombstoning old xref, will retry")
				return true
			}
		}
	}

	// Publish to indexer and FGA-sync.
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerAction), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish participant event")
		return isTransientError(err)
	}

	// Store uid+username+meetingAndOccurrenceID so future updates and deletes can recover
	// them without an extra lookup. Retry transient write failures.
	mappingValue := buildRegistrantMappingValue(participantData.UID, participantData.Username, participantData.MeetingAndOccurrenceID)
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte(mappingValue)); err != nil {
		// Always retry: isTransientError misses context.DeadlineExceeded and similar errors
		// whose messages don't match its keyword list. Without this mapping, future
		// username-change events and hard deletes cannot recover the old username, breaking
		// stale-access revocation. ErrInvalidKey cannot occur here (key is uid-only, no
		// special characters).
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store participant mapping, will retry")
		return true
	}
	// Write the new-username cross-reference so sibling handlers can find this record.
	// ErrInvalidKey (username contains characters outside [-a-zA-Z0-9_.]) is accepted: the
	// xref simply cannot be written, and sibling merges for this participant will be skipped.
	// Any other failure is transient — retry so the xref is not silently lost.
	if participantData.Username != "" {
		ownXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.ownXrefPrefix, participantData.MeetingAndOccurrenceID, participantData.Username)
		if _, err := h.v1MappingsKV.Put(ctx, ownXrefKey, []byte(participantData.UID)); err != nil {
			if errors.Is(err, jetstream.ErrInvalidKey) {
				funcLogger.WarnContext(ctx, "skipping xref write: username contains invalid KV key characters")
			} else {
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error storing participant cross-reference mapping, will retry")
				return true
			}
		}
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting participant", "action", string(indexerAction))
	return false
}

// syncParticipantDelete is the shared implementation for handlePastMeetingInviteeDelete and
// handlePastMeetingAttendeeDelete. All side-specific behaviour is carried by cfg.
func (h *EventHandlers) syncParticipantDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
	id string,
	cfg participantDeleteConfig,
) (retry bool) {
	funcLogger := h.logger.With("key", key, cfg.ownXrefPrefix+"_id", id)

	mappingKey := fmt.Sprintf("%s.%s", cfg.mappingKeyPrefix, id)
	if h.isTombstoned(ctx, mappingKey) {
		funcLogger.DebugContext(ctx, "participant delete already processed, skipping")
		return false
	}
	funcLogger.InfoContext(ctx, "processing past meeting participant delete")

	// Recover username and meetingAndOccurrenceID from v1Data (soft deletes) or from the
	// rich mapping written by the update handler (hard NATS deletes where v1Data is nil).
	var username, meetingAndOccurrenceID string
	if v1Data == nil {
		if entry, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
			if !entryIsTombstoned(entry) {
				_, username, meetingAndOccurrenceID = parseRegistrantMappingValue(string(entry.Value()))
			}
		} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error reading participant mapping on delete, will retry")
			return true
		}
	} else {
		username = utils.GetString(v1Data["lf_sso"])
		meetingAndOccurrenceID = utils.GetString(v1Data["meeting_and_occurrence_id"])
	}

	// Check if a sibling record still exists — if so, apply a partial delete that
	// preserves the sibling's access rather than revoking it entirely.
	// Distinguish ErrKeyNotFound (no sibling) from transient errors: a transient failure
	// would fall through to fullParticipantDelete and incorrectly revoke the sibling's access.
	if username != "" && meetingAndOccurrenceID != "" {
		siblingXrefKey := fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.siblingXrefPrefix, meetingAndOccurrenceID, username)
		entry, err := h.v1MappingsKV.Get(ctx, siblingXrefKey)
		if err != nil {
			if !isKVAbsenceError(err) {
				funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error reading sibling xref on delete, will retry")
				return true
			}
			// ErrKeyNotFound or ErrInvalidKey: no xref exists or can exist — treat as no sibling.
		} else if !entryIsTombstoned(entry) {
			survivingID := string(entry.Value())
			funcLogger.DebugContext(ctx, "participant has active sibling record; applying partial delete",
				"surviving_sibling_id", survivingID)
			return h.partialParticipantDelete(ctx, funcLogger, key, id, survivingID, meetingAndOccurrenceID, username, cfg)
		}
	}

	// Full delete — no sibling record survives.
	return h.fullParticipantDelete(ctx, funcLogger, key, id, meetingAndOccurrenceID, username, cfg)
}

// fullParticipantDelete sends a full indexer delete and FGA member_remove for a participant
// when no sibling record survives.
func (h *EventHandlers) fullParticipantDelete(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, id, meetingAndOccurrenceID, username string,
	cfg participantDeleteConfig,
) (retry bool) {
	var accessPayload []byte
	var deleteAccessSubject string
	if username != "" {
		var err error
		if accessPayload, err = buildGenericMemberRemovePayload("v1_past_meeting", meetingAndOccurrenceID, username); err != nil {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to build member remove payload")
			return false
		}
		deleteAccessSubject = fgaconstants.GenericMemberRemoveSubject
	} else {
		funcLogger.DebugContext(ctx, "no username available, skipping access control message for participant delete")
	}

	result := h.handleMeetingTypeDelete(ctx, key, id, accessPayload, meetingDeleteConfig{
		indexerSubject:      "lfx.index.v1_past_meeting_participant",
		deleteAccessSubject: deleteAccessSubject,
		tombstoneKeyFmts:    []string{cfg.mappingKeyPrefix + ".%s"},
	})
	if !result && username != "" && meetingAndOccurrenceID != "" {
		if err := h.tombstoneMapping(ctx, fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
			cfg.ownXrefPrefix, meetingAndOccurrenceID, username)); err != nil {
			return true
		}
	}
	return result
}

// partialParticipantDelete handles the case where a participant record is deleted but a sibling
// record (the other side) still exists. It sends an indexer UPDATE with the sibling's state so
// the participant retains access from their surviving record.
func (h *EventHandlers) partialParticipantDelete(
	ctx context.Context,
	funcLogger *slog.Logger,
	key, id, survivingID, meetingAndOccurrenceID, username string,
	cfg participantDeleteConfig,
) (retry bool) {
	// Fetch the surviving sibling's data to build an accurate participant record.
	siblingEntry, err := h.v1ObjectsKV.Get(ctx, cfg.siblingObjectPrefix+survivingID)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "transient error fetching sibling data for partial delete")
			return true
		}
		// Sibling is gone — fall back to a full delete.
		funcLogger.WarnContext(ctx, "surviving sibling not found during partial delete; falling back to full delete",
			"surviving_sibling_id", survivingID)
		return h.fullParticipantDelete(ctx, funcLogger, key, id, meetingAndOccurrenceID, username, cfg)
	}

	siblingData, err := decodeData(siblingEntry.Value())
	if err != nil {
		// Corrupt sibling payload: we cannot determine what relations the sibling grants,
		// so falling back to fullParticipantDelete would wrongly emit member_remove and
		// revoke the surviving sibling's FGA access (violates fga-contract.md §partial-delete).
		// Retry instead: self-heals on transient corruption; for permanent corruption the
		// event is dropped after max-delivery without incorrectly revoking access.
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to decode sibling data for partial delete, will retry")
		return true
	}

	participantData, err := cfg.siblingConvert(ctx, siblingData, h.userLookup, h.idMapper, h.v1ObjectsKV, funcLogger)
	if err != nil {
		// Same reasoning as decodeData failure above: we cannot safely issue member_remove
		// when a sibling xref exists, regardless of whether the error is transient or permanent.
		// Always retry to preserve the sibling's FGA access.
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to convert sibling data for partial delete, will retry")
		return true
	}
	cfg.setSiblingFlags(participantData)

	if err := h.publisher.PublishIndexerDelete(ctx, "lfx.index.v1_past_meeting_participant", id); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send indexer delete for partial delete")
		return isTransientError(err)
	}
	if err := h.publisher.PublishPastMeetingParticipantEvent(ctx, string(indexerConstants.ActionUpdated), participantData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to send partial delete indexer update")
		return isTransientError(err)
	}

	// Tombstone own mapping and xref; the sibling's records remain active.
	if err := h.tombstoneMapping(ctx, fmt.Sprintf("%s.%s", cfg.mappingKeyPrefix, id)); err != nil {
		return true
	}
	if err := h.tombstoneMapping(ctx, fmt.Sprintf("v1_participant_by_meeting_user.%s.%s.%s",
		cfg.ownXrefPrefix, meetingAndOccurrenceID, username)); err != nil {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully applied partial participant delete (sibling record remains active)")
	return false
}
