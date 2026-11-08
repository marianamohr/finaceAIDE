package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// LocalClient stores files on the local filesystem.
// Used in development when AWS credentials are not configured.
type LocalClient struct {
	dir     string // absolute path to upload directory
	baseURL string // e.g. http://localhost:8080/uploads
}

// NewLocalClient creates a LocalClient that writes files to dir
// and serves them under baseURL.
func NewLocalClient(dir, baseURL string) (*LocalClient, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("storage: create local dir: %w", err)
	}
	return &LocalClient{dir: dir, baseURL: baseURL}, nil
}

// Upload saves data to a UUID-named file and returns the key (filename).
func (c *LocalClient) Upload(_ context.Context, data []byte, mimeType string) (string, error) {
	ext := mimeExt(mimeType)
	key := uuid.New().String() + ext
	path := filepath.Join(c.dir, key)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("storage: write local file: %w", err)
	}
	return key, nil
}

// PublicURL returns the URL to serve the file.
func (c *LocalClient) PublicURL(_, key string) string {
	return fmt.Sprintf("%s/%s", c.baseURL, key)
}

// PresignGetURL returns the regular public URL (local files need no signing).
func (c *LocalClient) PresignGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return fmt.Sprintf("%s/%s", c.baseURL, key), nil
}

func mimeExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
