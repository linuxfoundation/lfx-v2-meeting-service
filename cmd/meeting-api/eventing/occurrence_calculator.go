// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/teambition/rrule-go"
)

const (
	meetingEndBuffer = 40 * time.Minute
)

var weekdaysABBRV = []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
var typeName = []string{"Daily", "Weekly", "Monthly"}

// OccurrenceCalculator calculates meeting occurrences based on RRULE recurrence patterns.
// The algorithm mirrors the canonical ITX CalculateOccurrencesV2 logic to ensure that
// what is stored in OpenSearch matches what ITX computes.
type OccurrenceCalculator struct {
	logger *slog.Logger
}

// NewOccurrenceCalculator creates a new occurrence calculator
func NewOccurrenceCalculator(logger *slog.Logger) *OccurrenceCalculator {
	return &OccurrenceCalculator{
		logger: logger,
	}
}

// seriesSegment represents a contiguous run of occurrences governed by a single recurrence pattern.
// The base segment starts at the meeting's original StartTime.
// Each all_following updated occurrence starts a new segment.
type seriesSegment struct {
	startUnix         int64 // unix timestamp of this segment's first occurrence
	oldOccurrenceUnix int64 // for all_following segments: the original occurrence being replaced (0 for base)
	recurrence        *models.ZoomMeetingRecurrence
	duration          int
	title             string
	description       string
}

