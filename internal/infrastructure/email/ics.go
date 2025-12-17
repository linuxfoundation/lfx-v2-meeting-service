// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package email

import (
	"fmt"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// ICS constants for consistent values across all generated ICS files
const (
	ICSProdID         = "-//Linux Foundation//LFX Meeting Service//EN"
	ICALVersion       = "2.0"
	ICALScale         = "GREGORIAN"
	ICALMaxLineLength = 75 // this is arbitrarily set to 75 characters to avoid long lines
)

// ICS organizer information
const (
	OrganizerEmail = "itx@linuxfoundation.org"
	OrganizerName  = "ITX"
)

// UTF-8 byte masks for line folding safety
const (
	UTF8TwoBitMask         = 0xC0 // Mask to isolate first two bits (11000000)
	UTF8ContinuationPrefix = 0x80 // UTF-8 continuation byte prefix (10000000)
)

// MeetingICSGenerator is the interface for generating ICS calendar files
type MeetingICSGenerator interface {
	GenerateMeetingInvitationICS(param ICSMeetingInvitationParams) (string, error)
	GenerateMeetingUpdateICS(params ICSMeetingUpdateParams) (string, error)
	GenerateMeetingCancellationICS(params ICSMeetingCancellationParams) (string, error)
	GenerateOccurrenceCancellationICS(params ICSOccurrenceCancellationParams) (string, error)
}

// ICSGenerator generates ICS (iCalendar) files for meeting invitations
type ICSGenerator struct{}

// NewICSGenerator creates a new ICS generator
func NewICSGenerator() *ICSGenerator {
	return &ICSGenerator{}
}

// Ensure [ICSGenerator] implements [MeetingICSGenerator]
var _ MeetingICSGenerator = (*ICSGenerator)(nil)

// ICSMeetingInvitationParams contains all the information needed to generate an ICS file
// for a meeting invitation
type ICSMeetingInvitationParams struct {
	MeetingUID               string // Unique meeting identifier for consistent ICS UID
	MeetingTitle             string
	MeetingType              string
	Description              string
	StartTime                time.Time
	DurationMinutes          int
	Timezone                 string
	PlatformJoinLink         string
	DirectZoomJoinLink       string
	MeetingID                string
	Passcode                 string
	RecipientEmail           string
	ProjectName              string
	Recurrence               *models.Recurrence
	Sequence                 int         // ICS sequence number for calendar updates
	CancelledOccurrenceTimes []time.Time // List of cancelled occurrence start times to exclude via EXDATE
	MeetingAttachments       []*models.MeetingAttachment
}

// GenerateMeetingICS generates an ICS file content for a meeting invitation
func (g *ICSGenerator) GenerateMeetingInvitationICS(param ICSMeetingInvitationParams) (string, error) {
	// Load timezone
	loc, err := time.LoadLocation(param.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone %q: %w", param.Timezone, err)
	}

	// Generate consistent UID using meeting UID
	uid := param.MeetingUID
	dtstamp := time.Now().UTC().Format("20060102T150405Z")

	// Convert times to the meeting timezone
	startLocal := param.StartTime.In(loc)
	endLocal := startLocal.Add(time.Duration(param.DurationMinutes) * time.Minute)

	// Format times in YYYYMMDDTHHMMSS format
	dtstart := startLocal.Format("20060102T150405")
	dtend := endLocal.Format("20060102T150405")

	// Build the ICS content
	var ics strings.Builder

	// Calendar header
	ics.WriteString("BEGIN:VCALENDAR\r\n")
	ics.WriteString(fmt.Sprintf("VERSION:%s\r\n", ICALVersion))
	ics.WriteString(fmt.Sprintf("PRODID:%s\r\n", ICSProdID))
	ics.WriteString(fmt.Sprintf("CALSCALE:%s\r\n", ICALScale))
	ics.WriteString("METHOD:REQUEST\r\n")

	// Timezone definition
	ics.WriteString(generateTimezoneDefinition(param.Timezone, loc))

	// Event
	ics.WriteString("BEGIN:VEVENT\r\n")
	ics.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))
	ics.WriteString(fmt.Sprintf("ORGANIZER;CN=%s:mailto:%s\r\n", OrganizerName, OrganizerEmail))
	ics.WriteString(fmt.Sprintf("DTSTART;TZID=%s:%s\r\n", param.Timezone, dtstart))
	ics.WriteString(fmt.Sprintf("DTEND;TZID=%s:%s\r\n", param.Timezone, dtend))

	// Add recurrence rule if provided
	if param.Recurrence != nil {
		rrule := generateRRule(param.Recurrence)
		if rrule != "" {
			ics.WriteString(fmt.Sprintf("RRULE:%s\r\n", rrule))
		}

		// Add EXDATE for cancelled occurrences to exclude them from the series
		if len(param.CancelledOccurrenceTimes) > 0 {
			var exdates []string
			for _, cancelledTime := range param.CancelledOccurrenceTimes {
				// Convert to meeting timezone and format as YYYYMMDDTHHMMSS
				cancelledLocal := cancelledTime.In(loc)
				exdates = append(exdates, cancelledLocal.Format("20060102T150405"))
			}
			// Join all cancelled dates with commas
			ics.WriteString(fmt.Sprintf("EXDATE;TZID=%s:%s\r\n", param.Timezone, strings.Join(exdates, ",")))
		}
	}

	// Meeting details
	ics.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICSText(param.MeetingTitle)))

	// Build enhanced description with meeting details
	descriptionText := buildDescription(DescriptionParams{
		MeetingDescription: param.Description,
		MeetingID:          param.MeetingID,
		MeetingPasscode:    param.Passcode,
		PlatformJoinLink:   param.PlatformJoinLink,
		DirectZoomJoinLink: param.DirectZoomJoinLink,
		ProjectName:        param.ProjectName,
		MeetingAttachments: param.MeetingAttachments,
	})
	ics.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICSText(descriptionText)))

	// Location (Zoom URL) - only add if join link exists
	if param.PlatformJoinLink != "" {
		ics.WriteString(fmt.Sprintf("LOCATION:%s\r\n", param.PlatformJoinLink))
		// URL property for the join link
		ics.WriteString(fmt.Sprintf("URL:%s\r\n", param.PlatformJoinLink))
	}

	// Attendee
	if param.RecipientEmail != "" {
		ics.WriteString(fmt.Sprintf("ATTENDEE;CUTYPE=INDIVIDUAL;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE;CN=%s:mailto:%s\r\n",
			param.RecipientEmail, param.RecipientEmail))
	}

	// Meeting properties
	ics.WriteString("STATUS:CONFIRMED\r\n")
	ics.WriteString("TRANSP:OPAQUE\r\n")
	ics.WriteString("CLASS:PUBLIC\r\n")
	ics.WriteString("PRIORITY:5\r\n")
	ics.WriteString(fmt.Sprintf("SEQUENCE:%d\r\n", param.Sequence))

	// Alarm
	ics.WriteString("BEGIN:VALARM\r\n")
	ics.WriteString("TRIGGER:-PT10M\r\n")
	ics.WriteString("ACTION:DISPLAY\r\n")
	ics.WriteString(fmt.Sprintf("DESCRIPTION:Reminder: %s\r\n", escapeICSText(param.MeetingTitle)))
	ics.WriteString("END:VALARM\r\n")

	ics.WriteString("END:VEVENT\r\n")
	ics.WriteString("END:VCALENDAR\r\n")

	return ics.String(), nil
}

