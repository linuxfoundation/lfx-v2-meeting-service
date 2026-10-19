// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
)

type pastMeetingCleanupPublisher struct {
	recordingTestPublisher
	summaries      int
	parentPuts     int
	summaryAccess  []string
	attachments    []string
	lastAttachment *models.PastMeetingAttachmentEventData
	participants   []string
	accessDeletes  int
	accessSubjects []string
	events         []string
	parentErr      error
	recordingErr   error
	transcriptErr  error
	flushErr       error
	onFlush        func()
	failSubject    string
	deleteErr      error
	accessErr      error
	beforeArtifact func(string)
	onRecording    func()
	onSummary      func()
	onAttachment   func()
	onParticipant  func()
	onDelete       func(string)
	beforeDelete   func(string)
	onAccess       func()
	beforeAccess   func(string)
	onParent       func()
}

func (p *pastMeetingCleanupPublisher) Flush(context.Context) error {
	if p.onFlush != nil {
		p.onFlush()
	}
	return p.flushErr
}

func (p *pastMeetingCleanupPublisher) PublishIndexerDelete(_ context.Context, subject, id string) error {
	if p.beforeDelete != nil {
		p.beforeDelete(subject)
	}
	if subject == p.failSubject {
		if p.deleteErr != nil {
			return p.deleteErr
		}
		return errors.New("indexer unavailable")
	}
	p.indexerDeletes = append(p.indexerDeletes, subject+":"+id)
	p.events = append(p.events, "delete:"+subject+":"+id)
	if p.onDelete != nil {
		p.onDelete(subject)
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishAccessDelete(_ context.Context, subject string, _ []byte) error {
	if p.beforeAccess != nil {
		p.beforeAccess(subject)
	}
	if p.accessErr != nil {
		return p.accessErr
	}
	p.accessDeletes++
	p.accessSubjects = append(p.accessSubjects, subject)
	p.events = append(p.events, "access:"+subject)
	if p.onAccess != nil {
		p.onAccess()
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingEvent(_ context.Context, _ string, _ *models.PastMeetingEventData) error {
	if p.parentErr != nil {
		return p.parentErr
	}
	p.parentPuts++
	p.events = append(p.events, "parent-put")
	if p.onParent != nil {
		p.onParent()
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingSummaryEvent(_ context.Context, _ string, _ *models.SummaryEventData, access string) error {
	if p.beforeArtifact != nil {
		p.beforeArtifact(eventing.IndexV1PastMeetingSummarySubject)
	}
	p.summaries++
	p.events = append(p.events, "summary-put")
	p.summaryAccess = append(p.summaryAccess, access)
	if p.onSummary != nil {
		p.onSummary()
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingRecordingEvent(_ context.Context, _ string, data *models.RecordingEventData) error {
	if p.beforeArtifact != nil {
		p.beforeArtifact(eventing.IndexV1PastMeetingRecordingSubject)
	}
	if p.recordingErr != nil {
		return p.recordingErr
	}
	p.recordingData = data
	p.events = append(p.events, "recording-put")
	if p.onRecording != nil {
		p.onRecording()
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingTranscriptEvent(_ context.Context, _ string, data *models.TranscriptEventData) error {
	if p.transcriptErr != nil {
		return p.transcriptErr
	}
	p.transcriptData = data
	p.events = append(p.events, "transcript-put")
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingAttachmentEvent(_ context.Context, _ string, data *models.PastMeetingAttachmentEventData) error {
	if p.beforeArtifact != nil {
		p.beforeArtifact(eventing.IndexV1PastMeetingAttachmentSubject)
	}
	p.attachments = append(p.attachments, data.UID)
	p.lastAttachment = data
	p.events = append(p.events, "attachment-put")
	if p.onAttachment != nil {
		p.onAttachment()
	}
	return nil
}

func (p *pastMeetingCleanupPublisher) PublishPastMeetingParticipantEvent(_ context.Context, _ string, data *models.PastMeetingParticipantEventData) error {
	p.participants = append(p.participants, data.UID)
	p.events = append(p.events, "participant-put:"+data.UID)
	if p.onParticipant != nil {
		p.onParticipant()
	}
	return nil
}

func TestPastMeetingDelete_RetractsArtifacts(t *testing.T) {
	const (
		parentID        = testSummaryMeetOcc
		otherParentID   = "12345678902-1700000000000"
		legacySummaryID = "00000000-0000-0000-0000-000000000005"
		attachmentID    = "00000000-0000-0000-0000-000000000002"
		inviteeID       = "00000000-0000-0000-0000-000000000003"
		attendeeID      = "00000000-0000-0000-0000-000000000004"
	)
	parentMapping := "v1_past_meetings." + parentID
	objects := newMemKV()
	mappings := newMemKV()
	deletedParent := map[string]any{
		"meeting_and_occurrence_id": parentID,
		"proj_id":                   "proj-sfid",
		"recording_access":          "public",
		"transcript_access":         "public",
		"ai_summary_access":         "public",
		"_sdc_deleted_at":           "2026-01-02T00:00:00Z",
	}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, deletedParent)
	objects.data["itx-zoom-past-meetings-summaries."+legacySummaryID] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": parentID})
	objects.data["itx-zoom-past-meetings-attachments."+attachmentID] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": parentID})
	objects.data["itx-zoom-past-meetings-attachments.other"] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": otherParentID})
	objects.data["itx-zoom-past-meetings-invitees."+inviteeID] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": parentID})
	objects.data["itx-zoom-past-meetings-attendees."+attendeeID] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": parentID})
	mappings.data[parentMapping] = []byte("1")
	mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	t.Run("deletes all children before marking the parent deleted", func(t *testing.T) {
		require.False(t, handleKVPut(context.Background(), testPastMeetingKey, deletedParent, h))
		assert.Equal(t, tombstoneMarker, string(mappings.data[parentMapping]))
		assert.Equal(t, 1, publisher.accessDeletes)
		assert.ElementsMatch(t, []string{
			eventing.IndexV1PastMeetingRecordingSubject + ":" + parentID,
			eventing.IndexV1PastMeetingTranscriptSubject + ":" + parentID,
			eventing.IndexV1PastMeetingSummarySubject + ":" + testSummaryID,
			eventing.IndexV1PastMeetingSummarySubject + ":" + legacySummaryID,
			eventing.IndexV1PastMeetingAttachmentSubject + ":" + attachmentID,
			eventing.IndexV1PastMeetingParticipantSubject + ":" + inviteeID,
			eventing.IndexV1PastMeetingParticipantSubject + ":" + attendeeID,
			eventing.IndexV1PastMeetingSubject + ":" + parentID,
		}, publisher.indexerDeletes)
		assert.False(t, slices.Contains(publisher.indexerDeletes, eventing.IndexV1PastMeetingAttachmentSubject+":other"))
	})

	t.Run("late public recording and summary events cannot restore children", func(t *testing.T) {
		recording := map[string]any{"meeting_and_occurrence_id": parentID, "proj_id": "proj-sfid", "recording_access": "public"}
		summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": parentID, "approved": true}
		require.False(t, h.handlePastMeetingRecordingUpdate(context.Background(), "itx-zoom-past-meetings-recordings."+parentID, recording))
		require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
		assert.Nil(t, publisher.recordingData)
		assert.Zero(t, publisher.summaries)
		assert.Equal(t, tombstoneMarker, string(mappings.data[parentMapping]))
		assert.Equal(t, []byte(`["`+testSummaryID+`","`+legacySummaryID+`"]`), mappings.data[testSummaryIndexKey])
	})
	t.Run("redelivery skips completed child scans", func(t *testing.T) {
		previousDeletes := len(publisher.indexerDeletes)
		require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
		assert.Len(t, publisher.indexerDeletes, previousDeletes)
		assertPastMeetingCleanupComplete(t, mappings, parentID)
	})
}

func TestPastMeetingDelete_ChildCleanupFailureRetriesBeforeTombstone(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z",
	})
	parentMapping := "v1_past_meetings." + testSummaryMeetOcc
	mappings.data[parentMapping] = []byte("1")
	mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
	publisher := &pastMeetingCleanupPublisher{failSubject: eventing.IndexV1PastMeetingSummarySubject}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	assert.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Equal(t, "1", string(mappings.data[parentMapping]), "failed cleanup must leave delete retryable")
	assert.Equal(t, "1", string(mappings.data[pastMeetingRestorePendingPrefix+testSummaryMeetOcc]), "intent must precede child cleanup")
	assert.Zero(t, publisher.accessDeletes)

	publisher.failSubject = ""
	assert.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Equal(t, tombstoneMarker, string(mappings.data[parentMapping]))
	assert.Equal(t, 1, publisher.accessDeletes)
}

func TestPastMeetingDelete_PartialCleanupRestoresChildrenWithoutTombstone(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	objects.data["itx-zoom-past-meetings-attachments.attachment-1"] = mustMarshalJSON(t, map[string]any{
		"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc,
	})
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte("1")
	publisher := &pastMeetingCleanupPublisher{failSubject: eventing.IndexV1PastMeetingSubject}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	assert.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Equal(t, "1", string(mappings.data[pastMeetingRestorePendingPrefix+testSummaryMeetOcc]))
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	assert.Equal(t, []byte(`["attachment-1"]`), mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc])

	publisher.failSubject = ""
	delete(parent, "_sdc_deleted_at")
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	assert.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
	assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
}

func TestPastMeetingDelete_RedeliveryAfterRestoreCannotDeleteLiveParent(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	publisher := &pastMeetingCleanupPublisher{failSubject: eventing.IndexV1PastMeetingRecordingSubject}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	assert.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	delete(parent, "_sdc_deleted_at")
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	assert.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	deletes := len(publisher.indexerDeletes)
	publisher.failSubject = ""
	assert.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Len(t, publisher.indexerDeletes, deletes, "stale delete cannot retract restored documents")
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
}

func TestPastMeetingDelete_RestoreDuringCleanupRepublishesParentAndChildren(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	objects.data["itx-zoom-past-meetings-attachments.attachment-1"] = mustMarshalJSON(t, map[string]any{
		"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc,
	})
	publisher := &pastMeetingCleanupPublisher{}
	publisher.onDelete = func(subject string) {
		if subject == eventing.IndexV1PastMeetingSubject {
			delete(parent, "_sdc_deleted_at")
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
		}
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Positive(t, publisher.parentPuts, "the restored parent access must be republished after cleanup")
	assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
}

func TestPastMeetingParticipant_RestoreDuringCompensationRepairsAccess(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
	objects.data["itx-zoom-past-meetings-invitees.invitee-1"] = mustMarshalJSON(t, participant)
	mappings.data[pastMeetingInviteeIndexPrefix+testSummaryMeetOcc+".invitee-1"] = []byte("1")
	publisher := &pastMeetingCleanupPublisher{}
	publisher.onAccess = func() {
		delete(parent, "_sdc_deleted_at")
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", participant))
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Positive(t, publisher.parentPuts, "the parent-wide access revoked in flight must be republished")
	assert.Equal(t, []string{"invitee-1"}, publisher.participants)
}

func TestPastMeetingUpdate_MappingReadFailureCannotHideRestore(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	mappingKey := "v1_past_meetings." + testSummaryMeetOcc
	mappings.data[mappingKey] = []byte(tombstoneMarker)
	mappings.getErrs[mappingKey] = errors.New("kv unavailable")
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = &pastMeetingCleanupPublisher{}

	assert.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, tombstoneMarker, string(mappings.data[mappingKey]))
}

func TestPastMeetingDelete_ChildListingFailureRetries(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	mappingKey := "v1_past_meetings." + testSummaryMeetOcc
	mappings.data[mappingKey] = []byte("1")
	objects.listErrs["itx-zoom-past-meetings-attachments.>"] = errors.New("kv unavailable")
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = &pastMeetingCleanupPublisher{}

	assert.True(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Equal(t, "1", string(mappings.data[mappingKey]))
}

func TestPastMeetingDelete_ReplaysCleanupForExistingTombstone(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSummarySubject+":"+testSummaryID)
	assert.Equal(t, 1, publisher.accessDeletes)
}

func TestPastMeetingDelete_CorruptUnrelatedChildDoesNotBlockCleanup(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data["itx-zoom-past-meetings-invitees.corrupt"] = []byte{0xc1}
	objects.data["itx-zoom-past-meetings-invitees.valid"] = mustMarshalJSON(t, map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc})
	h := newRetriggerHandler(nil, objects, mappings)
	publisher := &pastMeetingCleanupPublisher{}
	h.publisher = publisher

	require.False(t, h.handlePastMeetingDelete(context.Background(), testPastMeetingKey, nil))
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":valid")
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSubject+":"+testSummaryMeetOcc)
	assertPastMeetingCleanupComplete(t, mappings, testSummaryMeetOcc)
}

func TestPastMeetingParticipantIndex_UsesOneKeyPerParticipant(t *testing.T) {
	mappings := newMemKV()
	for _, id := range []string{"invitee-1", "invitee-2"} {
		require.NoError(t, recordPastMeetingParticipantID(context.Background(), mappings, pastMeetingInviteeIndexPrefix, testSummaryMeetOcc, id))
	}
	ids, err := listIndexedPastMeetingChildIDs(context.Background(), mappings, pastMeetingChildKinds[2], testSummaryMeetOcc)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"invitee-1", "invitee-2"}, ids)
	assert.NotContains(t, mappings.data, pastMeetingInviteeIndexPrefix+testSummaryMeetOcc, "no shared JSON array should be written")
}

func TestPastMeetingRestore_RepublishesTrackedAttachmentsAndParticipants(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parentID := testSummaryMeetOcc
	parent := map[string]any{"meeting_and_occurrence_id": parentID, "proj_id": "proj-sfid", "meeting_id": "12345678901"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	objects.data["itx-zoom-past-meetings-attachments.attachment-1"] = mustMarshalJSON(t, map[string]any{
		"id": "attachment-1", "meeting_and_occurrence_id": parentID, "proj_id": "proj-sfid",
	})
	objects.data["itx-zoom-past-meetings-invitees.invitee-1"] = mustMarshalJSON(t, map[string]any{
		"invitee_id": "invitee-1", "meeting_and_occurrence_id": parentID, "proj_id": "proj-sfid",
	})
	objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, map[string]any{
		"id": "attendee-1", "meeting_and_occurrence_id": parentID, "proj_id": "proj-sfid",
	})
	mappings.data["v1_past_meetings."+parentID] = []byte(tombstoneMarker)
	mappings.data[pastMeetingCleanupKeyPrefix+parentID] = []byte("1")
	mappings.data[pastMeetingAttachmentIndexPrefix+parentID] = []byte(`["attachment-1"]`)
	mappings.data[pastMeetingInviteeIndexPrefix+parentID+".invitee-1"] = []byte("1")
	mappings.data[pastMeetingAttendeeIndexPrefix+parentID+".attendee-1"] = []byte("1")
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"attachment-1", "attachment-1"}, publisher.attachments)
	assert.ElementsMatch(t, []string{"invitee-1", "attendee-1", "invitee-1", "attendee-1"}, publisher.participants)
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+parentID]))
	assert.NotContains(t, mappings.data, pastMeetingCleanupKeyPrefix+parentID)

	// Regular updates do not re-run the restore of unchanged children.
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Len(t, publisher.attachments, 2, "a single batched convergence replay follows creation/restoration; ordinary updates add none")
	assert.Len(t, publisher.participants, 4)
}