// CalculateOccurrences generates occurrence objects for a recurring meeting.
//
// The implementation is a port of ITX's CalculateOccurrencesV2:
//  1. Build contiguous series segments (base + each all_following update).
//  2. Expand each segment via RRULE, bounded by the next segment's oldOccurrenceUnix
//     (the original-timeline occurrence that the next cadence change replaces).
//  3. Global dedup: skip any occurrence that is replaced by an all_following update,
//     except for the anchor occurrence of the segment that owns it.
//  4. Overlay single (non-all_following) updated occurrences.
//  5. Sort by occurrence ID (unix timestamp) and apply the limit.
//
// This is the canonical algorithm — diverging from it causes occurrence/rule mismatch
// between OpenSearch (written here) and ITX (the source of truth).
func (c *OccurrenceCalculator) CalculateOccurrences(
	ctx context.Context,
	meeting models.MeetingEventData,
	pastOccurrences bool,
	includeCancelled bool,
	numOccurrencesToReturn int,
) ([]models.Occurrence, error) {
	if meeting.Recurrence == nil {
		return nil, nil
	}

	// 1. Build series segments
	segments, err := c.buildSeriesSegments(meeting)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(segments, func(a, b seriesSegment) int {
		return cmp.Compare(a.startUnix, b.startUnix)
	})

	timezone := meeting.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Build a global map: originalOccurrenceID → segment index, for replaced-occurrence dedup.
	// An occurrence that appears here is "owned" by the replacing segment (not the base series).
	oldOccurrenceToSegmentIndex := make(map[string]int)
	for i, seg := range segments {
		if seg.oldOccurrenceUnix > 0 {
			oldID := strconv.FormatInt(seg.oldOccurrenceUnix, 10)
			oldOccurrenceToSegmentIndex[oldID] = i
		}
	}

	// 2. Expand segments
	occurrencesByID := make(map[string]models.Occurrence)
	for si, seg := range segments {
		var boundUnix int64

		if seg.oldOccurrenceUnix == 0 {
			// Base segment: check if an earlier all_following segment bounds it
			for i := 0; i < si; i++ {
				prevSeg := segments[i]
				if prevSeg.oldOccurrenceUnix > 0 && prevSeg.startUnix < seg.startUnix {
					boundUnix = prevSeg.oldOccurrenceUnix
					c.logger.DebugContext(ctx, "base segment bounded by earlier all_following segment",
						"base_start", seg.startUnix, "bound_at", boundUnix)
					break
				}
			}
		}

		// If not yet bounded, check the next segment
		if boundUnix == 0 && si+1 < len(segments) {
			nextSeg := segments[si+1]
			switch {
			case nextSeg.oldOccurrenceUnix == 0 && seg.oldOccurrenceUnix > 0 && seg.startUnix < nextSeg.startUnix:
				// all_following segment starts before the base segment — don't bound it
				c.logger.DebugContext(ctx, "all_following segment starts before base; not bounding",
					"seg_start", seg.startUnix, "base_start", nextSeg.startUnix)
			case nextSeg.oldOccurrenceUnix > 0:
				boundUnix = nextSeg.oldOccurrenceUnix
			default:
				boundUnix = nextSeg.startUnix
			}
		}

		segStart := time.Unix(seg.startUnix, 0)
		rruleOccurrences, err := c.getRRuleOccurrences(segStart, timezone, seg.recurrence, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get rrule occurrences for segment starting at %d: %w", seg.startUnix, err)
		}
		c.logger.DebugContext(ctx, "segment expanded",
			"meeting_id", meeting.ID, "segment_idx", si,
			"seg_start", seg.startUnix, "bound", boundUnix,
			"occurrences_count", len(rruleOccurrences))

		for _, o := range rruleOccurrences {
			// Stop when we reach the boundary of the next segment
			if boundUnix > 0 && o.Unix() >= boundUnix {
				break
			}
			if !pastOccurrences && isOccurrencePast(o, seg.duration) {
				continue
			}
			occurrenceID := strconv.FormatInt(o.Unix(), 10)

			// Global dedup: skip occurrences replaced by an all_following update,
			// unless this is the anchor of the current segment (the occurrence that triggers the new pattern).
			if segIdx, isReplaced := oldOccurrenceToSegmentIndex[occurrenceID]; isReplaced {
				isCurrentAnchor := seg.oldOccurrenceUnix > 0 &&
					occurrenceID == strconv.FormatInt(seg.oldOccurrenceUnix, 10) &&
					segIdx == si
				if !isCurrentAnchor {
					continue
				}
			}

			// Recurrence is only stamped on the anchor of an all_following segment.
			// Base-segment occurrences never carry a Recurrence field.
			var rec *models.ZoomMeetingRecurrence
			if seg.oldOccurrenceUnix > 0 && o.Unix() == seg.startUnix {
				rec = seg.recurrence
			}

			title := seg.title
			if title == "" {
				title = meeting.Title
			}
			description := seg.description
			if description == "" {
				description = meeting.Description
			}

			occ := models.Occurrence{
				OccurrenceID: occurrenceID,
				StartTime:    o.UTC(),
				Duration:     seg.duration,
				IsCancelled:  false,
				Title:        title,
				Description:  description,
				Recurrence:   rec,
			}

			if slices.Contains(meeting.CancelledOccurrences, occurrenceID) {
				if !includeCancelled {
					continue
				}
				occ.IsCancelled = true
			}

			occurrencesByID[occurrenceID] = occ
		}
	}

	// 3. Overlay single (non-all_following) updated occurrences.
	//    These override individual occurrences without starting a new recurrence series.
	var singles []models.UpdatedOccurrence
	allFollowingByOldID := make(map[string]models.UpdatedOccurrence)
	for _, uo := range meeting.UpdatedOccurrences {
		if uo.AllFollowing {
			allFollowingByOldID[uo.OldOccurrenceID] = uo
		} else {
			singles = append(singles, uo)
		}
	}
	slices.SortFunc(singles, func(a, b models.UpdatedOccurrence) int {
		ai, _ := strconv.ParseInt(a.NewOccurrenceID, 10, 64)
		bi, _ := strconv.ParseInt(b.NewOccurrenceID, 10, 64)
		return cmp.Compare(ai, bi)
	})

	for _, uo := range singles {
		delete(occurrencesByID, uo.OldOccurrenceID)

		// If an all_following update generated an occurrence at a different ID for the same
		// original occurrence, remove it too (the single update takes priority).
		if afUo, found := allFollowingByOldID[uo.OldOccurrenceID]; found {
			if afUo.NewOccurrenceID != uo.NewOccurrenceID {
				delete(occurrencesByID, afUo.NewOccurrenceID)
			}
		}

		newUnix, err := strconv.ParseInt(uo.NewOccurrenceID, 10, 64)
		if err != nil {
			c.logger.DebugContext(ctx, "skipping single updated occurrence with unparseable NewOccurrenceID",
				"new_occurrence_id", uo.NewOccurrenceID)
			continue
		}
		newStart := time.Unix(newUnix, 0)

		// Inherit duration from the effective segment at this time, then apply the override.
		eff := effectiveSegmentForTime(segments, newUnix)
		duration := eff.duration
		if duration == 0 {
			duration = meeting.Duration
		}
		if uo.Duration != 0 {
			duration = uo.Duration
		}

		if !pastOccurrences && isOccurrencePast(newStart, duration) {
			continue
		}

		title := uo.Title
		if title == "" {
			title = eff.title
		}
		if title == "" {
			title = meeting.Title
		}
		description := uo.Description
		if description == "" {
			description = eff.description
		}
		if description == "" {
			description = meeting.Description
		}

		occ := models.Occurrence{
			OccurrenceID: uo.NewOccurrenceID,
			StartTime:    newStart.UTC(),
			Duration:     duration,
			IsCancelled:  false,
			Title:        title,
			Description:  description,
			Recurrence:   nil, // Single updates never start a new recurrence series
		}
		if slices.Contains(meeting.CancelledOccurrences, uo.NewOccurrenceID) {
			if !includeCancelled {
				continue
			}
			occ.IsCancelled = true
		}
		occurrencesByID[occ.OccurrenceID] = occ
	}

	// 4. Sort and apply the occurrence limit
	result := make([]models.Occurrence, 0, len(occurrencesByID))
	for _, occ := range occurrencesByID {
		result = append(result, occ)
	}
	slices.SortFunc(result, func(a, b models.Occurrence) int {
		ai, _ := strconv.ParseInt(a.OccurrenceID, 10, 64)
		bi, _ := strconv.ParseInt(b.OccurrenceID, 10, 64)
		return cmp.Compare(ai, bi)
	})
	if numOccurrencesToReturn > 0 && len(result) > numOccurrencesToReturn {
		result = result[:numOccurrencesToReturn]
	}

	c.logger.DebugContext(ctx, "calculated occurrences",
		"meeting_id", meeting.ID, "count", len(result))
	return result, nil
}