// ICSMeetingUpdateParams holds parameters for generating a meeting update ICS file
type ICSMeetingUpdateParams struct {
	MeetingUID         string // Unique meeting identifier for consistent ICS UID
	MeetingTitle       string
	Description        string
	StartTime          time.Time
	Duration           int // Duration in minutes
	Timezone           string
	PlatformJoinLink   string
	DirectZoomJoinLink string
	MeetingID          string
	Passcode           string
	RecipientEmail     string
	ProjectName        string
	Recurrence         *models.Recurrence
	Sequence           int                         // Incremented sequence number for updates
	MeetingAttachments []*models.MeetingAttachment // Meeting attachments to include in description
}

// GenerateMeetingUpdateICS generates an ICS file for updating a meeting
func (g *ICSGenerator) GenerateMeetingUpdateICS(params ICSMeetingUpdateParams) (string, error) {
	// Load timezone
	loc, err := time.LoadLocation(params.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone %q: %w", params.Timezone, err)
	}

	// Generate consistent UID using meeting UID
	uid := params.MeetingUID
	dtstamp := time.Now().UTC().Format("20060102T150405Z")

	// Convert times to the meeting timezone
	startLocal := params.StartTime.In(loc)
	endLocal := startLocal.Add(time.Duration(params.Duration) * time.Minute)

	// Format times in YYYYMMDDTHHMMSS format
	dtstart := startLocal.Format("20060102T150405")
	dtend := endLocal.Format("20060102T150405")

	// Build the ICS content
	var ics strings.Builder

	// Calendar header
	ics.WriteString("BEGIN:VCALENDAR\r\n")
	ics.WriteString(fmt.Sprintf("VERSION:%s\r\n", ICALVersion))
	ics.WriteString(fmt.Sprintf("PRODID:%s\r\n", ICSProdID))
	ics.WriteString(fmt.Sprintf("CALSCALE:%s\r\n", ICALScale))
	ics.WriteString("METHOD:REQUEST\r\n")

	// Timezone definition
	ics.WriteString(generateTimezoneDefinition(params.Timezone, loc))

	// Event
	ics.WriteString("BEGIN:VEVENT\r\n")
	ics.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	ics.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", dtstamp))
	ics.WriteString(fmt.Sprintf("ORGANIZER;CN=%s:mailto:%s\r\n", OrganizerName, OrganizerEmail))
	ics.WriteString(fmt.Sprintf("DTSTART;TZID=%s:%s\r\n", params.Timezone, dtstart))
	ics.WriteString(fmt.Sprintf("DTEND;TZID=%s:%s\r\n", params.Timezone, dtend))

	// Add recurrence rule if provided
	if params.Recurrence != nil {
		rrule := generateRRule(params.Recurrence)
		if rrule != "" {
			ics.WriteString(fmt.Sprintf("RRULE:%s\r\n", rrule))
		}
	}

	// Meeting details
	ics.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICSText(params.MeetingTitle)))

	// Build enhanced description with meeting details
	descriptionText := buildDescription(DescriptionParams{
		MeetingDescription: params.Description,
		MeetingID:          params.MeetingID,
		MeetingPasscode:    params.Passcode,
		PlatformJoinLink:   params.PlatformJoinLink,
		DirectZoomJoinLink: params.DirectZoomJoinLink,
		ProjectName:        params.ProjectName,
		MeetingAttachments: params.MeetingAttachments,
	})
	ics.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICSText(descriptionText)))

	// Location (Platform join meeting URL) - only add if join link exists
	if params.PlatformJoinLink != "" {
		ics.WriteString(fmt.Sprintf("LOCATION:%s\r\n", params.PlatformJoinLink))
		// URL property for the join link
		ics.WriteString(fmt.Sprintf("URL:%s\r\n", params.PlatformJoinLink))
	}

	// Attendee
	if params.RecipientEmail != "" {
		ics.WriteString(fmt.Sprintf("ATTENDEE;CUTYPE=INDIVIDUAL;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE;CN=%s:mailto:%s\r\n",
			params.RecipientEmail, params.RecipientEmail))
	}

	// Meeting properties
	ics.WriteString("STATUS:CONFIRMED\r\n")
	ics.WriteString("TRANSP:OPAQUE\r\n")
	ics.WriteString("CLASS:PUBLIC\r\n")
	ics.WriteString("PRIORITY:5\r\n")
	ics.WriteString(fmt.Sprintf("SEQUENCE:%d\r\n", params.Sequence)) // Incremented sequence for updates

	// Alarm
	ics.WriteString("BEGIN:VALARM\r\n")
	ics.WriteString("TRIGGER:-PT10M\r\n")
	ics.WriteString("ACTION:DISPLAY\r\n")
	ics.WriteString(fmt.Sprintf("DESCRIPTION:Reminder: %s\r\n", escapeICSText(params.MeetingTitle)))
	ics.WriteString("END:VALARM\r\n")

	ics.WriteString("END:VEVENT\r\n")
	ics.WriteString("END:VCALENDAR\r\n")

	return ics.String(), nil
}

