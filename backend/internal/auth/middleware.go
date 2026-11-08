package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys from other packages.
type contextKey string

const (
	contextKeyUserID    contextKey = "userID"
	contextKeyAccountID contextKey = "accountID"
	contextKeyRole      contextKey = "role"
	contextKeyStatus    contextKey = "status"
)

// Middleware returns an HTTP middleware that validates the Bearer token
// and injects userID, accountID, role, and status into the request context.
// Requests without a valid token receive 401 and are not forwarded.
// Pending or suspended accounts receive 403.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearer(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.ValidateAccessToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Block pending and suspended accounts.
		switch claims.Status {
		case "pending":
			http.Error(w, `{"error":"account_pending"}`, http.StatusForbidden)
			return
		case "suspended":
			http.Error(w, `{"error":"account_suspended"}`, http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, contextKeyAccountID, claims.AccountID)
		ctx = context.WithValue(ctx, contextKeyRole, claims.Role)
		ctx = context.WithValue(ctx, contextKeyStatus, claims.Status)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is a middleware that allows only users with role "admin".
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(contextKeyRole).(string)
		if role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserIDFromContext retrieves the authenticated user's UUID from the context.
// Panics if called outside an authenticated route — this is intentional:
// a missing userID in a protected handler is a programming error, not a runtime condition.
func UserIDFromContext(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	if !ok {
		panic("auth: userID not found in context — route must be protected by Middleware")
	}
	return id
}

// AccountIDFromContext retrieves the authenticated user's account UUID from the context.
func AccountIDFromContext(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(contextKeyAccountID).(uuid.UUID)
	if !ok {
		panic("auth: accountID not found in context — route must be protected by Middleware")
	}
	return id
}

// RoleFromContext retrieves the authenticated user's role from the context.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(contextKeyRole).(string)
	return role
}

// ContextWithValues injects userID, accountID, role, and status into ctx.
// Intended for tests that need an authenticated context without going through the JWT middleware.
func ContextWithValues(ctx context.Context, userID, accountID uuid.UUID, role, status string) context.Context {
	ctx = context.WithValue(ctx, contextKeyUserID, userID)
	ctx = context.WithValue(ctx, contextKeyAccountID, accountID)
	ctx = context.WithValue(ctx, contextKeyRole, role)
	ctx = context.WithValue(ctx, contextKeyStatus, status)
	return ctx
}

// extractBearer parses the Authorization header and returns the raw token string.
// Returns empty string if the header is absent or malformed.
func extractBearer(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