// buildSeriesSegments builds the ordered list of contiguous series segments for a meeting.
// The base segment covers the meeting's original recurrence pattern.
// Each all_following updated occurrence starts a new segment from its NewOccurrenceID,
// inheriting duration/title/description/recurrence from the previous segment when not set.
func (c *OccurrenceCalculator) buildSeriesSegments(meeting models.MeetingEventData) ([]seriesSegment, error) {
	startObj, err := time.Parse(time.RFC3339, meeting.StartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse meeting start_time %q: %w", meeting.StartTime, err)
	}

	base := seriesSegment{
		startUnix:   startObj.Unix(),
		recurrence:  meeting.Recurrence,
		duration:    meeting.Duration,
		title:       meeting.Title,
		description: meeting.Description,
	}
	segments := []seriesSegment{base}

	// Collect and sort all_following updates by their new start time (ascending)
	var updates []models.UpdatedOccurrence
	for _, uo := range meeting.UpdatedOccurrences {
		if uo.AllFollowing {
			updates = append(updates, uo)
		}
	}
	slices.SortFunc(updates, func(a, b models.UpdatedOccurrence) int {
		ai, _ := strconv.ParseInt(a.NewOccurrenceID, 10, 64)
		bi, _ := strconv.ParseInt(b.NewOccurrenceID, 10, 64)
		return cmp.Compare(ai, bi)
	})

	// Build segments by inheriting forward — each update only overrides what it explicitly sets.
	curr := base
	for _, uo := range updates {
		newUnix, err := strconv.ParseInt(uo.NewOccurrenceID, 10, 64)
		if err != nil {
			c.logger.Warn("failed to parse NewOccurrenceID for all_following update, skipping",
				"new_occurrence_id", uo.NewOccurrenceID)
			continue
		}
		oldUnix, err := strconv.ParseInt(uo.OldOccurrenceID, 10, 64)
		if err != nil {
			c.logger.Warn("failed to parse OldOccurrenceID for all_following update, skipping",
				"old_occurrence_id", uo.OldOccurrenceID)
			continue
		}
		if uo.Recurrence != nil {
			curr.recurrence = uo.Recurrence
		}
		if uo.Duration != 0 {
			curr.duration = uo.Duration
		}
		if uo.Title != "" {
			curr.title = uo.Title
		}
		if uo.Description != "" {
			curr.description = uo.Description
		}
		segments = append(segments, seriesSegment{
			startUnix:         newUnix,
			oldOccurrenceUnix: oldUnix,
			recurrence:        curr.recurrence,
			duration:          curr.duration,
			title:             curr.title,
			description:       curr.description,
		})
	}
	return segments, nil
}