func TestPastMeetingRestore_RetriesChildrenAfterParentMappingBecomesLive(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	attachmentKey := "itx-zoom-past-meetings-attachments.attachment-1"
	objects.data[attachmentKey] = mustMarshalJSON(t, map[string]any{
		"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc,
	})
	objects.getErrs[attachmentKey] = errors.New("kv unavailable")
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	assert.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	assert.Equal(t, "1", string(mappings.data[pastMeetingRestorePendingPrefix+testSummaryMeetOcc]))
	assert.NotContains(t, mappings.data, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc)

	delete(objects.getErrs, attachmentKey)
	assert.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
	assert.NotContains(t, mappings.data, pastMeetingCleanupKeyPrefix+testSummaryMeetOcc)
	assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
}

func TestPastMeetingCreate_RetriesTrackedChildrenAfterMappingBecomesLive(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	attachmentKey := "itx-zoom-past-meetings-attachments.attachment-1"
	objects.data[attachmentKey] = mustMarshalJSON(t, map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc})
	objects.getErrs[attachmentKey] = errors.New("kv unavailable")
	mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc] = []byte(`["attachment-1"]`)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	assert.True(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, "1", string(mappings.data["v1_past_meetings."+testSummaryMeetOcc]))
	assert.Equal(t, "1", string(mappings.data[pastMeetingRestorePendingPrefix+testSummaryMeetOcc]))

	delete(objects.getErrs, attachmentKey)
	assert.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
	assert.NotContains(t, mappings.data, pastMeetingRestorePendingPrefix+testSummaryMeetOcc)
}

