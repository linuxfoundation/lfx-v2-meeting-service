// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import "errors"

// MigrationFailureCategory returns only allowlisted operator-safe categories,
// never raw database/transport errors, credentials, keys or source values.
func MigrationFailureCategory(err error) string {
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		switch cause.Error() {
		case "another meeting state import is active":
			return "import_busy"
		case "verified cutover already exists; refusing to overwrite authoritative Postgres state":
			return "already_verified"
		case "source contains no meeting state; refusing an unproven empty cutover":
			return "empty_source"
		case "invalid owned source key; manual reconciliation required", "invalid owned source state; manual reconciliation required":
			return "invalid_source_state"
		case "target state differs from source; refusing to overwrite":
			return "target_conflict"
		case "legacy source changed during import; cutover is not verified":
			return "source_changed"
		}
		for _, table := range []string{"meeting_resource_sync_state", "meeting_committee_associations", "meeting_summary_associations", "meeting_participant_lookup", "meeting_invitation_guards", "meeting_participant_aliases"} {
			if cause.Error() == "target contains unmatched rows in "+table+"; manual reconciliation required" {
				return "target_extra_rows"
			}
		}
	}
	return "infrastructure_failure"
}
