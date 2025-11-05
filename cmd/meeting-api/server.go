// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
	"time"

	goahttp "goa.design/goa/v3/http"

	genhttp "github.com/linuxfoundation/lfx-v2-meeting-service/gen/http/meeting_service/server"
	genquerysvc "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/middleware"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// uploadMeetingAttachmentMetadata holds file metadata extracted during multipart decoding
type uploadMeetingAttachmentMetadata struct {
	FileName    string
	ContentType string
}

// uploadMeetingAttachmentDecoder decodes multipart form data for file uploads
func uploadMeetingAttachmentDecoder(mr *multipart.Reader, p **genquerysvc.UploadMeetingAttachmentPayload) error {
	// Initialize the payload if it's nil
	if *p == nil {
		*p = &genquerysvc.UploadMeetingAttachmentPayload{}
	}

	var metadata uploadMeetingAttachmentMetadata

	// Read all parts from the multipart form
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch part.FormName() {
		case "file":
			// Capture file metadata from the part
			metadata.FileName = part.FileName()
			metadata.ContentType = part.Header.Get("Content-Type")
			if metadata.ContentType == "" {
				metadata.ContentType = "application/octet-stream"
			}

			// Read the file data
			fileData, err := io.ReadAll(part)
			if err != nil {
				return err
			}
			(*p).File = fileData
		case "description":
			// Read the description field
			descData, err := io.ReadAll(part)
			if err != nil {
				return err
			}
			desc := string(descData)
			(*p).Description = &desc
		}
	}

	// Store metadata in a way the handler can access it
	// We'll use a package-level map temporarily (not ideal for production, but works for now)
	if metadata.FileName != "" {
		attachmentMetadataStore.Store(*p, metadata)
	}

	return nil
}

// attachmentMetadataStore temporarily stores file metadata during request processing
var attachmentMetadataStore sync.Map

// createPastMeetingAttachmentDecoder decodes multipart form data for past meeting attachment creation
func createPastMeetingAttachmentDecoder(mr *multipart.Reader, p **genquerysvc.CreatePastMeetingAttachmentPayload) error {
	// Initialize the payload if it's nil
	if *p == nil {
		*p = &genquerysvc.CreatePastMeetingAttachmentPayload{}
	}

	var metadata uploadPastMeetingAttachmentMetadata

	// Read all parts from the multipart form
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch part.FormName() {
		case "file":
			// Capture file metadata from the part
			metadata.FileName = part.FileName()
			metadata.ContentType = part.Header.Get("Content-Type")
			if metadata.ContentType == "" {
				metadata.ContentType = "application/octet-stream"
			}

			// Read the file data
			fileData, err := io.ReadAll(part)
			if err != nil {
				return err
			}
			(*p).File = fileData
		case "description":
			// Read the description field
			descData, err := io.ReadAll(part)
			if err != nil {
				return err
			}
			desc := string(descData)
			(*p).Description = &desc
		case "source_object_uid":
			// Read the source_object_uid field
			sourceData, err := io.ReadAll(part)
			if err != nil {
				return err
			}
			sourceUID := string(sourceData)
			(*p).SourceObjectUID = &sourceUID
		}
	}

	// Store metadata in a way the handler can access it
	if metadata.FileName != "" {
		pastMeetingAttachmentMetadataStore.Store(*p, metadata)
	}

	return nil
}

// pastMeetingAttachmentMetadataStore temporarily stores file metadata during request processing for past meeting attachments
var pastMeetingAttachmentMetadataStore sync.Map

// setupHTTPServer configures and starts the HTTP server
func setupHTTPServer(flags flags, svc *MeetingsAPI, gracefulCloseWG *sync.WaitGroup) *http.Server {
	// Wrap it in the generated endpoints
	endpoints := genquerysvc.NewEndpoints(svc)

	// Build an HTTP handler
	mux := goahttp.NewMuxer()
	requestDecoder := goahttp.RequestDecoder
	responseEncoder := goahttp.ResponseEncoder

	// Create a custom encoder that sets ETag header for get-one-meeting
	// and proper headers for file downloads
	customEncoder := func(ctx context.Context, w http.ResponseWriter) goahttp.Encoder {
		encoder := responseEncoder(ctx, w)

		// Check if we have an ETag in the context
		if etag, ok := ctx.Value(constants.ETagContextID).(string); ok {
			w.Header().Set("ETag", etag)
		}

		// Check if we have attachment metadata for file downloads
		if metadata, ok := getDownloadAttachmentMetadata(ctx); ok {
			// Set the correct Content-Type based on the file's actual type
			w.Header().Set("Content-Type", metadata.ContentType)
			// Set Content-Disposition header with the original filename
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", metadata.FileName))
			// Clean up the metadata after use
			deleteDownloadAttachmentMetadata(ctx)

			// Return a custom encoder that writes raw bytes instead of JSON-encoding them
			return goahttp.EncodingFunc(func(v any) error {
				if bytes, ok := v.([]byte); ok {
					_, err := w.Write(bytes)
					return err
				}
				// Fallback to regular encoding if not bytes
				return encoder.Encode(v)
			})
		}

		// Check if we have past meeting attachment metadata for file downloads
		if metadata, ok := getPastMeetingDownloadAttachmentMetadata(ctx); ok {
			// Set the correct Content-Type based on the file's actual type
			w.Header().Set("Content-Type", metadata.ContentType)
			// Set Content-Disposition header with the original filename
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", metadata.FileName))
			// Clean up the metadata after use
			deletePastMeetingDownloadAttachmentMetadata(ctx)

			// Return a custom encoder that writes raw bytes instead of JSON-encoding them
			return goahttp.EncodingFunc(func(v any) error {
				if bytes, ok := v.([]byte); ok {
					_, err := w.Write(bytes)
					return err
				}
				// Fallback to regular encoding if not bytes
				return encoder.Encode(v)
			})
		}

		return encoder
	}

	koDataPath := os.Getenv("KO_DATA_PATH")
	if koDataPath == "" {
		koDataPath = "../../gen/http"
	}

	koDataDir := http.Dir(koDataPath)

	genHttpServer := genhttp.New(
		endpoints,
		mux,
		requestDecoder,
		customEncoder,
		nil,
		nil,
		uploadMeetingAttachmentDecoder,
		createPastMeetingAttachmentDecoder,
		koDataDir,
		koDataDir,
		koDataDir,
		koDataDir,
	)

	// Mount the handler on the mux
	genhttp.Mount(mux, genHttpServer)

	var handler http.Handler = mux

	// Add HTTP middleware
	// Note: Order matters - RequestIDMiddleware should come first in the chain,
	// so it should be the last middleware added to the handler since it is executed in reverse order.
	handler = middleware.WebhookBodyCaptureMiddleware()(handler)
	handler = middleware.RequestLoggerMiddleware()(handler)
	handler = middleware.RequestIDMiddleware()(handler)
	handler = middleware.AuthorizationMiddleware()(handler)

	// Set up http listener in a goroutine using provided command line parameters.
	var addr string
	if flags.Bind == "*" {
		addr = ":" + flags.Port
	} else {
		addr = flags.Bind + ":" + flags.Port
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
	}
	gracefulCloseWG.Add(1)
	go func() {
		slog.With("addr", addr).Debug("starting http server, listening on port " + flags.Port)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.With(logging.ErrKey, err).Error("http listener error")
			os.Exit(1)
		}
		// Because ErrServerClosed is *immediately* returned when Shutdown is
		// called, not when when Shutdown completes, this must not yet decrement
		// the wait group.
	}()

	return httpServer
}