func TestPastMeetingArtifacts_DeletedParentNeverPublishesPublic(t *testing.T) {
	for _, tc := range []struct {
		name       string
		softDelete bool
	}{
		{name: "soft-deleted parent before its delete event", softDelete: true},
		{name: "hard-deleted parent with a tombstone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			if tc.softDelete {
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{
					"proj_id":           "proj-sfid",
					"recording_access":  "public",
					"transcript_access": "public",
					"ai_summary_access": "public",
					"_sdc_deleted_at":   "2026-01-02T00:00:00Z",
				})
			} else {
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
			}
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher

			recording := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "recording_access": "public"}
			summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}
			require.False(t, h.handlePastMeetingRecordingUpdate(context.Background(), "itx-zoom-past-meetings-recordings."+testSummaryMeetOcc, recording))
			require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
			assert.Nil(t, publisher.recordingData)
			assert.Zero(t, publisher.summaries)
			assert.ElementsMatch(t, []string{
				eventing.IndexV1PastMeetingRecordingSubject + ":" + testSummaryMeetOcc,
				eventing.IndexV1PastMeetingTranscriptSubject + ":" + testSummaryMeetOcc,
				eventing.IndexV1PastMeetingSummarySubject + ":" + testSummaryID,
				eventing.IndexV1PastMeetingRecordingSubject + ":" + testSummaryMeetOcc,
				eventing.IndexV1PastMeetingTranscriptSubject + ":" + testSummaryMeetOcc,
				eventing.IndexV1PastMeetingSummarySubject + ":" + testSummaryID,
			}, publisher.indexerDeletes, "initial retraction and claimed repair each replay only this source's idempotent artifact deletes")
			assert.Equal(t, []byte(`["`+testSummaryID+`"]`), mappings.data[testSummaryIndexKey], "the ID must survive for a future restore")
		})
	}
}

