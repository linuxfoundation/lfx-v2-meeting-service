// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"strings"
)

// ParsePastMeetingID converts a past meeting ID into its meeting ID and occurrence ID
// e.g. 1234567890-1692164906 -> meetingID=1234567890, occurrenceID=1692164906
// e.g. 1234567890 -> meetingID=1234567890, occurrenceID=
func ParsePastMeetingID(id string) (meetingID string, occurrenceID string) {
	meetingAndOccurrenceIDs := strings.Split(id, "-")
	if len(meetingAndOccurrenceIDs) == 1 {
		meetingID = id
	} else if len(meetingAndOccurrenceIDs) == 2 {
		meetingID = meetingAndOccurrenceIDs[0]
		occurrenceID = meetingAndOccurrenceIDs[1]
	}
	return
}

// GetPastMeetingID converts a meeting ID and occurrence ID into its past meeting ID
// e.g. meetingID=1234567890, occurrenceID=1692164906 -> 1234567890-1692164906
// e.g. meetingID=1234567890, occurrenceID= -> 1234567890
func GetPastMeetingID(meetingID string, occurrenceID string) (pastMeetingID string) {
	pastMeetingID = meetingID
	if occurrenceID != "" {
		pastMeetingID = fmt.Sprintf("%s-%s", meetingID, occurrenceID)
	}
	return
}
