// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// AuthorizationMiddleware creates a middleware that adds a request ID to the context
func AuthorizationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := r.Header.Get(constants.AuthorizationHeader)
			ctx := context.WithValue(r.Context(), constants.AuthorizationContextID, authorization)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