func TestPastMeetingAttachment_DeletedParentRetractsLateEvent(t *testing.T) {
	for _, deletion := range []string{"soft", "hard"} {
		t.Run(deletion, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			h := newRetriggerHandler(nil, objects, mappings)
			publisher := &pastMeetingCleanupPublisher{}
			h.publisher = publisher
			publisher.onAttachment = func() {
				if deletion == "hard" {
					delete(objects.data, testPastMeetingKey) // before the parent tombstone
				} else {
					parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
				}
			}
			attachment := map[string]any{
				"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc,
			}
			require.False(t, h.handlePastMeetingAttachmentUpdate(context.Background(), "itx-zoom-past-meetings-attachments.attachment-1", attachment))
			assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
			assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingAttachmentSubject+":attachment-1")
		})
	}
}

func TestPastMeetingAttachment_AbsentOrDeletedParentTracksForLaterCreate(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "deleted"}[deleted], func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
			if deleted {
				parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
				objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			}
			attachmentKey := "itx-zoom-past-meetings-attachments.attachment-1"
			attachment := map[string]any{"id": "attachment-1", "meeting_and_occurrence_id": testSummaryMeetOcc}
			objects.data[attachmentKey] = mustMarshalJSON(t, attachment)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher

			require.False(t, h.handlePastMeetingAttachmentUpdate(context.Background(), attachmentKey, attachment))
			assert.Empty(t, publisher.attachments)
			assert.Equal(t, []byte(`["attachment-1"]`), mappings.data[pastMeetingAttachmentIndexPrefix+testSummaryMeetOcc])

			delete(parent, "_sdc_deleted_at")
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
			if deleted {
				mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
			}
			require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
			assert.Equal(t, []string{"attachment-1"}, publisher.attachments)
		})
	}
}

