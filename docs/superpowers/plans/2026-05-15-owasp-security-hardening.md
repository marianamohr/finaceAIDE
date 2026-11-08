# OWASP Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate the highest-impact vulnerabilities identified in the OWASP Top 10:2025 audit of the Go backend and React frontend.

**Architecture:** Fixes are grouped by priority tier. Tier 1 (critical) addresses token storage in localStorage and missing audit logging. Tier 2 (medium) covers SameSite cookie hardening, rate limiting on register, password max-length, and invite token URL exposure. Tier 3 (low) handles deprecated headers, TypeScript strict mode, and CSP tightening.

**Tech Stack:** Go 1.23, chi v5, GORM, zerolog, React 18, TypeScript, Zustand, Vite

---

## File Map

| File | Change |
|---|---|
| `backend/internal/auth/jwt.go` | Add `jti` claim to `Claims` struct and `sign()` |
| `backend/internal/handlers/auth.go` | Fix `SameSite=Strict`, add password max-length check, add `SameSite=Strict` to Logout cookie |
| `backend/internal/handlers/admin.go` | Add structured audit log lines on every state-changing action |
| `backend/internal/middleware/ratelimit.go` | Add `RegisterRateLimit` function |
| `backend/internal/middleware/security_headers.go` | Remove deprecated `X-XSS-Protection`, upgrade CSP |
| `backend/cmd/server/main.go` | Wire `RegisterRateLimit` onto `POST /auth/register` |
| `frontend/src/store/auth.ts` | Remove `token` from `partialize` (stop persisting JWT to localStorage) |
| `frontend/src/services/api.ts` | Remove `console.log` in `uploadImage`, remove localStorage access token fallback |
| `frontend/src/pages/Invite.tsx` | Token already sent in POST body — add note, no change needed (already correct) |
| `frontend/tsconfig.app.json` | Enable `strict: true`, `noImplicitAny: true` |

---

## Tier 1 — Critical

### Task 1: Remove access token from localStorage

The JWT access token is currently persisted via Zustand's `persist` middleware under key `fai-a`. Any XSS script can read `localStorage.getItem("fai-a")` and extract the token. The fix is to stop persisting `token` entirely — it lives in Zustand memory and is re-hydrated from a fresh `/auth/refresh` call on page load.

**Files:**
- Modify: `frontend/src/store/auth.ts`
- Modify: `frontend/src/services/api.ts` (ensure refresh-on-load still works)

- [ ] **Step 1: Update partialize in auth store**

In `frontend/src/store/auth.ts`, change the `partialize` config so `token` is NOT persisted. Only persist `isAuthenticated` and `user` (role/status for UI routing):

```typescript
// frontend/src/store/auth.ts
import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface User {
  id?: string;
  email?: string;
  role: string;
  status: string;
  account_id?: string;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  setAuth: (user: User, token: string) => void;
  setToken: (token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,
      setAuth: (user, token) => set({ user, token, isAuthenticated: true }),
      setToken: (token) => set({ token, isAuthenticated: true }),
      logout: () => set({ user: null, token: null, isAuthenticated: false }),
    }),
    {
      name: "fai-a",
      partialize: (state) => ({
        // token intentionally excluded — re-acquired via /auth/refresh on load
        isAuthenticated: state.isAuthenticated,
        user: state.user ? { role: state.user.role, status: state.user.status } : null,
      }),
    }
  )
);
```

- [ ] **Step 2: Ensure app re-acquires token on load**

In `frontend/src/services/api.ts`, verify the 401 → refresh flow handles the case where `isAuthenticated` is true but `token` is null (first load after this change). The existing `request()` logic already handles this: on the first API call, the server returns 401 (no access token in header), then the refresh flow runs using the httpOnly cookie. No code change needed — **verify by reading lines 36–70 of `api.ts`**.

The current guard is `if (res.status === 401 && useAuthStore.getState().isAuthenticated)`. After this change, on first page load: `isAuthenticated = true` (persisted), `token = null` → first request gets 401 → refresh fires via cookie → new token acquired. This is correct behaviour.

- [ ] **Step 3: Remove stale token from existing localStorage entries**

Add a one-time migration in `frontend/src/main.tsx` that clears the old `token` field from any existing `fai-a` localStorage entry. Place this before `ReactDOM.createRoot`:

