// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

func TestNewPreferredEmailReply(t *testing.T) {
	t.Run("nil selection yields null fields (use primary)", func(t *testing.T) {
		reply := newPreferredEmailReply(nil)
		assert.Nil(t, reply.EmailID)
		assert.Nil(t, reply.Email)

		data, err := json.Marshal(reply)
		require.NoError(t, err)
		assert.JSONEq(t, `{"email_id":null,"email":null}`, string(data))
	})

	t.Run("empty EmailID yields null fields", func(t *testing.T) {
		reply := newPreferredEmailReply(&domain.PreferredEmail{Email: "x@work.com"})
		assert.Nil(t, reply.EmailID)
		assert.Nil(t, reply.Email)
	})

	t.Run("populated selection yields both fields", func(t *testing.T) {
		reply := newPreferredEmailReply(&domain.PreferredEmail{EmailID: "e1", Email: "alice@work.com"})
		require.NotNil(t, reply.EmailID)
		require.NotNil(t, reply.Email)
		assert.Equal(t, "e1", *reply.EmailID)
		assert.Equal(t, "alice@work.com", *reply.Email)
	})
}

func TestPreferredEmailRequest_Decoding(t *testing.T) {
	t.Run("null email_id decodes to nil pointer", func(t *testing.T) {
		var req preferredEmailRequest
		require.NoError(t, json.Unmarshal([]byte(`{"token":"jwt","email_id":null}`), &req))
		assert.Equal(t, "jwt", req.Token)
		assert.Nil(t, req.EmailID)
	})

	t.Run("omitted email_id decodes to nil pointer", func(t *testing.T) {
		var req preferredEmailRequest
		require.NoError(t, json.Unmarshal([]byte(`{"token":"jwt"}`), &req))
		assert.Nil(t, req.EmailID)
	})

	t.Run("concrete email_id decodes to its value", func(t *testing.T) {
		var req preferredEmailRequest
		require.NoError(t, json.Unmarshal([]byte(`{"token":"jwt","email_id":"e1"}`), &req))
		require.NotNil(t, req.EmailID)
		assert.Equal(t, "e1", *req.EmailID)
	})

	t.Run("email address decodes to its value", func(t *testing.T) {
		var req preferredEmailRequest
		require.NoError(t, json.Unmarshal([]byte(`{"token":"jwt","email":"alice@work.com"}`), &req))
		require.NotNil(t, req.Email)
		assert.Equal(t, "alice@work.com", *req.Email)
		assert.Nil(t, req.EmailID)
	})
}
