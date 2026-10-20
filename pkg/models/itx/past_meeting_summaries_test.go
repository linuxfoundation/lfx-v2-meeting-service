// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// TestPastMeetingSummaryResponse_EmailSentDecode pins the wire contract the
// summary converter rests on: email_sent is a pointer so a present `false` and
// an absent key decode differently. A non-pointer field would collapse both to
// false and the proxy would report a value ITX never sent.
func TestPastMeetingSummaryResponse_EmailSentDecode(t *testing.T) {
	t.Run("present false decodes to a non-nil false", func(t *testing.T) {
		var resp itx.PastMeetingSummaryResponse
		require.NoError(t, json.Unmarshal([]byte(`{"id":"00000000-0000-0000-0000-000000000001","email_sent":false}`), &resp))

		require.NotNil(t, resp.EmailSent)
		assert.False(t, *resp.EmailSent)
	})

	t.Run("absent key decodes to nil", func(t *testing.T) {
		var resp itx.PastMeetingSummaryResponse
		require.NoError(t, json.Unmarshal([]byte(`{"id":"00000000-0000-0000-0000-000000000001"}`), &resp))

		assert.Nil(t, resp.EmailSent)
	})
}
