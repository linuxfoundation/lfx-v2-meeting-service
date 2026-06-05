// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

// InviteFeatureConfig holds LFID invite feature settings shared by the event
// processor (outbound invites) and the invite_accepted subscriber.
type InviteFeatureConfig struct {
	// Enabled controls whether invite sending and acceptance handling are active.
	Enabled bool
	// SelfServeBaseURL is the LFX self-serve app URL embedded in invite emails as return_url.
	SelfServeBaseURL string
}