// effectiveSegmentForTime returns the segment that governs a given unix timestamp.
// Segments must be sorted by startUnix ascending before calling this.
func effectiveSegmentForTime(segments []seriesSegment, unix int64) seriesSegment {
	if len(segments) == 0 {
		return seriesSegment{}
	}
	eff := segments[0]
	for _, seg := range segments {
		if seg.startUnix <= unix {
			eff = seg
		} else {
			break
		}
	}
	return eff
}

// getEffectiveRecurrence returns the recurrence rule that is active as of time t.
// It walks the all_following updates sorted by start time and returns the last one
// whose segment start is at or before t — mirroring ITX's GetRecurrenceAtTime logic.
// If no all_following updates have started by t, the meeting's base recurrence is returned.
func getEffectiveRecurrence(meeting models.MeetingEventData, t time.Time) *models.ZoomMeetingRecurrence {
	if meeting.Recurrence == nil {
		return nil
	}

	type entry struct {
		startUnix int64
		rec       *models.ZoomMeetingRecurrence
	}
	var entries []entry
	for _, uo := range meeting.UpdatedOccurrences {
		if !uo.AllFollowing || uo.Recurrence == nil {
			continue
		}
		unix, err := strconv.ParseInt(uo.NewOccurrenceID, 10, 64)
		if err != nil {
			continue
		}
		entries = append(entries, entry{startUnix: unix, rec: uo.Recurrence})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Compare(a.startUnix, b.startUnix)
	})

	current := meeting.Recurrence
	for _, e := range entries {
		// Use the same "strictly after" test as ITX's GetRecurrenceAtTime.
		if t.After(time.Unix(e.startUnix, 0)) {
			current = e.rec
		}
	}
	return current
}

func isOccurrencePast(startTime time.Time, duration int) bool {
	return startTime.Add(time.Duration(duration) * time.Minute).Add(meetingEndBuffer).Before(time.Now())
}

// timeInLocation returns error if name is invalid or empty.
// Otherwise, it returns the time for the given location. Example:
// if name == "Asia/Shanghai", returned time is in "Asia/Shanghai".
func timeInLocation(t time.Time, name string) (time.Time, error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Time{}, err
	}

	return t.In(loc), err
}

// getRRuleOccurrences given a start time, optional timezone, and recurrence pattern, calculates and returns
// the list of occurrence times
func (c *OccurrenceCalculator) getRRuleOccurrences(startTime time.Time, timezone string, recurrence *models.ZoomMeetingRecurrence, endTime *time.Time) ([]time.Time, error) {
	rruleString, err := c.getRRule(recurrence, endTime)
	if err != nil {
		return nil, err
	}

	if timezone != "" {
		startTime, err = timeInLocation(startTime, timezone)
		if err != nil {
			return nil, err
		}
	}

	set := rrule.Set{}
	r, err := rrule.StrToRRule(rruleString)
	if err != nil {
		return nil, err
	}
	r.DTStart(startTime)
	set.RRule(r)

	return set.All(), nil
}

