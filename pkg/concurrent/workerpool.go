// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package concurrent

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// WorkerPool represents a pool of workers that can process jobs concurrently
type WorkerPool struct {
	workerCount int
}

// Run executes all functions using errgroup with goroutine limiting
// Returns the first error encountered, and cancels remaining work
func (wp *WorkerPool) Run(ctx context.Context, functions ...func() error) error {
	if len(functions) == 0 {
		return nil
	}

	// Create errgroup with context
	g, groupCtx := errgroup.WithContext(ctx)

	// Set the limit of concurrent goroutines
	g.SetLimit(wp.workerCount)

	// Submit all functions to the errgroup
	for _, fn := range functions {
		g.Go(func() error {
			// Check if context was cancelled before starting
			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			default:
			}

			return fn()
		})
	}

	// Wait for all functions to complete and return first error
	return g.Wait()
}

// RunAll executes all functions without cancellation on error
// Returns a slice containing only the non-nil errors that occurred
func (wp *WorkerPool) RunAll(ctx context.Context, functions ...func() error) []error {
	if len(functions) == 0 {
		return nil
	}

	// Use a channel to collect errors safely from concurrent goroutines
	type indexedError struct {
		index int
		err   error
	}
	errorChan := make(chan indexedError, len(functions))
	
	// Use errgroup without context cancellation
	g := new(errgroup.Group)
	g.SetLimit(wp.workerCount)

	// Submit all functions to the errgroup
	for i, fn := range functions {
		g.Go(func() error {
			// Check if the original context was cancelled
			select {
			case <-ctx.Done():
				errorChan <- indexedError{index: i, err: ctx.Err()}
				return nil // Return nil to errgroup so it continues
			default:
			}

			// Execute the function and send any error to the channel
			if err := fn(); err != nil {
				errorChan <- indexedError{index: i, err: err}
			}
			return nil // Always return nil to prevent errgroup from cancelling
		})
	}

	// Wait for all functions to complete
	g.Wait() // This will always return nil since we never return errors to errgroup
	close(errorChan)
	
	// Collect all errors from the channel
	var errors []error
	for ie := range errorChan {
		errors = append(errors, ie.err)
	}
	
	return errors
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workerCount int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &WorkerPool{
		workerCount: workerCount,
	}
}
