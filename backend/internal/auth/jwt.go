package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenTTL  = 3 * 24 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

// TokenPair holds both tokens returned after login or refresh.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Claims is the payload embedded in every JWT.
type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	AccountID uuid.UUID `json:"account_id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	jwt.RegisteredClaims
}

// Manager holds the RSA key pair and issues / validates tokens.
type Manager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewManager parses the PEM-encoded keys from config and returns a Manager.
func NewManager(privatePEM, publicPEM string) (*Manager, error) {
	priv, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privatePEM))
	if err != nil {
		return nil, errors.New("auth: invalid JWT private key")
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicPEM))
	if err != nil {
		return nil, errors.New("auth: invalid JWT public key")
	}
	return &Manager{privateKey: priv, publicKey: pub}, nil
}

// IssueTokenPair generates a short-lived access token and a long-lived refresh token.
func (m *Manager) IssueTokenPair(userID, accountID uuid.UUID, role, status string) (TokenPair, error) {
	access, err := m.sign(userID, accountID, role, status, accessTokenTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := m.sign(userID, accountID, role, status, refreshTokenTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// ValidateAccessToken parses and validates an access token, returning its claims.
func (m *Manager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return m.parse(tokenStr)
}

// ValidateRefreshToken parses and validates a refresh token, returning its claims.
// The handler must issue a new pair and rotate the refresh token on success.
func (m *Manager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return m.parse(tokenStr)
}

// sign creates a signed JWT with the given TTL.
func (m *Manager) sign(userID, accountID uuid.UUID, role, status string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		Status:    status,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(m.privateKey)
	if err != nil {
		return "", errors.New("auth: failed to sign token")
	}
	return signed, nil
}

// parse validates signature, expiry, and algorithm, then returns claims.
func (m *Manager) parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("auth: unexpected signing method")
		}
		return m.publicKey, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ErrInvalidToken is returned when a token is malformed, expired, or has a bad signature.
var ErrInvalidToken = errors.New("auth: invalid or expired token")
