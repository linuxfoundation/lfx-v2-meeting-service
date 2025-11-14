// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package platform

import (
	"fmt"
	"sync"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	require.NotNil(t, registry, "NewRegistry should return non-nil registry")

	// Registry implements PlatformRegistry interface (verified by function signature)
	// We can verify basic functionality
	provider, err := registry.GetProvider("NonExistent")
	assert.Nil(t, provider)
	assert.Error(t, err)
}

func TestRegistry_RegisterProvider(t *testing.T) {
	registry := NewRegistry()
	mockProvider := &mocks.MockPlatformProvider{}

	// Register a provider
	registry.RegisterProvider("Zoom", mockProvider)

	// Verify we can retrieve it
	provider, err := registry.GetProvider("Zoom")
	require.NoError(t, err)
	assert.Equal(t, mockProvider, provider)
}

func TestRegistry_RegisterProvider_Overwrite(t *testing.T) {
	registry := NewRegistry()
	mockProvider1 := &mocks.MockPlatformProvider{}
	mockProvider2 := &mocks.MockPlatformProvider{}

	// Register first provider
	registry.RegisterProvider("Zoom", mockProvider1)

	// Overwrite with second provider
	registry.RegisterProvider("Zoom", mockProvider2)

	// Verify second provider is returned
	provider, err := registry.GetProvider("Zoom")
	require.NoError(t, err)
	assert.Equal(t, mockProvider2, provider, "Should return the most recently registered provider")
}

func TestRegistry_RegisterProvider_MultiplePlatforms(t *testing.T) {
	registry := NewRegistry()
	zoomProvider := &mocks.MockPlatformProvider{}
	teamsProvider := &mocks.MockPlatformProvider{}
	webexProvider := &mocks.MockPlatformProvider{}

	// Register multiple providers
	registry.RegisterProvider("Zoom", zoomProvider)
	registry.RegisterProvider("Teams", teamsProvider)
	registry.RegisterProvider("Webex", webexProvider)

	// Verify each can be retrieved independently
	provider, err := registry.GetProvider("Zoom")
	require.NoError(t, err)
	assert.Equal(t, zoomProvider, provider)

	provider, err = registry.GetProvider("Teams")
	require.NoError(t, err)
	assert.Equal(t, teamsProvider, provider)

	provider, err = registry.GetProvider("Webex")
	require.NoError(t, err)
	assert.Equal(t, webexProvider, provider)
}

func TestRegistry_GetProvider_NotFound(t *testing.T) {
	registry := NewRegistry()

	// Try to get a provider that doesn't exist
	provider, err := registry.GetProvider("NonExistentPlatform")

	assert.Nil(t, provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform provider not found")
	assert.Contains(t, err.Error(), "NonExistentPlatform")
}

func TestRegistry_GetProvider_EmptyString(t *testing.T) {
	registry := NewRegistry()

	// Try to get a provider with empty string
	provider, err := registry.GetProvider("")

	assert.Nil(t, provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform provider not found")
}

func TestRegistry_GetProvider_CaseSensitive(t *testing.T) {
	registry := NewRegistry()
	mockProvider := &mocks.MockPlatformProvider{}

	// Register with specific casing
	registry.RegisterProvider("Zoom", mockProvider)

	// Verify case-sensitive lookup
	_, err := registry.GetProvider("zoom")
	require.Error(t, err, "Platform names should be case-sensitive")

	_, err = registry.GetProvider("ZOOM")
	require.Error(t, err, "Platform names should be case-sensitive")

	// Verify exact match works
	provider, err := registry.GetProvider("Zoom")
	require.NoError(t, err)
	assert.Equal(t, mockProvider, provider)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	mockProvider := &mocks.MockPlatformProvider{}

	// Register initial provider
	registry.RegisterProvider("Zoom", mockProvider)

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	iterations := 100

	// Channels to collect results from goroutines
	type readResult struct {
		provider domain.PlatformProvider
		err      error
	}
	readResults := make(chan readResult, iterations)
	missingResults := make(chan error, iterations)

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			provider, err := registry.GetProvider("Zoom")
			readResults <- readResult{provider: provider, err: err}
		}()
	}

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newProvider := &mocks.MockPlatformProvider{}
			// Register to different platforms to avoid contention
			registry.RegisterProvider(fmt.Sprintf("Platform%d", idx), newProvider)
		}(i)
	}

	// Concurrent reads of non-existent provider
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.GetProvider("NonExistent")
			missingResults <- err
		}()
	}

	wg.Wait()
	close(readResults)
	close(missingResults)

	// Assert on results in main goroutine
	for result := range readResults {
		assert.NoError(t, result.err)
		assert.NotNil(t, result.provider)
	}

	for err := range missingResults {
		assert.Error(t, err)
	}
}

