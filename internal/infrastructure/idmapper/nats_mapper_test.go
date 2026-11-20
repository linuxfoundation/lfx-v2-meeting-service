// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParticipantAliasesNeverFallBackToHelper(t *testing.T) {
	mapper := &NATSMapper{}
	for name, lookup := range map[string]func(context.Context, string) (string, error){
		"invitee":                 mapper.MapInviteeIDToParticipantV2,
		"attendee":                mapper.MapAttendeeIDToParticipantV2,
		"participant to invitee":  mapper.MapParticipantV2ToInviteeID,
		"participant to attendee": mapper.MapParticipantV2ToAttendeeID,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := lookup(context.Background(), "test-id")
			require.Error(t, err)
			require.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
			_, err = lookup(context.Background(), "")
			require.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
		})
	}
}

// The owning project SFID from the committee mapping reply is what CreateMeeting
// uses to bind a committee-authorized create to the committee's own project.
func TestParseCommitteeMapping(t *testing.T) {
	t.Run("returns project and committee SFIDs", func(t *testing.T) {
		project, committee, err := parseCommitteeMapping("a0P000000PROJSFID1EAC:a0C000000COMMSFID01AC")
		require.NoError(t, err)
		assert.Equal(t, "a0P000000PROJSFID1EAC", project)
		assert.Equal(t, "a0C000000COMMSFID01AC", committee)
	})

	t.Run("returns an empty project when the reply has no project SFID", func(t *testing.T) {
		project, committee, err := parseCommitteeMapping("a0C000000COMMSFID01AC")
		require.NoError(t, err)
		assert.Empty(t, project)
		assert.Equal(t, "a0C000000COMMSFID01AC", committee)
	})

	t.Run("rejects malformed replies", func(t *testing.T) {
		for _, reply := range []string{"a:b:c", "a0P000000PROJSFID1EAC:"} {
			_, _, err := parseCommitteeMapping(reply)
			assert.Error(t, err, reply)
		}
	})
}