```typescript
// frontend/src/main.tsx — add near top, before createRoot
try {
  const raw = localStorage.getItem("fai-a");
  if (raw) {
    const parsed = JSON.parse(raw);
    if (parsed?.state?.token) {
      delete parsed.state.token;
      localStorage.setItem("fai-a", JSON.stringify(parsed));
    }
  }
} catch {}
```

- [ ] **Step 4: Build and check no TypeScript errors**

```bash
cd frontend && npm run build 2>&1 | tail -20
```

Expected: `✓ built in` with no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/store/auth.ts frontend/src/main.tsx
git commit -m "security: stop persisting JWT access token in localStorage"
```

---

### Task 2: Add audit logging to admin actions

Every admin state-change (status update, account change, settings change, invite creation) currently has zero audit trail. A compromised admin account could modify users with no forensic record. Fix: add structured zerolog lines with actor identity (admin user ID from JWT context), target user ID, action, and old→new values.

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/internal/auth/middleware.go` (verify `UserIDFromContext` is exported — it is)

- [ ] **Step 1: Add audit log to UpdateStatus**

In `backend/internal/handlers/admin.go`, after `h.users.UpdateStatus(id, req.Status)` succeeds, add:

```go
// in UpdateStatus, replace the success block:
if err := h.users.UpdateStatus(id, req.Status); err != nil {
    if err == repository.ErrNotFound {
        respondError(w, http.StatusNotFound, "user not found")
        return
    }
    log.Error().Err(err).Msg("admin: update status")
    respondError(w, http.StatusInternalServerError, "internal error")
    return
}

actorID := auth.UserIDFromContext(r.Context())
log.Info().
    Str("audit", "admin_action").
    Str("action", "update_user_status").
    Str("actor_id", actorID.String()).
    Str("target_user_id", id.String()).
    Str("new_status", req.Status).
    Msg("admin: user status updated")

respondJSON(w, http.StatusOK, map[string]string{"status": req.Status})
```

- [ ] **Step 2: Add audit log to UpdateAccount**

After `h.users.UpdateAccount(id, accountID)` succeeds:

```go
actorID := auth.UserIDFromContext(r.Context())
log.Info().
    Str("audit", "admin_action").
    Str("action", "update_user_account").
    Str("actor_id", actorID.String()).
    Str("target_user_id", id.String()).
    Str("new_account_id", accountID.String()).
    Msg("admin: user account updated")
```

- [ ] **Step 3: Add audit log to UpdateUserSettings**

After `h.users.UpdateShowTools(id, *req.ShowTools)` succeeds:

```go
actorID := auth.UserIDFromContext(r.Context())
log.Info().
    Str("audit", "admin_action").
    Str("action", "update_user_show_tools").
    Str("actor_id", actorID.String()).
    Str("target_user_id", id.String()).
    Bool("show_tools", *req.ShowTools).
    Msg("admin: user show_tools updated")
```

- [ ] **Step 4: Add audit log to CreateInvite**

After `h.invites.Create(invite)` succeeds:

```go
actorID := auth.UserIDFromContext(r.Context())
log.Info().
    Str("audit", "admin_action").
    Str("action", "create_invite").
    Str("actor_id", actorID.String()).
    Str("invite_email", req.Email).
    Msg("admin: invite created")
```

- [ ] **Step 5: Add import for auth package if missing**

Check the imports in `admin.go`. Add `"github.com/marianamohr/financas/internal/auth"` if not already present:

```go
import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/marianamohr/financas/internal/auth"
    "github.com/marianamohr/financas/internal/models"
    "github.com/marianamohr/financas/internal/repository"
    "github.com/rs/zerolog/log"
)
```

- [ ] **Step 6: Build backend**

```bash
cd backend && go build ./... 2>&1
```

Expected: no output (clean build).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/handlers/admin.go
git commit -m "security: add structured audit logging to all admin actions"
```

---

## Tier 2 — Medium

### Task 3: Harden refresh token cookie (SameSite=Strict)

The refresh token cookie uses `SameSite=Lax`, which allows cross-site top-level POST submissions (a CSRF vector). Change to `SameSite=Strict`. The comment at line 298 already says "SameSite=Strict" — the code contradicts itself.

**Files:**
- Modify: `backend/internal/handlers/auth.go` — lines 307 and 213

- [ ] **Step 1: Fix setRefreshCookie**

In `backend/internal/handlers/auth.go`, change `setRefreshCookie` (line ~299):

```go
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string) {
    http.SetCookie(w, &http.Cookie{
        Name:     refreshTokenCookie,
        Value:    token,
        Path:     "/",
        Expires:  time.Now().Add(30 * 24 * time.Hour),
        HttpOnly: true,
        Secure:   h.secure,
        SameSite: http.SameSiteStrictMode,
    })
}
```

- [ ] **Step 2: Fix Logout cookie**

In `Logout` handler (line ~206), also change to `SameSiteStrictMode`:

```go
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:     refreshTokenCookie,
        Value:    "",
        Path:     "/",
        MaxAge:   -1,
        HttpOnly: true,
        Secure:   h.secure,
        SameSite: http.SameSiteStrictMode,
    })
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Build backend**

