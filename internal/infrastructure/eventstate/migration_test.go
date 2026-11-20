// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type metadataStream struct {
	jetstream.Stream
	info jetstream.StreamInfo
}

func (s *metadataStream) CachedInfo() *jetstream.StreamInfo { return &s.info }

type fakeMessageRequester struct {
	request func(context.Context, string, []byte) (*nats.Msg, error)
}

func (r fakeMessageRequester) RequestWithContext(ctx context.Context, subject string, data []byte) (*nats.Msg, error) {
	return r.request(ctx, subject, data)
}

func TestNatsStateSourceNext(t *testing.T) {
	stream := &metadataStream{info: jetstream.StreamInfo{Config: jetstream.StreamConfig{Name: "KV_v1-mappings", AllowDirect: true}}}
	t.Run("uses leader API despite direct-enabled metadata", func(t *testing.T) {
		source := natsStateSource{stream: stream, apiPrefix: "$JS.test.API.", requester: fakeMessageRequester{request: func(_ context.Context, subject string, data []byte) (*nats.Msg, error) {
			require.Equal(t, "$JS.test.API.STREAM.MSG.GET.KV_v1-mappings", subject)
			require.JSONEq(t, `{"seq":7,"next_by_subj":"$KV.v1-mappings.v1_meetings.>"}`, string(data))
			body, err := json.Marshal(map[string]any{"message": map[string]any{"subject": "$KV.v1-mappings.v1_meetings.test", "seq": 19, "data": []byte("!del"), "hdrs": []byte("NATS/1.0\r\nKV-Operation: DEL\r\n\r\n")}})
			require.NoError(t, err)
			return &nats.Msg{Data: body}, nil
		}}}
		msg, err := source.Next(context.Background(), 7, "$KV.v1-mappings.v1_meetings.>")
		require.NoError(t, err)
		require.Equal(t, uint64(19), msg.Sequence)
		require.Equal(t, "!del", string(msg.Data))
		require.Equal(t, "DEL", msg.Header.Get("KV-Operation"))
		require.True(t, stream.info.Config.AllowDirect, "must not alter shared stream configuration")
	})
	t.Run("leader missing message remains a scan-end sentinel", func(t *testing.T) {
		source := natsStateSource{stream: stream, apiPrefix: "$JS.API.", requester: fakeMessageRequester{request: func(context.Context, string, []byte) (*nats.Msg, error) {
			return &nats.Msg{Data: []byte(`{"error":{"code":404,"err_code":10037,"description":"no message found"}}`)}, nil
		}}}
		_, err := source.Next(context.Background(), 20, "$KV.v1-mappings.v1_meetings.>")
		require.ErrorIs(t, err, jetstream.ErrMsgNotFound)
	})
}