// ICSMeetingCancellationParams holds parameters for generating a meeting cancellation ICS file
type ICSMeetingCancellationParams struct {
	MeetingUID      string // Unique meeting identifier for consistent ICS UID
	MeetingTitle    string
	StartTime       time.Time
	DurationMinutes int
	Timezone        string
	RecipientEmail  string
	Recurrence      *models.Recurrence
	Sequence        int // ICS sequence number for calendar updates
}

// GenerateMeetingCancellationICS generates an ICS file for cancelling a meeting
func (g *ICSGenerator) GenerateMeetingCancellationICS(params ICSMeetingCancellationParams) (string, error) {
	loc, err := time.LoadLocation(params.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}

	startTime := params.StartTime.In(loc)
	endTime := startTime.Add(time.Duration(params.DurationMinutes) * time.Minute)

	// Use the same UID as the invitation for proper cancellation matching
	uid := params.MeetingUID

	var icsContent strings.Builder
	icsContent.WriteString("BEGIN:VCALENDAR\r\n")
	icsContent.WriteString(fmt.Sprintf("VERSION:%s\r\n", ICALVersion))
	icsContent.WriteString(fmt.Sprintf("PRODID:%s\r\n", ICSProdID))
	icsContent.WriteString("METHOD:CANCEL\r\n")
	icsContent.WriteString(fmt.Sprintf("CALSCALE:%s\r\n", ICALScale))
	icsContent.WriteString("BEGIN:VEVENT\r\n")
	icsContent.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	icsContent.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))
	icsContent.WriteString(fmt.Sprintf("DTSTART;TZID=%s:%s\r\n", params.Timezone, startTime.Format("20060102T150405")))
	icsContent.WriteString(fmt.Sprintf("DTEND;TZID=%s:%s\r\n", params.Timezone, endTime.Format("20060102T150405")))
	icsContent.WriteString(fmt.Sprintf("SUMMARY:%s (CANCELLED)\r\n", escapeICSText(params.MeetingTitle)))
	icsContent.WriteString("STATUS:CANCELLED\r\n")
	icsContent.WriteString(fmt.Sprintf("SEQUENCE:%d\r\n", params.Sequence))
	icsContent.WriteString(fmt.Sprintf("ORGANIZER;CN=%s:mailto:%s\r\n", OrganizerName, OrganizerEmail))

	if params.RecipientEmail != "" {
		icsContent.WriteString(fmt.Sprintf("ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:%s\r\n", params.RecipientEmail))
	}

	// Add recurrence rule if this was a recurring meeting
	if params.Recurrence != nil {
		rrule := generateRRule(params.Recurrence)
		if rrule != "" {
			icsContent.WriteString(fmt.Sprintf("RRULE:%s\r\n", rrule))
		}
	}

	icsContent.WriteString("END:VEVENT\r\n")

	// Include timezone definition
	icsContent.WriteString(generateTimezoneDefinition(params.Timezone, loc))

	icsContent.WriteString("END:VCALENDAR\r\n")

	return icsContent.String(), nil
}

