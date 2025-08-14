// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package zoom

import (
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// Ensure Client implements PlatformProvider
var _ domain.PlatformProvider = (*Client)(nil)
