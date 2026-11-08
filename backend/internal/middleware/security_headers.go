package middleware

import "net/http"

// SecurityHeaders adds OWASP-recommended HTTP security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent MIME-type sniffing.
		h.Set("X-Content-Type-Options", "nosniff")

		// Deny framing entirely — no clickjacking.
		h.Set("X-Frame-Options", "DENY")

		// Force HTTPS for 1 year (includeSubDomains + preload).
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Restrict referrer information.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy:
		// - default-src 'self'              → only same-origin by default
		// - connect-src 'self'              → API calls only to same origin
		// - img-src 'self' data: blob:      → allow data URIs and blobs for image previews
		// - style-src 'self' 'unsafe-inline'→ React may inject inline styles
		// - script-src 'self'               → no inline scripts, no eval
		// - frame-ancestors 'none'          → equivalent to X-Frame-Options DENY for CSP-aware browsers
		// - upgrade-insecure-requests       → auto-upgrade HTTP sub-resources to HTTPS
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"connect-src 'self'; "+
				"img-src 'self' data: blob:; "+
				"style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; "+
				"frame-ancestors 'none'; "+
				"upgrade-insecure-requests;",
		)

		// Disable FLoC / Privacy Sandbox topics.
		h.Set("Permissions-Policy", "interest-cohort=()")

		next.ServeHTTP(w, r)
	})
}
