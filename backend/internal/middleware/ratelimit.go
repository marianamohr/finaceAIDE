package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// RegisterRateLimit returns a middleware that allows at most `requests` registration
// attempts per `windowSeconds` seconds, keyed by remote IP.
func RegisterRateLimit(requests int, windowSeconds int) func(http.Handler) http.Handler {
	window := time.Duration(windowSeconds) * time.Second
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many registration attempts, try again later"}`)) //nolint:errcheck
		}),
	)
}

// LoginRateLimit returns a middleware that allows at most `requests` requests
// per `windowSeconds` seconds, keyed by remote IP.
// Intended for the POST /auth/login route only.
func LoginRateLimit(requests int, windowSeconds int) func(http.Handler) http.Handler {
	window := time.Duration(windowSeconds) * time.Second
	return httprate.Limit(
		requests,
		window,
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "900")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many login attempts, try again in 15 minutes"}`)) //nolint:errcheck
		}),
	)
}
