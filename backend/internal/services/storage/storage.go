package storage

import (
	"context"
	"time"
)

// Storage is the interface implemented by both Client (S3) and LocalClient.
type Storage interface {
	Upload(ctx context.Context, data []byte, mimeType string) (string, error)
	PublicURL(endpoint, key string) string
	// PresignGetURL returns a time-limited download URL for the given object key.
	PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}