func TestPastMeetingSummary_AbsentParentIsTrackedForFirstPut(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}
	objects.data[testSummaryKVKey] = mustMarshalJSON(t, summary)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
	assert.Zero(t, publisher.summaries)
	assert.Equal(t, []byte(`["`+testSummaryID+`"]`), mappings.data[testSummaryIndexKey])

	parent := map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc,
		"proj_id":                   "proj-sfid",
		"ai_summary_access":         "public",
	}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, 1, publisher.summaries, "the parent's initial create must publish the tracked summary")
}

func TestPastMeetingSummary_ArrivingAfterDeletionIsTrackedForRestore(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}
	objects.data[testSummaryKVKey] = mustMarshalJSON(t, summary)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
	assert.Zero(t, publisher.summaries)
	assert.Equal(t, []byte(`["`+testSummaryID+`"]`), mappings.data[testSummaryIndexKey])

	delete(parent, "_sdc_deleted_at")
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, 1, publisher.summaries, "the restored parent must find the late summary")
}

func TestPastMeetingSummary_RestoreDuringRegistrationPublishesCurrentParent(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc,
		"proj_id":                   "proj-sfid",
		"ai_summary_access":         "public",
		"_sdc_deleted_at":           "2026-01-02T00:00:00Z",
	}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.onCreate = func(key string) {
		if key == testSummaryIndexKey {
			delete(parent, "_sdc_deleted_at")
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
		}
	}
	h := newRetriggerHandler(nil, objects, mappings)
	publisher := &pastMeetingCleanupPublisher{}
	h.publisher = publisher
	summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}

	require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
	assert.Equal(t, []string{"public"}, publisher.summaryAccess)
	assert.NotContains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSummarySubject+":"+testSummaryID)
}