func TestRegistry_ConcurrentOverwrite(t *testing.T) {
	registry := NewRegistry()

	var wg sync.WaitGroup
	iterations := 50

	// Concurrent overwrites of the same provider
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newProvider := &mocks.MockPlatformProvider{}
			registry.RegisterProvider("Zoom", newProvider)
		}()
	}

	wg.Wait()

	// Verify registry is still in valid state
	provider, err := registry.GetProvider("Zoom")
	require.NoError(t, err)
	assert.NotNil(t, provider, "Registry should be in valid state after concurrent overwrites")
}

func TestRegistry_InterfaceCompliance(t *testing.T) {
	// Verify Registry implements PlatformRegistry interface at compile time
	var registry interface{} = &Registry{}
	_, ok := registry.(domain.PlatformRegistry)
	assert.True(t, ok, "Registry must implement PlatformRegistry interface")
}

func TestRegistry_NilProvider(t *testing.T) {
	registry := NewRegistry()

	// Register nil provider (edge case, but should be allowed)
	registry.RegisterProvider("NilPlatform", nil)

	// Should be able to retrieve the nil provider
	provider, err := registry.GetProvider("NilPlatform")
	require.NoError(t, err)
	assert.Nil(t, provider, "Should be able to register and retrieve nil provider")
}

func TestRegistry_ProviderIsolation(t *testing.T) {
	registry := NewRegistry()
	mockProvider1 := &mocks.MockPlatformProvider{}
	mockProvider2 := &mocks.MockPlatformProvider{}

	// Register to Platform1
	registry.RegisterProvider("Platform1", mockProvider1)

	// Register to Platform2
	registry.RegisterProvider("Platform2", mockProvider2)

	// Verify providers are isolated
	provider1, err := registry.GetProvider("Platform1")
	require.NoError(t, err)

	provider2, err := registry.GetProvider("Platform2")
	require.NoError(t, err)

	// Verify they're the correct instances
	assert.Same(t, mockProvider1, provider1, "Platform1 should return mockProvider1")
	assert.Same(t, mockProvider2, provider2, "Platform2 should return mockProvider2")

	// Verify they're different instances (different memory addresses)
	assert.NotSame(t, provider1, provider2, "Different platforms should have different provider instances")
}

func TestRegistry_EmptyRegistryState(t *testing.T) {
	registry := NewRegistry()

	// Try to get from empty registry
	_, err := registry.GetProvider("AnyPlatform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform provider not found")
}

func TestRegistry_SpecialCharactersInPlatformName(t *testing.T) {
	registry := NewRegistry()
	mockProvider := &mocks.MockPlatformProvider{}

	tests := []struct {
		name         string
		platformName string
	}{
		{
			name:         "platform with spaces",
			platformName: "Platform Name",
		},
		{
			name:         "platform with special characters",
			platformName: "Platform-Name_v2.0",
		},
		{
			name:         "platform with unicode",
			platformName: "Platform™",
		},
		{
			name:         "platform with numbers",
			platformName: "Platform123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register provider with special characters
			registry.RegisterProvider(tt.platformName, mockProvider)

			// Verify retrieval
			provider, err := registry.GetProvider(tt.platformName)
			require.NoError(t, err)
			assert.Equal(t, mockProvider, provider)
		})
	}
}
