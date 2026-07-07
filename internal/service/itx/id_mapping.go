// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	pkgitx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func mapProjectFieldV2ToV1(ctx context.Context, mapper domain.IDMapper, projectID *string) error {
	if projectID == nil || *projectID == "" {
		return nil
	}
	v1ID, err := mapper.MapProjectV2ToV1(ctx, *projectID)
	if err != nil {
		return err
	}
	*projectID = v1ID
	return nil
}

func mapProjectFieldV1ToV2(ctx context.Context, mapper domain.IDMapper, projectID *string) error {
	if projectID == nil || *projectID == "" {
		return nil
	}
	v2UID, err := mapper.MapProjectV1ToV2(ctx, *projectID)
	if err != nil {
		return err
	}
	*projectID = v2UID
	return nil
}

func mapCommitteeFieldV2ToV1(ctx context.Context, mapper domain.IDMapper, committeeID *string) error {
	if committeeID == nil || *committeeID == "" {
		return nil
	}
	v1ID, err := mapper.MapCommitteeV2ToV1(ctx, *committeeID)
	if err != nil {
		return err
	}
	*committeeID = v1ID
	return nil
}

func mapCommitteeFieldV1ToV2Graceful(ctx context.Context, mapper domain.IDMapper, v1ID, logMessage string) string {
	if v1ID == "" {
		return ""
	}
	v2UID, err := mapper.MapCommitteeV1ToV2(ctx, v1ID)
	if err != nil {
		slog.WarnContext(ctx, logMessage, "v1_id", v1ID, "err", err)
		return ""
	}
	return v2UID
}

func mapMeetingCommitteesV2ToV1(ctx context.Context, mapper domain.IDMapper, committees []models.Committee) error {
	for i := range committees {
		if committees[i].UID == "" {
			continue
		}
		v1SFID, err := mapper.MapCommitteeV2ToV1(ctx, committees[i].UID)
		if err != nil {
			return err
		}
		committees[i].UID = v1SFID
	}
	return nil
}

func mapMeetingCommitteesV1ToV2Graceful(ctx context.Context, mapper domain.IDMapper, committees []pkgitx.Committee, logMessage string) {
	for i := range committees {
		if committees[i].ID == "" {
			continue
		}
		committees[i].ID = mapCommitteeFieldV1ToV2Graceful(ctx, mapper, committees[i].ID, logMessage)
	}
}

func mapITXCommitteesV2ToV1(ctx context.Context, mapper domain.IDMapper, committees []pkgitx.Committee) error {
	for i := range committees {
		if committees[i].ID == "" {
			continue
		}
		v1ID, err := mapper.MapCommitteeV2ToV1(ctx, committees[i].ID)
		if err != nil {
			return err
		}
		committees[i].ID = v1ID
	}
	return nil
}

func mapITXCommitteesV1ToV2Graceful(ctx context.Context, mapper domain.IDMapper, committees []pkgitx.Committee, logMessage string) {
	for i := range committees {
		if committees[i].ID == "" {
			continue
		}
		committees[i].ID = mapCommitteeFieldV1ToV2Graceful(ctx, mapper, committees[i].ID, logMessage)
	}
}
