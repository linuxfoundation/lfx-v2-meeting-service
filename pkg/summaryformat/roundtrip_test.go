// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package summaryformat_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/summaryformat"
)

// documentedSample is the markdown shown in docs/event-processing.md ("Summary Content
// Assembly"), the shape LFX Self Serve displays and sends back as edited_content.
const documentedSample = "## Overview\nTeam discussed Q1 roadmap\n\n## Key Topics\n### Feature Planning\nPrioritized auth improvements\n\n### Bug Review\nIdentified 3 critical issues\n\n## Next Steps\n- Schedule design review\n- Update roadmap doc"

var documentedParts = summaryformat.Parts{
	Overview: "Team discussed Q1 roadmap",
	Details: []itx.ZoomMeetingSummaryDetails{
		{Label: "Feature Planning", Summary: "Prioritized auth improvements"},
		{Label: "Bug Review", Summary: "Identified 3 critical issues"},
	},
	NextSteps: []string{"Schedule design review", "Update roadmap doc"},
}

// TestRoundTrip_ParseFormat proves Parse is the inverse of Format on canonical parts:
// trimmed, non-empty fields and single-line steps, including text that looks like
// structure inside a sentence.
func TestRoundTrip_ParseFormat(t *testing.T) {
	tests := []struct {
		name  string
		parts summaryformat.Parts
	}{
		{name: "documented sample", parts: documentedParts},
		{name: "overview only", parts: summaryformat.Parts{Overview: "Just an overview."}},
		{name: "details only", parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{
			{Label: "Topic A", Summary: "Summary A"},
		}}},
		{name: "next steps only", parts: summaryformat.Parts{NextSteps: []string{"Step one", "Step two"}}},
		{
			name: "label-less detail keeps its empty label",
			parts: summaryformat.Parts{
				Overview: "Overview.",
				Details:  []itx.ZoomMeetingSummaryDetails{{Summary: "A detail without a label."}},
			},
		},
		{
			name:  "detail with an empty summary keeps its label",
			parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{{Label: "Only Label"}}},
		},
		{
			name: "label-less detail followed by labelled ones",
			parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{
				{Summary: "Unlabelled first."},
				{Label: "Second", Summary: "Labelled second."},
			}},
		},
		{
			name: "colons, dashes and '## ' inside sentences are content",
			parts: summaryformat.Parts{
				Overview: "Decision: ship in Q2 - not Q3. Use ## for headings in docs.",
				Details: []itx.ZoomMeetingSummaryDetails{
					{Label: "Budget: 2026 - draft", Summary: "Approved: 15% - pending board. See ## Overview above."},
					{Label: "Risks", Summary: "- not a step\n* nor this one"},
				},
				NextSteps: []string{"Follow up: finance - by Friday", "Write ## Notes section"},
			},
		},
		{
			name: "multi-line overview and summaries with blank lines",
			parts: summaryformat.Parts{
				Overview: "First paragraph.\n\nSecond paragraph.",
				Details: []itx.ZoomMeetingSummaryDetails{
					{Label: "Topic", Summary: "Line one.\nLine two.\n\nLine four."},
					{Label: "Other", Summary: "Single line."},
				},
				NextSteps: []string{"One step"},
			},
		},
		{
			name:  "step text that starts with a second marker keeps it",
			parts: summaryformat.Parts{NextSteps: []string{"- nested-looking step", "* starred"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.parts, summaryformat.Parse(summaryformat.Format(tt.parts)))
		})
	}
}

// TestRoundTrip_SectionCombinations proves the inverse for every combination of the three
// sections present or absent, in both directions: Parse(Format(p)) == p and
// Format(Parse(Format(p))) == Format(p). The all-absent case renders "" and parses back to
// zero Parts.
func TestRoundTrip_SectionCombinations(t *testing.T) {
	const (
		overview = "Team discussed Q1 roadmap."
		step     = "Schedule design review"
	)
	detail := itx.ZoomMeetingSummaryDetails{Label: "Feature Planning", Summary: "Focus on authentication improvements."}

	for mask := 0; mask < 8; mask++ {
		var p summaryformat.Parts
		name := ""
		if mask&1 != 0 {
			p.Overview = overview
			name += "+overview"
		}
		if mask&2 != 0 {
			p.Details = []itx.ZoomMeetingSummaryDetails{detail}
			name += "+details"
		}
		if mask&4 != 0 {
			p.NextSteps = []string{step}
			name += "+steps"
		}
		if name == "" {
			name = "all absent"
		}

		t.Run(name, func(t *testing.T) {
			text := summaryformat.Format(p)
			got := summaryformat.Parse(text)
			assert.Equal(t, p, got, "Parse(Format(p)) == p")
			assert.Equal(t, text, summaryformat.Format(got), "Format(Parse(t)) == t")

			// Each present section is in the text, each absent one is not.
			assert.Equal(t, p.Overview != "", strings.Contains(text, summaryformat.HeadingOverview))
			assert.Equal(t, len(p.Details) > 0, strings.Contains(text, summaryformat.HeadingKeyTopics))
			assert.Equal(t, len(p.NextSteps) > 0, strings.Contains(text, summaryformat.HeadingNextSteps))
		})
	}
}

