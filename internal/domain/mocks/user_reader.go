// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockUserReader implements domain.UserReader for testing.
type MockUserReader struct {
	mock.Mock
}

func (m *MockUserReader) SubByEmail(ctx context.Context, email string) (string, error) {
	args := m.Called(ctx, email)
	return args.String(0), args.Error(1)
}
