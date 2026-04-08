package main

import (
	"context"
	"net/http"
	"strings"
)

type authCtxKey int

const (
	ctxKeyAPIKeyID authCtxKey = iota + 1
	ctxKeyAPIKeyLabel
)

func apiKeyIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyAPIKeyID).(string)
	return v
}

func apiKeyLabelFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyAPIKeyLabel).(string)
	return v
}

func withAPIKeyContext(r *http.Request, id, label string) *http.Request {
	return r.WithContext(context.WithValue(
		context.WithValue(r.Context(), ctxKeyAPIKeyID, id),
		ctxKeyAPIKeyLabel, label))
}

// apiKeyMiddleware attaches API key identity and enforces auth rules:
// - /dashboard/api/* always requires a valid configured key (Bearer).
// - /v1/* requires a key only when authRequiredForAPI(); optional keys still attach context when valid.
func apiKeyMiddleware(cfg *FileConfig, next http.Handler) http.Handler {
	if cfg == nil {
		cfg = &FileConfig{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		bearer := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		id, label, ok := cfg.lookupSecret(bearer)

		if strings.HasPrefix(path, "/dashboard/api/") {
			if bearer == "" || !ok {
				writeOpenAIAPIError(w, http.StatusUnauthorized, "Valid API key required for the dashboard API.", "invalid_request_error")
				return
			}
			next.ServeHTTP(w, withAPIKeyContext(r, id, label))
			return
		}

		if strings.HasPrefix(path, "/v1/") {
			need := cfg.authRequiredForAPI()
			if need {
				if bearer == "" {
					writeOpenAIAPIError(w, http.StatusUnauthorized, "Missing API key. Pass Authorization: Bearer <key>.", "invalid_request_error")
					return
				}
				if !ok {
					writeOpenAIAPIError(w, http.StatusUnauthorized, "Incorrect API key provided.", "invalid_request_error")
					return
				}
			}
			if ok {
				r = withAPIKeyContext(r, id, label)
			}
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