// TestRoundTrip_FormatParse proves Format(Parse(t)) == t for the documented markdown, so
// an unedited round trip through the proxy leaves the text byte for byte as the index
// carries it.
func TestRoundTrip_FormatParse(t *testing.T) {
	assert.Equal(t, documentedSample, summaryformat.Format(summaryformat.Parse(documentedSample)))
}

// TestRoundTrip_TolerantInputs proves that text the grammar tolerates (rather than
// writes) parses to a stable structure: Parse(Format(Parse(t))) == Parse(t), and the
// recovered structure is the intended one.
func TestRoundTrip_TolerantInputs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want summaryformat.Parts
	}{
		{
			name: "no headings at all is an overview",
			text: "Just some text the user typed.\n\nWith two paragraphs.",
			want: summaryformat.Parts{Overview: "Just some text the user typed.\n\nWith two paragraphs."},
		},
		{
			name: "deleted ## Overview heading",
			text: "Team discussed Q1 roadmap\n\n## Key Topics\n### Feature Planning\nPrioritized auth improvements\n\n## Next Steps\n- Schedule design review",
			want: summaryformat.Parts{
				Overview:  "Team discussed Q1 roadmap",
				Details:   []itx.ZoomMeetingSummaryDetails{{Label: "Feature Planning", Summary: "Prioritized auth improvements"}},
				NextSteps: []string{"Schedule design review"},
			},
		},
		{
			name: "CRLF line ends",
			text: "## Overview\r\nTeam discussed Q1 roadmap\r\n\r\n## Key Topics\r\n### Feature Planning\r\nPrioritized auth improvements\r\n\r\n### Bug Review\r\nIdentified 3 critical issues\r\n\r\n## Next Steps\r\n- Schedule design review\r\n- Update roadmap doc\r\n",
			want: documentedParts,
		},
		{
			name: "steps without dashes, with other bullets, and blank lines",
			text: "## Next Steps\nSchedule design review\n\n* Update roadmap doc\n• Assign bug fixes\n-\n",
			want: summaryformat.Parts{NextSteps: []string{"Schedule design review", "Update roadmap doc", "Assign bug fixes"}},
		},
		{
			name: "unbulleted step starting with a dash is kept verbatim",
			text: "## Next Steps\n-5% budget",
			want: summaryformat.Parts{NextSteps: []string{"-5% budget"}},
		},
		{
			name: "unbulleted step starting with two dashes is kept verbatim",
			text: "## Next Steps\n--dry-run first",
			want: summaryformat.Parts{NextSteps: []string{"--dry-run first"}},
		},
		{
			name: "surrounding whitespace on headings and content",
			text: "  ## Overview  \n  Overview text.  \n\n ## Key Topics \n ###  Topic  \n  Summary text.\n ## Next Steps \n  -  Step  ",
			want: summaryformat.Parts{
				Overview:  "Overview text.",
				Details:   []itx.ZoomMeetingSummaryDetails{{Label: "Topic", Summary: "Summary text."}},
				NextSteps: []string{"Step"},
			},
		},
		{
			name: "unknown '## ' heading is content of the current section",
			text: "## Overview\nText.\n## Decisions\nMore text.",
			want: summaryformat.Parts{Overview: "Text.\n## Decisions\nMore text."},
		},
		{
			name: "headings are case-sensitive",
			text: "## overview\nText.\n## NEXT STEPS\n- not a step",
			want: summaryformat.Parts{Overview: "## overview\nText.\n## NEXT STEPS\n- not a step"},
		},
		{
			name: "repeated headings append to their section",
			text: "## Overview\nFirst.\n## Next Steps\n- One\n## Overview\nSecond.\n## Next Steps\n- Two",
			want: summaryformat.Parts{Overview: "First.\nSecond.", NextSteps: []string{"One", "Two"}},
		},
		{
			name: "damaged nested shape from the former parser recovers its structure",
			// What repeated edits through the former proxy parser left in ITX: the whole
			// markdown stored as the overview, re-rendered under a new "## Overview".
			text: "## Overview\n## Overview\nTeam discussed Q1 roadmap\n\n## Key Topics\n### Feature Planning\nPrioritized auth improvements\n\n### Bug Review\nIdentified 3 critical issues\n\n## Next Steps\n- Schedule design review\n- Update roadmap doc",
			want: documentedParts,
		},
		{
			name: "a detail whose label and summary are both empty is dropped",
			text: "## Key Topics\n###\n\n### Kept\nText.",
			want: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{{Label: "Kept", Summary: "Text."}}},
		},
		{
			name: "only a Key Topics heading parses to zero parts",
			text: "## Key Topics",
			want: summaryformat.Parts{},
		},
		{
			name: "empty text",
			text: "",
			want: summaryformat.Parts{},
		},
		{
			name: "whitespace-only text",
			text: "  \n\t\n",
			want: summaryformat.Parts{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summaryformat.Parse(tt.text)
			assert.Equal(t, tt.want, got, "Parse")
			assert.Equal(t, got, summaryformat.Parse(summaryformat.Format(got)), "Parse(Format(Parse(t))) == Parse(t)")
		})
	}
}
