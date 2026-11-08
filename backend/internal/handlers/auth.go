package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

const refreshTokenCookie = "refresh_token"

// AuthHandler handles registration, login and token refresh.
type AuthHandler struct {
	users    *repository.UserRepository
	accounts *repository.AccountRepository
	invites  *repository.InviteRepository
	jwt      *auth.Manager
	pmRepo   *repository.PaymentMethodRepository
	secure   bool // true in production (HTTPS only cookies)
}

func NewAuthHandler(
	users *repository.UserRepository,
	accounts *repository.AccountRepository,
	invites *repository.InviteRepository,
	jwt *auth.Manager,
	pmRepo *repository.PaymentMethodRepository,
	secure bool,
) *AuthHandler {
	return &AuthHandler{users: users, accounts: accounts, invites: invites, jwt: jwt, pmRepo: pmRepo, secure: secure}
}

// Register godoc
// POST /auth/register
// Body: { "email": "...", "password": "..." }
// Response 201: { "access_token": "...", "user": {...} }
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 72 {
		respondError(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("register: hash password")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Create a personal account for this user.
	role := "user"
	status := "pending"
	account := &models.Account{Name: req.Email}
	if err := h.accounts.Create(account); err != nil {
		log.Error().Err(err).Msg("register: create account")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: hash,
		Role:         role,
		Status:       status,
		AccountID:    account.ID,
	}
	if err := h.users.Create(user); err != nil {
		respondError(w, http.StatusConflict, "email already registered")
		return
	}

	// Criar forma de pagamento padrão (Débito) — idempotente
	var pmCount int64
	if countErr := h.pmRepo.DB().Model(&models.PaymentMethod{}).
		Where("account_id = ?", account.ID).Count(&pmCount).Error; countErr == nil && pmCount == 0 {
		defaultPM := &models.PaymentMethod{
			AccountID: account.ID,
			Name:      "Débito",
			Kind:      models.KindDebit,
			Color:     "#6366f1",
			Active:    true,
		}
		if createErr := h.pmRepo.Create(defaultPM); createErr != nil {
			log.Warn().Err(createErr).Msg("register: create default payment method")
			// Non-fatal: continue registration
		}
	}

	pair, err := h.jwt.IssueTokenPair(user.ID, user.AccountID, user.Role, user.Status)
	if err != nil {
		log.Error().Err(err).Msg("register: issue token pair")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	respondJSON(w, http.StatusCreated, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user": map[string]string{
			"id":         user.ID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"status":     user.Status,
			"account_id": user.AccountID.String(),
		},
	})
}

// Login godoc
// POST /auth/login
// Body: { "email": "...", "password": "..." }
// Response 200: { "access_token": "...", "user": {...} }
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, err := h.users.FindByEmail(req.Email)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		log.Error().Err(err).Msg("login: check password")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	pair, err := h.jwt.IssueTokenPair(user.ID, user.AccountID, user.Role, user.Status)
	if err != nil {
		log.Error().Err(err).Msg("login: issue token pair")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken, // also in body for PWA fallback
		"user": map[string]string{
			"id":         user.ID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"status":     user.Status,
			"account_id": user.AccountID.String(),
		},
	})
}

// Refresh godoc
// POST /auth/refresh
// Reads refresh token from httpOnly cookie or X-Refresh-Token header (PWA fallback).
// Issues a new pair (rotation).
// Response 200: { "access_token": "...", "refresh_token": "...", "user": {...} }
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	token := ""
	if cookie, err := r.Cookie(refreshTokenCookie); err == nil {
		token = cookie.Value
	} else if h := r.Header.Get("X-Refresh-Token"); h != "" {
		token = h
	}
	if token == "" {
		respondError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	claims, err := h.jwt.ValidateRefreshToken(token)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Re-fetch user to get latest status/role (may have changed since token was issued).
	user, err := h.users.FindByID(claims.UserID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	pair, err := h.jwt.IssueTokenPair(user.ID, user.AccountID, user.Role, user.Status)
	if err != nil {
		log.Error().Err(err).Msg("refresh: issue token pair")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user": map[string]string{
			"id":         user.ID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"status":     user.Status,
			"account_id": user.AccountID.String(),
		},
	})
}

// Logout godoc
// POST /auth/logout
// Clears the refresh token cookie.
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

// AcceptInvite godoc
// POST /auth/invite/accept
// Body: { "token": "...", "password": "..." }
// Creates the account, marks invite as used, returns tokens.
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "token and password are required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 72 {
		respondError(w, http.StatusBadRequest, "password must be at most 72 characters")
		return
	}

	invite, err := h.invites.FindByToken(req.Token)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid or expired invite link")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("accept invite: hash password")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	account := &models.Account{Name: invite.Email}
	if err := h.accounts.Create(account); err != nil {
		log.Error().Err(err).Msg("accept invite: create account")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &models.User{
		Email:        strings.ToLower(strings.TrimSpace(invite.Email)),
		PasswordHash: hash,
		Role:         "user",
		Status:       "active",
		AccountID:    account.ID,
	}
	if err := h.users.Create(user); err != nil {
		respondError(w, http.StatusConflict, "email already registered")
		return
	}

	if err := h.invites.MarkUsed(req.Token); err != nil {
		log.Error().Err(err).Msg("accept invite: mark used")
		// non-fatal
	}

	defaultDebit := &models.PaymentMethod{
		AccountID: account.ID,
		Name:      "Débito",
		Kind:      models.KindDebit,
		Color:     "",
		Active:    true,
	}
	if err := h.pmRepo.Create(defaultDebit); err != nil {
		log.Error().Err(err).Msg("accept invite: create default debit payment method")
		// non-fatal: user can add manually
	}

	pair, err := h.jwt.IssueTokenPair(user.ID, user.AccountID, user.Role, user.Status)
	if err != nil {
		log.Error().Err(err).Msg("accept invite: issue token pair")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.setRefreshCookie(w, pair.RefreshToken)
	respondJSON(w, http.StatusCreated, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user": map[string]string{
			"id":         user.ID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"status":     user.Status,
			"account_id": user.AccountID.String(),
		},
	})
}

// setRefreshCookie writes the refresh token as an httpOnly, Secure, SameSite=Strict cookie.
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

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
