package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

const webauthnSessionTTL = 5 * time.Minute

// webAuthnUser adapts repository.UserWithSettings + passkeys to implement the webauthn.User interface.
type webAuthnUser struct {
	user     *repository.UserWithSettings
	passkeys []models.Passkey
}

func (u *webAuthnUser) WebAuthnID() []byte           { return u.user.ID[:] }
func (u *webAuthnUser) WebAuthnName() string          { return u.user.Email }
func (u *webAuthnUser) WebAuthnDisplayName() string   { return u.user.Email }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.passkeys))
	for i, pk := range u.passkeys {
		creds[i] = webauthn.Credential{
			ID:        pk.CredentialID,
			PublicKey: pk.PublicKey,
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: pk.SignCount,
			},
		}
	}
	return creds
}

// WebAuthnHandler handles passkey registration and authentication.
type WebAuthnHandler struct {
	wa       *webauthn.WebAuthn
	passkeys *repository.PasskeyRepository
	users    *repository.UserRepository
	jwt      *auth.Manager
}

func NewWebAuthnHandler(
	wa *webauthn.WebAuthn,
	passkeys *repository.PasskeyRepository,
	users *repository.UserRepository,
	jwt *auth.Manager,
) *WebAuthnHandler {
	return &WebAuthnHandler{wa: wa, passkeys: passkeys, users: users, jwt: jwt}
}

// RegisterBegin godoc
// POST /auth/passkey/register/begin  (JWT required)
// Returns PublicKeyCredentialCreationOptions for the client.
func (h *WebAuthnHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.users.FindByID(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "user not found")
		return
	}

	existingPasskeys, err := h.passkeys.FindByUserID(userID)
	if err != nil {
		log.Error().Err(err).Msg("passkey register begin: find existing")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	waUser := &webAuthnUser{user: user, passkeys: existingPasskeys}
	creation, session, err := h.wa.BeginRegistration(waUser)
	if err != nil {
		log.Error().Err(err).Msg("passkey register begin: begin registration")
		respondError(w, http.StatusInternalServerError, "could not begin registration")
		return
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		log.Error().Err(err).Msg("passkey register begin: marshal session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	sessionID := "reg-" + userID.String()
	if err := h.passkeys.SaveSession(sessionID, sessionData, webauthnSessionTTL); err != nil {
		log.Error().Err(err).Msg("passkey register begin: save session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, creation)
}

// RegisterFinish godoc
// POST /auth/passkey/register/finish  (JWT required)
// Validates the credential and stores it.
func (h *WebAuthnHandler) RegisterFinish(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.users.FindByID(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "user not found")
		return
	}

	existingPasskeys, err := h.passkeys.FindByUserID(userID)
	if err != nil {
		log.Error().Err(err).Msg("passkey register finish: find existing")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	sessionID := "reg-" + userID.String()
	sessionData, err := h.passkeys.GetSession(sessionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "registration session not found or expired")
		return
	}
	_ = h.passkeys.DeleteSession(sessionID)

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionData, &session); err != nil {
		log.Error().Err(err).Msg("passkey register finish: unmarshal session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	waUser := &webAuthnUser{user: user, passkeys: existingPasskeys}
	credential, err := h.wa.FinishRegistration(waUser, session, r)
	if err != nil {
		log.Error().Err(err).Msg("passkey register finish: finish registration")
		respondError(w, http.StatusBadRequest, "registration failed: "+err.Error())
		return
	}

	passkey := &models.Passkey{
		UserID:       userID,
		CredentialID: credential.ID,
		PublicKey:    credential.PublicKey,
		AAGUID:       credential.Authenticator.AAGUID,
		SignCount:    credential.Authenticator.SignCount,
	}
	if err := h.passkeys.Create(passkey); err != nil {
		log.Error().Err(err).Msg("passkey register finish: save passkey")
		respondError(w, http.StatusInternalServerError, "could not save passkey")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// LoginBegin godoc
// POST /auth/passkey/login/begin
// No body required — uses discoverable credentials so the browser presents
// available passkeys without needing to know the user's email first.
// Returns: { publicKey: ..., session_id: "..." }
func (h *WebAuthnHandler) LoginBegin(w http.ResponseWriter, r *http.Request) {
	assertion, session, err := h.wa.BeginDiscoverableLogin()
	if err != nil {
		log.Error().Err(err).Msg("passkey login begin: begin discoverable login")
		respondError(w, http.StatusInternalServerError, "could not begin login")
		return
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		log.Error().Err(err).Msg("passkey login begin: marshal session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Use the challenge as the session key — it's already unique and random.
	sessionID := "disc-" + session.Challenge
	if err := h.passkeys.SaveSession(sessionID, sessionData, webauthnSessionTTL); err != nil {
		log.Error().Err(err).Msg("passkey login begin: save session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"publicKey":  assertion.Response,
		"session_id": sessionID,
	})
}

// LoginFinish godoc
// POST /auth/passkey/login/finish?session_id=<id>
// Body: WebAuthn credential assertion JSON
// Returns access_token + user on success.
func (h *WebAuthnHandler) LoginFinish(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id query param is required")
		return
	}

	sessionData, err := h.passkeys.GetSession(sessionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "login session not found or expired")
		return
	}
	_ = h.passkeys.DeleteSession(sessionID)

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionData, &session); err != nil {
		log.Error().Err(err).Msg("passkey login finish: unmarshal session")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// The discoverable handler is called by the library with the raw credential ID
	// and user handle. We look up the passkey by credential ID to find the user.
	var authedUser *repository.UserWithSettings
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		pk, err := h.passkeys.FindByCredentialID(rawID)
		if err != nil {
			return nil, err
		}
		u, err := h.users.FindByID(pk.UserID)
		if err != nil {
			return nil, err
		}
		passkeys, err := h.passkeys.FindByUserID(u.ID)
		if err != nil {
			return nil, err
		}
		authedUser = u
		return &webAuthnUser{user: u, passkeys: passkeys}, nil
	}

	credential, err := h.wa.FinishDiscoverableLogin(handler, session, r)
	if err != nil {
		log.Error().Err(err).Msg("passkey login finish: finish discoverable login")
		respondError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	if authedUser == nil {
		respondError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	// Update sign count to prevent replay attacks.
	if err := h.passkeys.UpdateSignCount(credential.ID, credential.Authenticator.SignCount); err != nil {
		log.Warn().Err(err).Msg("passkey login finish: update sign count")
	}

	pair, err := h.jwt.IssueTokenPair(authedUser.ID, authedUser.AccountID, authedUser.Role, authedUser.Status)
	if err != nil {
		log.Error().Err(err).Msg("passkey login finish: issue token pair")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"user": map[string]string{
			"id":         authedUser.ID.String(),
			"email":      authedUser.Email,
			"role":       authedUser.Role,
			"status":     authedUser.Status,
			"account_id": authedUser.AccountID.String(),
		},
	})
}