func TestPastMeetingParticipant_RacedParentDeleteRetractsIndexAndAccess(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	publisher.onParticipant = func() {
		parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	}
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", participant))
	assert.Equal(t, []string{"invitee-1"}, publisher.participants)
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Equal(t, 1, publisher.accessDeletes, "the raced FGA member_put must be compensated")
}

func TestPastMeetingParticipant_FailedCompensationRetriesUnderTombstone(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	publisher := &pastMeetingCleanupPublisher{failSubject: eventing.IndexV1PastMeetingParticipantSubject}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	publisher.onParticipant = func() {
		parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
		mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	}
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
	key := "itx-zoom-past-meetings-invitees.invitee-1"

	assert.True(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, participant))
	publisher.failSubject = ""
	assert.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, participant))
	assert.Equal(t, []string{"invitee-1"}, publisher.participants, "retry must not republish under the tombstone")
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Equal(t, 2, publisher.accessDeletes, "finish the pending publication compensation, then handle the current late delivery with bounded per-source cleanup")
}

func TestPastMeetingParticipant_WithoutProjectTracksDeletedParentForRestore(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid", "_sdc_deleted_at": "2026-01-02T00:00:00Z"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	participant := map[string]any{"invitee_id": "invitee-1", "meeting_and_occurrence_id": testSummaryMeetOcc}
	participantKey := "itx-zoom-past-meetings-invitees.invitee-1"
	objects.data[participantKey] = mustMarshalJSON(t, participant)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), participantKey, participant))
	assert.Empty(t, publisher.participants)
	assert.Equal(t, "1", string(mappings.data[pastMeetingInviteeIndexPrefix+testSummaryMeetOcc+".invitee-1"]))

	delete(parent, "_sdc_deleted_at")
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"invitee-1", "invitee-1"}, publisher.participants, "restore plus one batched convergence replay")
}