// ICSOccurrenceCancellationParams holds parameters for generating a single occurrence cancellation ICS file
type ICSOccurrenceCancellationParams struct {
	MeetingUID          string // Unique meeting identifier for consistent ICS UID
	MeetingTitle        string
	OccurrenceStartTime time.Time // The specific occurrence start time
	Duration            int       // Duration in minutes
	Timezone            string
	RecipientEmail      string
	Sequence            int // ICS sequence number for calendar updates
}

// GenerateOccurrenceCancellationICS generates an ICS file to cancel a specific occurrence of a recurring meeting.
// This uses RECURRENCE-ID to target only a single occurrence rather than the entire series.
func (g *ICSGenerator) GenerateOccurrenceCancellationICS(params ICSOccurrenceCancellationParams) (string, error) {
	loc, err := time.LoadLocation(params.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone: %w", err)
	}

	startTime := params.OccurrenceStartTime.In(loc)
	endTime := startTime.Add(time.Duration(params.Duration) * time.Minute)

	// Use the same UID as the original meeting for proper cancellation matching
	uid := params.MeetingUID

	var icsContent strings.Builder
	icsContent.WriteString("BEGIN:VCALENDAR\r\n")
	icsContent.WriteString(fmt.Sprintf("VERSION:%s\r\n", ICALVersion))
	icsContent.WriteString(fmt.Sprintf("PRODID:%s\r\n", ICSProdID))
	icsContent.WriteString("METHOD:CANCEL\r\n")
	icsContent.WriteString(fmt.Sprintf("CALSCALE:%s\r\n", ICALScale))
	icsContent.WriteString("BEGIN:VEVENT\r\n")
	icsContent.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	icsContent.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z")))
	icsContent.WriteString(fmt.Sprintf("DTSTART;TZID=%s:%s\r\n", params.Timezone, startTime.Format("20060102T150405")))
	icsContent.WriteString(fmt.Sprintf("DTEND;TZID=%s:%s\r\n", params.Timezone, endTime.Format("20060102T150405")))

	// RECURRENCE-ID identifies which specific occurrence to cancel
	icsContent.WriteString(fmt.Sprintf("RECURRENCE-ID;TZID=%s:%s\r\n", params.Timezone, startTime.Format("20060102T150405")))

	icsContent.WriteString(fmt.Sprintf("SUMMARY:%s (CANCELLED)\r\n", escapeICSText(params.MeetingTitle)))
	icsContent.WriteString("STATUS:CANCELLED\r\n")
	icsContent.WriteString(fmt.Sprintf("SEQUENCE:%d\r\n", params.Sequence))
	icsContent.WriteString(fmt.Sprintf("ORGANIZER;CN=%s:mailto:%s\r\n", OrganizerName, OrganizerEmail))

	if params.RecipientEmail != "" {
		icsContent.WriteString(fmt.Sprintf("ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:%s\r\n", params.RecipientEmail))
	}

	icsContent.WriteString("END:VEVENT\r\n")

	// Include timezone definition
	icsContent.WriteString(generateTimezoneDefinition(params.Timezone, loc))

	icsContent.WriteString("END:VCALENDAR\r\n")

	return icsContent.String(), nil
}