```bash
cd backend && go build ./... 2>&1
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/auth.go
git commit -m "security: upgrade refresh token cookie to SameSite=Strict"
```

---

### Task 4: Add rate limiting to /auth/register

Currently only `/auth/login` has rate limiting. An attacker can create unlimited accounts (spam, resource exhaustion). Add a per-IP rate limit of 5 registrations per hour.

**Files:**
- Modify: `backend/internal/middleware/ratelimit.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add RegisterRateLimit to ratelimit.go**

In `backend/internal/middleware/ratelimit.go`, add after the `LoginRateLimit` function:

```go
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
```

- [ ] **Step 2: Wire rate limit onto POST /auth/register in main.go**

Find the auth routes block in `backend/cmd/server/main.go`. Add the rate limit middleware to the register route. Locate the existing login rate limit pattern and mirror it for register:

```go
// In the r.Route("/auth", ...) block, find:
//   r.With(middleware.LoginRateLimit(5, 900)).Post("/login", authHandler.Login)
// Add below it:
r.With(middleware.RegisterRateLimit(5, 3600)).Post("/register", authHandler.Register)
```

(Remove any existing unwrapped `r.Post("/register", authHandler.Register)` line.)

- [ ] **Step 3: Build backend**

```bash
cd backend && go build ./... 2>&1
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/middleware/ratelimit.go backend/cmd/server/main.go
git commit -m "security: add rate limiting to /auth/register (5 req/hour per IP)"
```

---

### Task 5: Add password maximum length validation (bcrypt 72-byte truncation)

bcrypt silently truncates passwords at 72 bytes. Two passwords identical in the first 72 characters are treated as the same. This is a subtle auth bypass for long passwords. Fix: reject passwords longer than 72 characters at the API level.

**Files:**
- Modify: `backend/internal/handlers/auth.go` — `Register`, `AcceptInvite`, and `ChangePassword` handlers

- [ ] **Step 1: Add max-length check in Register**

In `backend/internal/handlers/auth.go`, after line 50 (`if len(req.Password) < 8`), add:

```go
if len(req.Password) < 8 {
    respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
    return
}
if len(req.Password) > 72 {
    respondError(w, http.StatusBadRequest, "password must be at most 72 characters")
    return
}
```

- [ ] **Step 2: Add max-length check in AcceptInvite**

Find the `AcceptInvite` handler (line ~235) and mirror the same check:

```go
if len(req.Password) < 8 {
    respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
    return
}
if len(req.Password) > 72 {
    respondError(w, http.StatusBadRequest, "password must be at most 72 characters")
    return
}
```

- [ ] **Step 3: Find and update ChangePassword handler**

Read `backend/internal/handlers/user.go` to locate `ChangePassword`. Add the same `> 72` check there for `new_password`.

- [ ] **Step 4: Build backend**

```bash
cd backend && go build ./... 2>&1
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/auth.go backend/internal/handlers/user.go
git commit -m "security: reject passwords over 72 chars to prevent bcrypt truncation"
```

---

### Task 6: Remove deprecated X-XSS-Protection header

`X-XSS-Protection: 1; mode=block` is deprecated and ignored by modern browsers. In some older browsers it can actually introduce XSS vulnerabilities. Remove it and upgrade the CSP `style-src` directive.

**Files:**
- Modify: `backend/internal/middleware/security_headers.go`

- [ ] **Step 1: Remove X-XSS-Protection and upgrade CSP**

Replace the entire `SecurityHeaders` function body:

```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        h := w.Header()

        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("X-Frame-Options", "DENY")
        h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
        h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        h.Set("Content-Security-Policy",
            "default-src 'self'; "+
                "connect-src 'self'; "+
                "img-src 'self' data: blob:; "+
                "style-src 'self' 'unsafe-inline'; "+
                "script-src 'self'; "+
                "frame-ancestors 'none'; "+
                "upgrade-insecure-requests;",
        )
        h.Set("Permissions-Policy", "interest-cohort=()")

        next.ServeHTTP(w, r)
    })
}
```

Changes from before:
- Removed `X-XSS-Protection`
- Added `preload` to HSTS
- Added `upgrade-insecure-requests` to CSP

- [ ] **Step 2: Build backend**

```bash
cd backend && go build ./... 2>&1
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/middleware/security_headers.go
git commit -m "security: remove deprecated X-XSS-Protection, add HSTS preload and CSP upgrade-insecure-requests"
```

---

## Tier 3 — Low / Maintenance

### Task 7: Enable TypeScript strict mode

`strict: false` disables null checks and implicit-any, hiding potential runtime errors. Enabling it makes the codebase safer and catches XSS-prone patterns at compile time.

**Files:**
- Modify: `frontend/tsconfig.app.json`

- [ ] **Step 1: Enable strict in tsconfig.app.json**

Read the current file and change:

```json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true
  }
}
```

(Preserve all existing keys — only change these four.)

- [ ] **Step 2: Run TypeScript check**

```bash
cd frontend && npm run build 2>&1 | head -60
```

Expected: Likely some type errors will surface. Document them. If there are more than 10 errors, this task should be done incrementally — revert `strict: true` and do one file at a time in separate commits.

- [ ] **Step 3: Fix any type errors**

Common fixes for React + Zustand codebases:
- `e: any` in catch blocks → `e: unknown` + `e instanceof Error ? e.message : String(e)`
- Missing return types on functions
- `user?.email` instead of `user.email` where user can be null

Fix each error in the file it appears. Do not add `// @ts-ignore` comments.

