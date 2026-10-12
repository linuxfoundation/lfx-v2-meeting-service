// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// IDMapper defines the interface for mapping between LFX v1 and v2 identifiers
type IDMapper interface {
	// MapProjectV2ToV1 maps a v2 project UID to v1 project SFID
	MapProjectV2ToV1(ctx context.Context, v2UID string) (string, error)

	// MapProjectV1ToV2 maps a v1 project SFID to v2 project UID
	MapProjectV1ToV2(ctx context.Context, v1SFID string) (string, error)

	// MapCommitteeV2ToV1 maps a v2 committee UID to v1 committee identifiers (project_sfid:committee_sfid)
	MapCommitteeV2ToV1(ctx context.Context, v2UID string) (string, error)

	// MapCommitteeV1ToV2 maps a v1 committee SFID to v2 committee UID
	MapCommitteeV1ToV2(ctx context.Context, v1SFID string) (string, error)

	// MapInviteeIDToParticipantV2 maps ITX invitee ID to V2 participant ID
	MapInviteeIDToParticipantV2(ctx context.Context, inviteeID string) (string, error)

	// MapAttendeeIDToParticipantV2 maps ITX attendee ID to V2 participant ID
	MapAttendeeIDToParticipantV2(ctx context.Context, attendeeID string) (string, error)

	// MapParticipantV2ToInviteeID maps a V2 participant ID to ITX invitee ID
	MapParticipantV2ToInviteeID(ctx context.Context, v2ParticipantID string) (string, error)

	// MapParticipantV2ToAttendeeID maps a V2 participant ID to ITX attendee ID
	MapParticipantV2ToAttendeeID(ctx context.Context, v2ParticipantID string) (string, error)
}

// CommitteeProjectResolver is implemented by IDMappers that can report which v1
// project owns a committee. It is optional: mappers without ownership data
// (e.g. the no-op mapper) do not implement it, and callers that need ownership
// must treat its absence as "cannot verify".
type CommitteeProjectResolver interface {
	// MapCommitteeV2ToV1Project returns the v1 project SFID that owns the
	// committee identified by v2UID. An empty result with a nil error means the
	// mapping did not include the owning project.
	MapCommitteeV2ToV1Project(ctx context.Context, v2UID string) (string, error)
}