// generateTimezoneDefinition generates the VTIMEZONE component
func generateTimezoneDefinition(tzid string, _ *time.Location) string {
	// For simplicity, we'll use a basic timezone definition
	// In production, you might want to use a more comprehensive timezone database
	var tz strings.Builder
	tz.WriteString("BEGIN:VTIMEZONE\r\n")
	tz.WriteString(fmt.Sprintf("TZID:%s\r\n", tzid))
	tz.WriteString(fmt.Sprintf("X-LIC-LOCATION:%s\r\n", tzid))
	tz.WriteString("END:VTIMEZONE\r\n")
	return tz.String()
}

// generateRRule generates a recurrence rule (RRULE) from the meeting recurrence
func generateRRule(recurrence *models.Recurrence) string {
	if recurrence == nil {
		return ""
	}

	var parts []string

	// Recurrence type mapping
	// Type 1: Daily, 2: Weekly, 3: Monthly
	switch recurrence.Type {
	case 1: // Daily
		parts = append(parts, "FREQ=DAILY")
		if recurrence.RepeatInterval > 1 {
			parts = append(parts, fmt.Sprintf("INTERVAL=%d", recurrence.RepeatInterval))
		}
	case 2: // Weekly
		parts = append(parts, "FREQ=WEEKLY")
		if recurrence.RepeatInterval > 1 {
			parts = append(parts, fmt.Sprintf("INTERVAL=%d", recurrence.RepeatInterval))
		}
		if recurrence.WeeklyDays != "" {
			// Convert numeric days to RFC5545 format (SU,MO,TU,WE,TH,FR,SA)
			byday := convertWeeklyDays(recurrence.WeeklyDays)
			if byday != "" {
				parts = append(parts, fmt.Sprintf("BYDAY=%s", byday))
			}
		}
	case 3: // Monthly
		parts = append(parts, "FREQ=MONTHLY")
		if recurrence.RepeatInterval > 1 {
			parts = append(parts, fmt.Sprintf("INTERVAL=%d", recurrence.RepeatInterval))
		}
		if recurrence.MonthlyDay > 0 {
			parts = append(parts, fmt.Sprintf("BYMONTHDAY=%d", recurrence.MonthlyDay))
		} else if recurrence.MonthlyWeek > 0 && recurrence.MonthlyWeekDay > 0 {
			// Handle "nth weekday of month" pattern
			weekdayName := getWeekdayName(recurrence.MonthlyWeekDay)
			parts = append(parts, fmt.Sprintf("BYDAY=%d%s", recurrence.MonthlyWeek, weekdayName))
		}
	default:
		return ""
	}

	// Add end condition
	if recurrence.EndTimes > 0 {
		parts = append(parts, fmt.Sprintf("COUNT=%d", recurrence.EndTimes))
	} else if recurrence.EndDateTime != nil {
		endDate := recurrence.EndDateTime.UTC().Format("20060102T150405Z")
		parts = append(parts, fmt.Sprintf("UNTIL=%s", endDate))
	}

	return strings.Join(parts, ";")
}

