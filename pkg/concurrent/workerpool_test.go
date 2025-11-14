// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package concurrent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_Run(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	var counter int64
	functions := []func() error{
		func() error {
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond) // Simulate work
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 2)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 3)
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	err := pool.Run(ctx, functions...)
	require.NoError(t, err)
	assert.Equal(t, int64(6), atomic.LoadInt64(&counter))
}

func TestWorkerPool_Run_WithError(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	// Track which functions executed
	var executedFunc1, executedFunc2, executedFunc3 bool
	var mu sync.Mutex

	expectedError := errors.New("job failed")
	functions := []func() error{
		func() error {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executedFunc1 = true
			mu.Unlock()
			return nil
		},
		func() error {
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			executedFunc2 = true
			mu.Unlock()
			return expectedError
		},
		func() error {
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			executedFunc3 = true
			mu.Unlock()
			return nil
		},
	}

	err := pool.Run(ctx, functions...)

	// Verify error was returned
	require.Error(t, err)
	assert.Equal(t, expectedError, err)

	// Verify certain functions were executed while the remaining ones were not
	assert.True(t, executedFunc1, "Function 1 should have executed")
	assert.True(t, executedFunc2, "Function 2 should have executed")
	assert.False(t, executedFunc3, "Function 3 should not have executed")
}

func TestWorkerPool_Run_EmptyFunctions(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	err := pool.Run(ctx)
	require.NoError(t, err)
}

func TestWorkerPool_RunAll_ExecutesAllFunctions(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	// Track which functions executed
	var executedFunc1, executedFunc2, executedFunc3 bool
	var mu sync.Mutex

	errorFunc1 := errors.New("func1 failed")
	errorFunc3 := errors.New("func3 failed")

	functions := []func() error{
		func() error {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executedFunc1 = true
			mu.Unlock()
			return errorFunc1
		},
		func() error {
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			executedFunc2 = true
			mu.Unlock()
			return nil // This one succeeds
		},
		func() error {
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			executedFunc3 = true
			mu.Unlock()
			return errorFunc3
		},
	}

	errors := pool.RunAll(ctx, functions...)

	// Verify all functions executed
	assert.True(t, executedFunc1, "Function 1 should have executed")
	assert.True(t, executedFunc2, "Function 2 should have executed")
	assert.True(t, executedFunc3, "Function 3 should have executed")

	// Verify only actual errors are returned (no nils)
	require.Len(t, errors, 2, "Should have 2 errors (func1 and func3)")
	assert.Contains(t, errors, errorFunc1, "Should contain error from function 1")
	assert.Contains(t, errors, errorFunc3, "Should contain error from function 3")
}

func TestWorkerPool_RunAll_EmptyFunctions(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(2)

	errors := pool.RunAll(ctx)
	assert.Nil(t, errors)
}

func TestWorkerPool_RunAll_AllSucceed(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(3)

	var counter int64
	functions := []func() error{
		func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
		func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
	}

	errors := pool.RunAll(ctx, functions...)

	// Verify all functions executed
	assert.Equal(t, int64(3), atomic.LoadInt64(&counter))

	// Verify no errors were returned
	assert.Empty(t, errors, "Should have no errors when all functions succeed")
}

func TestWorkerPool_Run_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool(2)

	// Cancel context immediately
	cancel()

	functions := []func() error{
		func() error {
			return nil
		},
	}

	err := pool.Run(ctx, functions...)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestWorkerPool_RunAll_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool(2)

	// Cancel context immediately
	cancel()

	functions := []func() error{
		func() error {
			return nil
		},
	}

	errors := pool.RunAll(ctx, functions...)

	// RunAll should still attempt to execute, but get context.Canceled error
	require.Len(t, errors, 1)
	assert.Equal(t, context.Canceled, errors[0])
}

func TestNewWorkerPool_InvalidWorkerCount(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		expected    int
	}{
		{
			name:        "zero workers defaults to 1",
			workerCount: 0,
			expected:    1,
		},
		{
			name:        "negative workers defaults to 1",
			workerCount: -1,
			expected:    1,
		},
		{
			name:        "negative large number defaults to 1",
			workerCount: -100,
			expected:    1,
		},
		{
			name:        "positive workers returns same count",
			workerCount: 5,
			expected:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewWorkerPool(tt.workerCount)
			require.NotNil(t, pool)
			assert.Equal(t, tt.expected, pool.workerCount)
		})
	}
}
