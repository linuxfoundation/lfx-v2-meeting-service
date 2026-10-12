// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package idmapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