func TestPastMeetingParticipant_WithoutProjectRetriesFailedCompensation(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	publisher := &pastMeetingCleanupPublisher{failSubject: eventing.IndexV1PastMeetingParticipantSubject}
	publisher.onParticipant = func() {
		parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	key := "itx-zoom-past-meetings-invitees.invitee-1"

	assert.True(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")))
	publisher.failSubject = ""
	noProject := map[string]any{"invitee_id": "invitee-1", "meeting_and_occurrence_id": testSummaryMeetOcc}
	assert.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), key, noProject))
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Equal(t, 2, publisher.accessDeletes, "pending compensation and the current late delivery each remain independently retryable")
}

func TestPastMeetingParticipant_AbsentParentDoesNotRevokeFirstCreate(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")
	participantKey := "itx-zoom-past-meetings-invitees.invitee-1"
	objects.data[participantKey] = mustMarshalJSON(t, participant)
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), participantKey, participant))
	assert.Empty(t, publisher.participants)
	assert.Zero(t, publisher.accessDeletes, "an unsynced parent must not be revoked")

	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	require.False(t, h.handlePastMeetingUpdate(context.Background(), testPastMeetingKey, parent))
	assert.Equal(t, []string{"invitee-1", "invitee-1"}, publisher.participants, "first create plus one batched convergence replay")
}

func TestPastMeetingParticipant_LiveRestoreOverridesStaleTombstone(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	mappings.data[pastMeetingRestorePendingPrefix+testSummaryMeetOcc] = []byte("1")
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", participant))
	assert.Equal(t, []string{"invitee-1"}, publisher.participants)
	assert.Empty(t, publisher.indexerDeletes)
	assert.Zero(t, publisher.accessDeletes, "a stale tombstone must not revoke restored parent access")
}

func TestPastMeetingParticipant_UsernameChangeRacingDeleteRetractsBothDocuments(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
	mappings.data["v1_participant_by_meeting_user.attendee."+testSummaryMeetOcc+".alice"] = []byte("attendee-1")
	mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+".attendee-1"] = []byte("1")
	objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
	publisher := &pastMeetingCleanupPublisher{}
	publisher.onParticipant = func() {
		parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher
	participant := minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "bob")

	require.False(t, h.handlePastMeetingInviteeUpdate(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", participant))
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":attendee-1")
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Positive(t, publisher.accessDeletes)
}

func TestPastMeetingParticipant_DeleteAfterParentDoesNotRepublishSibling(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc, "_sdc_deleted_at": "2026-01-02T00:00:00Z",
	})
	mappings.data["v1_past_meetings."+testSummaryMeetOcc] = []byte(tombstoneMarker)
	mappings.data["v1_past_meeting_invitees.invitee-1"] = []byte(buildRegistrantMappingValue("invitee-1", "alice", testSummaryMeetOcc))
	mappings.data["v1_participant_by_meeting_user.attendee."+testSummaryMeetOcc+".alice"] = []byte("attendee-1")
	objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
	publisher := &pastMeetingCleanupPublisher{}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingInviteeDelete(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")))
	assert.Empty(t, publisher.participants, "a deleted parent must not get a sibling member_put")
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":invitee-1")
	assert.Equal(t, tombstoneMarker, string(mappings.data["v1_participant_by_meeting_user.invitee."+testSummaryMeetOcc+".alice"]))
}

