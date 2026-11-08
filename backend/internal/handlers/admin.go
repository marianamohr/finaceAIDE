package handlers

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

// AdminHandler handles admin-only user management endpoints.
type AdminHandler struct {
	users       *repository.UserRepository
	invites     *repository.InviteRepository
	frontendURL string
}

func NewAdminHandler(users *repository.UserRepository, invites *repository.InviteRepository, frontendURL string) *AdminHandler {
	return &AdminHandler{users: users, invites: invites, frontendURL: frontendURL}
}

// ListUsers godoc
// GET /admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListAllWithSettings()
	if err != nil {
		log.Error().Err(err).Msg("admin: list users")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

// UpdateUserSettings godoc
// PATCH /admin/users/{id}/settings
// Body: { "show_tools": true|false }
func (h *AdminHandler) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		ShowTools *bool `json:"show_tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ShowTools == nil {
		respondError(w, http.StatusBadRequest, "show_tools is required")
		return
	}

	if err := h.users.UpdateShowTools(id, *req.ShowTools); err != nil {
		log.Error().Err(err).Msg("admin: update user settings")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	actorID := auth.UserIDFromContext(r.Context())
	log.Info().
		Str("audit", "admin_action").
		Str("action", "update_user_show_tools").
		Str("actor_id", actorID.String()).
		Str("target_user_id", id.String()).
		Bool("show_tools", *req.ShowTools).
		Msg("admin: user show_tools updated")

	respondJSON(w, http.StatusOK, map[string]bool{"show_tools": *req.ShowTools})
}

// UpdateStatus godoc
// PATCH /admin/users/{id}/status
// Body: { "status": "active"|"suspended"|"pending" }
func (h *AdminHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Status {
	case "active", "suspended", "pending":
	default:
		respondError(w, http.StatusBadRequest, "status must be active, suspended, or pending")
		return
	}

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
}

// UpdateAccount godoc
// PATCH /admin/users/{id}/account
// Body: { "account_id": "<uuid>" }
func (h *AdminHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid account_id")
		return
	}

	if err := h.users.UpdateAccount(id, accountID); err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Error().Err(err).Msg("admin: update account")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	actorID := auth.UserIDFromContext(r.Context())
	log.Info().
		Str("audit", "admin_action").
		Str("action", "update_user_account").
		Str("actor_id", actorID.String()).
		Str("target_user_id", id.String()).
		Str("new_account_id", accountID.String()).
		Msg("admin: user account updated")

	respondJSON(w, http.StatusOK, map[string]string{"account_id": accountID.String()})
}

// CreateInvite godoc
// POST /admin/invites
// Body: { "email": "..." }
// Response: { "link": "https://frontend/invite?token=..." }
func (h *AdminHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}

	token, err := generateToken()
	if err != nil {
		log.Error().Err(err).Msg("admin: generate invite token")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	invite := &models.Invite{
		Email:     req.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := h.invites.Create(invite); err != nil {
		log.Error().Err(err).Msg("admin: create invite")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	actorID := auth.UserIDFromContext(r.Context())
	log.Info().
		Str("audit", "admin_action").
		Str("action", "create_invite").
		Str("actor_id", actorID.String()).
		Str("invite_email", req.Email).
		Msg("admin: invite created")

	link := h.frontendURL + "/invite?token=" + token
	respondJSON(w, http.StatusCreated, map[string]string{"link": link})
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetMetrics godoc
// GET /admin/metrics
func (h *AdminHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.users.GetMetrics()
	if err != nil {
		log.Error().Err(err).Msg("admin: get metrics")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, metrics)
}
