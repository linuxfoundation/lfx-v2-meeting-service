// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package summaryformat is the one markdown rendering of a past meeting summary's
// parts, shared by the index writer (cmd/meeting-api/eventing) and the ITX proxy
// (cmd/meeting-api/service). The index document's content/edited_content, the proxy's
// summary_data.content/edited_content and the proxy's edited_content request attribute
// all carry this text, so an edit made on the rendered text parses back into the
// structured fields ITX holds.
//
// # Grammar
//
// Format writes, in this order, each section only when its input is non-empty:
//
//	## Overview
//	<overview>
//
//	## Key Topics
//	### <label>
//	<summary>
//
//	## Next Steps
//	- <step>
//
// A detail writes "### <label>" only when its label is non-empty and "<summary>" (with a
// blank line after it) only when its summary is non-empty; a next step is written only
// when non-empty. The whole result is trimmed of surrounding whitespace, and empty parts
// render as "".
//
// Parse is line-oriented, after normalising "\r\n" and "\r" to "\n":
//
//   - A line whose trimmed text is exactly "## Overview", "## Key Topics" or "## Next
//     Steps" switches the current section and is not content. Headings are
//     case-sensitive; surrounding whitespace is the only tolerance. Any other line,
//     including one that starts with "## " for an unknown heading, is content of the
//     current section, verbatim.
//   - Before any heading the section is Overview, so text whose "## Overview" line was
//     deleted still parses.
//   - Overview: content lines are joined with "\n" and the result trimmed.
//   - Key Topics: a line whose trimmed text starts with "### " (or is exactly "###")
//     begins a new detail whose label is the trimmed rest; other lines are appended to
//     the current detail's summary, joined with "\n" and trimmed at the end. Content
//     before the first "### " is a detail with an empty label, which is what Format
//     writes for a label-less detail. A detail whose label and summary are both empty
//     is dropped.
//   - Next Steps: each non-blank line is one step, after stripping one leading "- ",
//     "* " or "• " (or a bare "-") and trimming; a line that is only a bullet marker
//     yields no step. Blank lines are ignored.
//
// Parse(Format(p)) == p for canonical parts: trimmed, non-empty fields, single-line
// steps, and label-less details only before the first labelled one. The grammar has limits
// by design, which it does not escape: a multi-line step is not representable (each line of
// Next Steps is one step); free text that contains a section heading or a "### " line on
// its own line is read as structure; and a label-less detail that follows a labelled one is
// merged by Parse into the previous detail's summary, because Format writes it with no
// "### " line of its own, so Parse(Format(p)) == p only when label-less details come first.
// A Next Steps line without a bullet marker is still one step.
package summaryformat

import (
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// Section headings of the format. Format writes them; Parse recognises them exactly.
const (
	HeadingOverview  = "## Overview"
	HeadingKeyTopics = "## Key Topics"
	HeadingNextSteps = "## Next Steps"

	detailPrefix = "### "
	stepPrefix   = "- "
)

// Parts are the structured fields of a summary as ITX holds them.
type Parts struct {
	Overview  string
	Details   []itx.ZoomMeetingSummaryDetails
	NextSteps []string
}

// Format renders Parts as the summary markdown written to the index and returned by the
// proxy. Empty parts render as "".
func Format(p Parts) string {
	if p.Overview == "" && len(p.Details) == 0 && len(p.NextSteps) == 0 {
		return ""
	}

	var sb strings.Builder

	if p.Overview != "" {
		sb.WriteString(HeadingOverview + "\n")
		sb.WriteString(p.Overview)
		sb.WriteString("\n\n")
	}

	if len(p.Details) > 0 {
		sb.WriteString(HeadingKeyTopics + "\n")
		for _, detail := range p.Details {
			if detail.Label != "" {
				sb.WriteString(detailPrefix)
				sb.WriteString(detail.Label)
				sb.WriteString("\n")
			}
			if detail.Summary != "" {
				sb.WriteString(detail.Summary)
				sb.WriteString("\n\n")
			}
		}
	}

	if len(p.NextSteps) > 0 {
		sb.WriteString(HeadingNextSteps + "\n")
		for _, step := range p.NextSteps {
			if step != "" {
				sb.WriteString(stepPrefix)
				sb.WriteString(step)
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(sb.String())
}

// section is the part of the text Parse is currently reading.
type section int

const (
	sectionOverview section = iota
	sectionKeyTopics
	sectionNextSteps
)

// Parse reads summary markdown back into Parts. Parse(Format(p)) == p for canonical p;
// Parse("") returns zero Parts (nil slices). See the package documentation for the
// grammar and its tolerances.
func Parse(text string) Parts {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var (
		p        Parts
		current  = sectionOverview
		overview []string
		detail   *itx.ZoomMeetingSummaryDetails
		summary  []string
	)

	// closeDetail finishes the detail being read, keeping it unless it is entirely empty.
	closeDetail := func() {
		if detail == nil {
			return
		}
		detail.Summary = strings.TrimSpace(strings.Join(summary, "\n"))
		if detail.Label != "" || detail.Summary != "" {
			p.Details = append(p.Details, *detail)
		}
		detail, summary = nil, nil
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case HeadingOverview:
			closeDetail()
			current = sectionOverview
			continue
		case HeadingKeyTopics:
			closeDetail()
			current = sectionKeyTopics
			continue
		case HeadingNextSteps:
			closeDetail()
			current = sectionNextSteps
			continue
		}

		switch current {
		case sectionOverview:
			overview = append(overview, line)

		case sectionKeyTopics:
			if label, ok := cutDetailHeading(trimmed); ok {
				closeDetail()
				detail = &itx.ZoomMeetingSummaryDetails{Label: label}
				continue
			}
			if detail == nil {
				// Content before the first "### ": a label-less detail.
				detail = &itx.ZoomMeetingSummaryDetails{}
			}
			summary = append(summary, line)

		case sectionNextSteps:
			if step := cutStep(trimmed); step != "" {
				p.NextSteps = append(p.NextSteps, step)
			}
		}
	}
	closeDetail()

	p.Overview = strings.TrimSpace(strings.Join(overview, "\n"))
	return p
}

// cutDetailHeading reports whether the trimmed line is a "### <label>" heading and
// returns the trimmed label.
func cutDetailHeading(trimmed string) (string, bool) {
	if trimmed == "###" {
		return "", true
	}
	rest, ok := strings.CutPrefix(trimmed, detailPrefix)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// cutStep returns the step on a trimmed Next Steps line after one leading bullet marker,
// or "" for a blank line or a bare marker.
func cutStep(trimmed string) string {
	for _, marker := range []string{"-", "*", "•"} {
		if trimmed == marker {
			return ""
		}
		if rest, ok := strings.CutPrefix(trimmed, marker+" "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
}