// getRRule returns the recurrence rule for a meeting recurrence as a string
func (c *OccurrenceCalculator) getRRule(reccurrence *models.ZoomMeetingRecurrence, endTime *time.Time) (string, error) {
	var rrule strings.Builder

	if reccurrence.Type < 1 || reccurrence.Type > 3 {
		return "", fmt.Errorf("invalid recurrence type: %d", reccurrence.Type)
	}

	fmt.Fprintf(&rrule, "FREQ=%s;", strings.ToUpper(typeName[reccurrence.Type-1]))
	rrule.WriteString("WKST=SU;")

	if reccurrence.RepeatInterval != 0 {
		fmt.Fprintf(&rrule, "INTERVAL=%d;", reccurrence.RepeatInterval)
	}

	if reccurrence.WeeklyDays != "" {
		s, err := parseByDay(reccurrence.WeeklyDays)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&rrule, "BYDAY=%s;", s)
	} else if reccurrence.MonthlyWeek != 0 && reccurrence.MonthlyWeekDay != 0 {
		fmt.Fprintf(&rrule, "BYDAY=%d%s;", reccurrence.MonthlyWeek, weekdaysABBRV[reccurrence.MonthlyWeekDay-1])
	}

	if reccurrence.MonthlyDay != 0 {
		switch reccurrence.MonthlyDay {
		case 29:
			rrule.WriteString("BYMONTHDAY=28,29;BYSETPOS=-1;") // fall back to the 28th on months with 28 days if recurrence set to every 29th
		case 30:
			rrule.WriteString("BYMONTHDAY=28,29,30;BYSETPOS=-1;")
		case 31:
			rrule.WriteString("BYMONTHDAY=28,29,30,31;BYSETPOS=-1;")
		default:
			fmt.Fprintf(&rrule, "BYMONTHDAY=%d;", reccurrence.MonthlyDay)
		}
	}

	if endTime != nil {
		fmt.Fprintf(&rrule, "UNTIL=%s;", endTime.Format("20060102T150405Z"))
	} else {
		// Use a local copy to avoid mutating the caller's recurrence object
		endTimes := reccurrence.EndTimes
		if reccurrence.EndDateTime != "" {
			endTimes = 0
			t, err := time.Parse(time.RFC3339, reccurrence.EndDateTime)
			if err != nil {
				return "", fmt.Errorf("failed to parse recurrence end_date_time %s: %w", reccurrence.EndDateTime, err)
			}
			fmt.Fprintf(&rrule, "UNTIL=%s;", t.Format("20060102T150405Z"))
		}

		if endTimes != 0 {
			fmt.Fprintf(&rrule, "COUNT=%d;", endTimes)
		} else if reccurrence.EndDateTime == "" {
			// No terminal condition — add a safety cap to prevent set.All() from
			// generating an unbounded sequence and exhausting memory.
			rrule.WriteString("COUNT=1000;")
		}
	}

	return strings.TrimSuffix(rrule.String(), ";"), nil
}

// parseByDay takes a list of weekdays as a string and returns the list of
// abbreviations as a string where 1 is Sunday and 7 is Saturday
// (e.g. "2,3,6" -> "MO,TU,FR")
func parseByDay(days string) (string, error) {
	stringSlice := strings.Split(days, ",")
	var weekdays strings.Builder
	var hasWritten bool
	for _, item := range stringSlice {
		weekdayNum, err := strconv.Atoi(item)
		if err != nil {
			return "", err
		}
		// A weekday can only be 1-7. Skip numbers that are not in this range.
		if weekdayNum < 1 || weekdayNum > 7 {
			continue
		}
		if hasWritten {
			weekdays.WriteString(",")
		}
		weekdays.WriteString(weekdaysABBRV[weekdayNum-1])
		hasWritten = true
	}
	return weekdays.String(), nil
}
