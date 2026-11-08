package models

import (
	"time"

	"github.com/google/uuid"
)

// Passkey stores a WebAuthn credential for a user.
type Passkey struct {
	ID           uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID `gorm:"not null;index;type:uuid"                      json:"user_id"`
	CredentialID []byte    `gorm:"not null;uniqueIndex"                          json:"credential_id"`
	PublicKey    []byte    `gorm:"not null"                                      json:"-"`
	AAGUID       []byte    `                                                     json:"-"`
	SignCount    uint32    `gorm:"not null;default:0"                            json:"-"`
	CreatedAt    time.Time `                                                     json:"created_at"`
}

// WebAuthnSession temporarily stores a challenge between begin and finish calls.
type WebAuthnSession struct {
	ID        string    `gorm:"primarykey"`
	Data      []byte    `gorm:"not null;type:bytea"`
	ExpiresAt time.Time `gorm:"not null;index"`
}
