// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// RecordingFileFetcher downloads a text recording file (such as a saved chat) from
// the provider link held in the recording record, on behalf of an authorized caller.
//
// Implementations bound the fetch (timeout, byte cap, text content only) and must
// never log or return the link: the errors they return carry no URL, host or file
// name, so a caller may log them and may pass a generic message to the client.
type RecordingFileFetcher interface {
	// FetchText fetches the file at downloadURL and returns at most maxBytes of its
	// body. truncated is true when the body was longer than maxBytes; data then holds
	// the first maxBytes bytes and the caller decides where to cut. An error means
	// the file could not be fetched as text and the caller must fail the request.
	FetchText(ctx context.Context, downloadURL string, maxBytes int64) (data []byte, truncated bool, err error)
}
