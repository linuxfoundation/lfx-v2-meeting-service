// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationFailureCategory(t *testing.T) {
	for message, want := range map[string]string{
		"another meeting state import is active":                                                      "import_busy",
		"verified cutover already exists; refusing to overwrite authoritative Postgres state":         "already_verified",
		"source contains no meeting state; refusing an unproven empty cutover":                        "empty_source",
		"invalid owned source state; manual reconciliation required":                                  "invalid_source_state",
		"target state differs from source; refusing to overwrite":                                     "target_conflict",
		"target contains unmatched rows in meeting_invitation_guards; manual reconciliation required": "target_extra_rows",
		"legacy source changed during import; cutover is not verified":                                "source_changed",
		"private-driver-details": "infrastructure_failure",
	} {
		t.Run(want, func(t *testing.T) {
			require.Equal(t, want, MigrationFailureCategory(fmt.Errorf("wrapped: %w", errors.New(message))))
		})
	}
}
