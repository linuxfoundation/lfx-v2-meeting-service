// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package summaryformat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/summaryformat"
)

// TestFormat_Goldens pins Format to the bytes the index writer produced before the
// formatter moved here: every expected string was captured by running the former
// cmd/meeting-api/eventing buildSummaryMarkdown over the same input on the base commit.
// A failure here means already-indexed documents would differ from freshly indexed ones.
func TestFormat_Goldens(t *testing.T) {
	tests := []struct {
		name  string
		parts summaryformat.Parts
		want  string
	}{
		{
			name: "documented sample (docs/event-processing.md)",
			parts: summaryformat.Parts{
				Overview: "Team discussed Q1 roadmap",
				Details: []itx.ZoomMeetingSummaryDetails{
					{Label: "Feature Planning", Summary: "Prioritized auth improvements"},
					{Label: "Bug Review", Summary: "Identified 3 critical issues"},
				},
				NextSteps: []string{"Schedule design review", "Update roadmap doc"},
			},
			want: "## Overview\nTeam discussed Q1 roadmap\n\n## Key Topics\n### Feature Planning\nPrioritized auth improvements\n\n### Bug Review\nIdentified 3 critical issues\n\n## Next Steps\n- Schedule design review\n- Update roadmap doc",
		},
		{
			name:  "overview only",
			parts: summaryformat.Parts{Overview: "Just an overview."},
			want:  "## Overview\nJust an overview.",
		},
		{
			name: "details only",
			parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{
				{Label: "Topic A", Summary: "Summary A"},
				{Label: "Topic B", Summary: "Summary B"},
			}},
			want: "## Key Topics\n### Topic A\nSummary A\n\n### Topic B\nSummary B",
		},
		{
			name:  "next steps only",
			parts: summaryformat.Parts{NextSteps: []string{"Step one", "Step two"}},
			want:  "## Next Steps\n- Step one\n- Step two",
		},
		{
			name: "label-less detail",
			parts: summaryformat.Parts{
				Overview: "Overview.",
				Details:  []itx.ZoomMeetingSummaryDetails{{Summary: "A detail without a label."}},
			},
			want: "## Overview\nOverview.\n\n## Key Topics\nA detail without a label.",
		},
		{
			name: "detail with an empty summary",
			parts: summaryformat.Parts{
				Overview: "Overview.",
				Details:  []itx.ZoomMeetingSummaryDetails{{Label: "Only Label"}},
			},
			want: "## Overview\nOverview.\n\n## Key Topics\n### Only Label",
		},
		{
			name:  "multi-line overview",
			parts: summaryformat.Parts{Overview: "First paragraph.\n\nSecond paragraph."},
			want:  "## Overview\nFirst paragraph.\n\nSecond paragraph.",
		},
		{
			name: "multi-line detail summary",
			parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{
				{Label: "Topic", Summary: "Line one.\nLine two.\n\nLine four."},
			}},
			want: "## Key Topics\n### Topic\nLine one.\nLine two.\n\nLine four.",
		},
		{
			name: "empty slices with a non-empty overview",
			parts: summaryformat.Parts{
				Overview:  "Only overview.",
				Details:   []itx.ZoomMeetingSummaryDetails{},
				NextSteps: []string{},
			},
			want: "## Overview\nOnly overview.",
		},
		{
			name:  "all empty",
			parts: summaryformat.Parts{},
			want:  "",
		},
		{
			name:  "empty step is skipped",
			parts: summaryformat.Parts{NextSteps: []string{"Step one", "", "Step three"}},
			want:  "## Next Steps\n- Step one\n- Step three",
		},
		{
			name:  "detail with empty label and summary still opens the section",
			parts: summaryformat.Parts{Details: []itx.ZoomMeetingSummaryDetails{{}}},
			want:  "## Key Topics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, summaryformat.Format(tt.parts))
		})
	}
}