func TestPastMeetingParticipant_PartialDeleteRacingParentRetractsSibling(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.data["v1_participant_by_meeting_user.attendee."+testSummaryMeetOcc+".alice"] = []byte("attendee-1")
	mappings.data[pastMeetingAttendeeIndexPrefix+testSummaryMeetOcc+".attendee-1"] = []byte("1")
	objects.data["itx-zoom-past-meetings-attendees.attendee-1"] = mustMarshalJSON(t, minimalAttendeeV1Data("attendee-1", testSummaryMeetOcc, "alice"))
	publisher := &pastMeetingCleanupPublisher{}
	publisher.onParticipant = func() {
		parent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
		objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	}
	h := newRetriggerHandler(nil, objects, mappings)
	h.publisher = publisher

	require.False(t, h.handlePastMeetingInviteeDelete(context.Background(), "itx-zoom-past-meetings-invitees.invitee-1", minimalInviteeV1Data("invitee-1", testSummaryMeetOcc, "alice")))
	assert.Equal(t, []string{"attendee-1"}, publisher.participants)
	assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingParticipantSubject+":attendee-1")
	assert.Positive(t, publisher.accessDeletes)
}

func TestPastMeetingSummary_ParentAccessChangedDuringRegistration(t *testing.T) {
	objects, mappings := newMemKV(), newMemKV()
	parent := map[string]any{
		"meeting_and_occurrence_id": testSummaryMeetOcc,
		"proj_id":                   "proj-sfid",
		"ai_summary_access":         "public",
	}
	objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
	mappings.onCreate = func(key string) {
		if key == testSummaryIndexKey {
			parent["ai_summary_access"] = "meeting_hosts"
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, parent)
		}
	}
	h := newRetriggerHandler(nil, objects, mappings)
	publisher := &pastMeetingCleanupPublisher{}
	h.publisher = publisher
	summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}

	require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
	assert.Equal(t, []string{"meeting_hosts"}, publisher.summaryAccess, "the post-registration parent read must win")
}

func TestPastMeetingArtifacts_DeleteRacingPublishRetractsChild(t *testing.T) {
	for _, tc := range []struct{ kind, deletion string }{
		{"recording", "soft"}, {"recording", "hard"}, {"summary", "soft"}, {"summary", "hard"},
	} {
		t.Run(tc.kind+"/"+tc.deletion, func(t *testing.T) {
			objects, mappings := newMemKV(), newMemKV()
			liveParent := map[string]any{
				"meeting_and_occurrence_id": testSummaryMeetOcc,
				"proj_id":                   "proj-sfid",
				"recording_access":          "public",
				"ai_summary_access":         "public",
			}
			objects.data[testPastMeetingKey] = mustMarshalJSON(t, liveParent)
			mappings.data[testSummaryIndexKey] = []byte(`["` + testSummaryID + `"]`)
			publisher := &pastMeetingCleanupPublisher{}
			h := newRetriggerHandler(nil, objects, mappings)
			h.publisher = publisher
			deleteParent := func() {
				if tc.deletion == "hard" {
					delete(objects.data, testPastMeetingKey) // no tombstone yet
				} else {
					liveParent["_sdc_deleted_at"] = "2026-01-02T00:00:00Z"
					objects.data[testPastMeetingKey] = mustMarshalJSON(t, liveParent)
				}
			}

			if tc.kind == "recording" {
				publisher.onRecording = deleteParent
				recording := map[string]any{"meeting_and_occurrence_id": testSummaryMeetOcc, "proj_id": "proj-sfid"}
				require.False(t, h.handlePastMeetingRecordingUpdate(context.Background(), "itx-zoom-past-meetings-recordings."+testSummaryMeetOcc, recording))
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingRecordingSubject+":"+testSummaryMeetOcc)
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingTranscriptSubject+":"+testSummaryMeetOcc)
			} else {
				publisher.onSummary = deleteParent
				summary := map[string]any{"id": testSummaryID, "meeting_and_occurrence_id": testSummaryMeetOcc, "approved": true}
				require.False(t, h.handlePastMeetingSummaryUpdate(context.Background(), testSummaryKVKey, summary))
				assert.Contains(t, publisher.indexerDeletes, eventing.IndexV1PastMeetingSummarySubject+":"+testSummaryID)
			}
		})
	}
}