- [ ] **Step 4: Build clean**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Expected: `✓ built in` with no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/tsconfig.app.json frontend/src/
git commit -m "security: enable TypeScript strict mode and fix resulting type errors"
```

---

### Task 8: Remove console.log from uploadImage (information disclosure)

`api.ts` has `console.log` calls in `uploadImage` that expose file metadata and response status to the browser console — visible to any user with DevTools open.

**Files:**
- Modify: `frontend/src/services/api.ts` — lines 268–270 and 281–282

- [ ] **Step 1: Remove console.log calls**

In `frontend/src/services/api.ts`, remove these blocks entirely:

```typescript
// Remove this block (lines ~268-272):
console.log("[upload] starting", {
  name: file.name,
  type: file.type,
  sizeKB: Math.round(file.size / 1024),
});

// Remove this line (~281):
console.log("[upload] response", { status: res.status, ok: res.ok });

// Remove this line (~287):
console.error("[upload] error", { status: res.status, message });
```

- [ ] **Step 2: Build**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/services/api.ts
git commit -m "security: remove console.log from uploadImage (information disclosure)"
```

---

## Verification Checklist

After all tasks are complete:

- [ ] **localStorage inspection:** Open browser DevTools → Application → Local Storage → `fai-a`. Verify `state.token` key does not exist.
- [ ] **Cookie inspection:** After login, DevTools → Application → Cookies. Verify `refresh_token` has `HttpOnly=true`, `Secure=true`, `SameSite=Strict`.
- [ ] **Admin audit log:** Perform a status change as admin via the UI. Check server logs for a structured log line with `"audit":"admin_action"`, `"actor_id"`, `"target_user_id"`.
- [ ] **Register rate limit:** Run `for i in {1..7}; do curl -s -X POST http://localhost:8080/auth/register -H 'Content-Type: application/json' -d '{"email":"test'$i'@x.com","password":"password1"}' | jq .error; done`. Expect 429 after the 5th attempt.
- [ ] **Password max length:** `curl -s -X POST http://localhost:8080/auth/register -H 'Content-Type: application/json' -d '{"email":"a@b.com","password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' | jq .error`. Expect `"password must be at most 72 characters"`.
- [ ] **No X-XSS-Protection header:** `curl -I http://localhost:8080/health | grep -i xss`. Expect no output.
- [ ] **HSTS preload:** `curl -I http://localhost:8080/health | grep Strict-Transport`. Expect `max-age=31536000; includeSubDomains; preload`.
- [ ] **TypeScript build clean:** `cd frontend && npm run build`. No errors.
- [ ] **No console.log in upload:** Open browser DevTools console, upload an image. No `[upload]` log lines should appear.
