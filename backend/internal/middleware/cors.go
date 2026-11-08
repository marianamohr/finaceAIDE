package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that allows requests from any of the given origins.
// Preflight OPTIONS requests are handled and short-circuited.
// All other origins receive 403.
func CORS(allowedOrigins ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimRight(o, "/")] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin == "" {
				// Non-browser requests (server-to-server, curl) — allow through.
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowed[origin]; !ok {
				http.Error(w, `{"error":"origin not allowed"}`, http.StatusForbidden)
				return
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true") // required for httpOnly cookie
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Refresh-Token")
			h.Set("Access-Control-Max-Age", "86400") // 24 h preflight cache

			// Short-circuit preflight.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