// convertWeeklyDays converts numeric weekday representation to RFC5545 format
func convertWeeklyDays(weeklyDays string) string {
	// Assuming weeklyDays is a comma-separated list of numbers (1=Sunday, 2=Monday, etc.)
	dayMap := map[string]string{
		"1": "SU",
		"2": "MO",
		"3": "TU",
		"4": "WE",
		"5": "TH",
		"6": "FR",
		"7": "SA",
	}

	days := strings.Split(weeklyDays, ",")
	var convertedDays []string
	for _, day := range days {
		day = strings.TrimSpace(day)
		if mapped, ok := dayMap[day]; ok {
			convertedDays = append(convertedDays, mapped)
		}
	}

	return strings.Join(convertedDays, ",")
}

// getWeekdayName converts numeric weekday to RFC5545 format
func getWeekdayName(weekday int) string {
	weekdays := []string{"", "SU", "MO", "TU", "WE", "TH", "FR", "SA"}
	if weekday >= 1 && weekday < len(weekdays) {
		return weekdays[weekday]
	}
	return ""
}

// buildMeetingAttachmentsDescription formats the attachments list for ICS description
func buildMeetingAttachmentsDescription(meetingAttachments []*models.MeetingAttachment) string {
	if len(meetingAttachments) == 0 {
		return ""
	}

	var section strings.Builder
	section.WriteString("Attachments:\n")

	// First, display all link attachments
	for _, attachment := range meetingAttachments {
		if attachment.Type == models.AttachmentTypeLink {
			// For link attachments, always show the URL (so it's copyable/clickable)
			section.WriteString(fmt.Sprintf("• %s", attachment.Link))
			if attachment.Description != "" {
				section.WriteString(fmt.Sprintf(" - %s", attachment.Description))
			}
			section.WriteString("\n")
		}
	}

	// Check if there are any file attachments
	hasFiles := false
	for _, attachment := range meetingAttachments {
		if attachment.Type != models.AttachmentTypeLink {
			hasFiles = true
			break
		}
	}

	// Then, display all file attachments with instructional message
	if hasFiles {
		section.WriteString("\nTo download files, click on the 'Join meeting' link:\n")
		for _, attachment := range meetingAttachments {
			if attachment.Type != models.AttachmentTypeLink {
				// For file attachments, show name (or filename if no name)
				if attachment.Name != "" {
					section.WriteString(fmt.Sprintf("• %s", attachment.Name))
				} else if attachment.FileName != "" {
					section.WriteString(fmt.Sprintf("• %s", attachment.FileName))
				}
				if attachment.Description != "" {
					section.WriteString(fmt.Sprintf(" - %s", attachment.Description))
				}
				section.WriteString("\n")
			}
		}
	}
	section.WriteString("\n")
	return section.String()
}

