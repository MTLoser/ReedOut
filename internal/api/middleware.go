package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/reedfamily/reedout/internal/auth"
)

func AuthMiddleware(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if strings.HasPrefix(token, "Bearer ") {
				token = token[7:]
			} else {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			user, err := authSvc.ValidateSession(token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