type DescriptionParams struct {
	MeetingDescription string
	MeetingID          string
	MeetingPasscode    string
	PlatformJoinLink   string
	DirectZoomJoinLink string
	ProjectName        string
	MeetingAttachments []*models.MeetingAttachment
}

// buildDescription builds the enhanced description with meeting details
func buildDescription(params DescriptionParams) string {
	var desc strings.Builder

	if params.ProjectName != "" {
		desc.WriteString(params.ProjectName)
		desc.WriteString(" Meeting")
		desc.WriteString("\n\n")
	}

	// Add attachments section before the description
	attachmentsSection := buildMeetingAttachmentsDescription(params.MeetingAttachments)
	if attachmentsSection != "" {
		desc.WriteString(attachmentsSection)
	}

	if params.MeetingDescription != "" {
		desc.WriteString(params.MeetingDescription)
		desc.WriteString("\n\n")
	}

	if params.PlatformJoinLink != "" {
		desc.WriteString("Join Meeting: ")
		desc.WriteString(params.PlatformJoinLink)
		desc.WriteString("\n\n")
	}

	if params.MeetingID != "" {
		desc.WriteString("Meeting ID: ")
		desc.WriteString(params.MeetingID)
		desc.WriteString("\n")
	}

	if params.MeetingPasscode != "" {
		desc.WriteString("Passcode: ")
		desc.WriteString(params.MeetingPasscode)
		desc.WriteString("\n")
	}

	if params.MeetingID != "" {
		desc.WriteString("\n")
		desc.WriteString("To dial in, find your local number: https://zoom.us/zoomconference\n")
		desc.WriteString("After dialing, enter Meeting ID: ")
		desc.WriteString(params.MeetingID)
		desc.WriteString("#\n")
		if params.MeetingPasscode != "" {
			desc.WriteString("Then enter Passcode: ")
			desc.WriteString(params.MeetingPasscode)
			desc.WriteString("#\n\n")
		}
	}

	if params.DirectZoomJoinLink != "" {
		desc.WriteString("If you cannot log in via LFX One, you can use this direct Zoom join link: ")
		desc.WriteString(params.DirectZoomJoinLink)
		desc.WriteString("\n")
	}

	return desc.String()
}

// escapeICSText escapes special characters in ICS text fields
func escapeICSText(text string) string {
	// Escape special characters according to RFC5545
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, ";", "\\;")

	// Fold long lines (75 characters max per line, continued lines start with space)
	return foldICSLine(text, ICALMaxLineLength)
}

// foldICSLine folds long lines according to RFC5545 (75 octets max)
func foldICSLine(line string, maxLength int) string {
	if len(line) <= maxLength {
		return line
	}

	var folded strings.Builder
	remaining := line
	first := true

	for len(remaining) > 0 {
		cutLength := maxLength
		if !first {
			cutLength = maxLength - 1 // Account for leading space on continued lines
		}

		if len(remaining) <= cutLength {
			if !first {
				folded.WriteString("\r\n ")
			}
			folded.WriteString(remaining)
			break
		}

		// Find a safe place to break (not in the middle of a UTF-8 sequence)
		breakPoint := cutLength
		for breakPoint > 0 && remaining[breakPoint-1]&UTF8TwoBitMask == UTF8ContinuationPrefix {
			breakPoint--
		}

		if !first {
			folded.WriteString("\r\n ")
		}
		folded.WriteString(remaining[:breakPoint])
		remaining = remaining[breakPoint:]
		first = false
	}

	return folded.String()
}
