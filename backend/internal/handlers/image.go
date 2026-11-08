package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/marianamohr/financas/internal/services/storage"
	"github.com/rs/zerolog/log"
)

type ImageHandler struct {
	storage storage.Storage
}

func NewImageHandler(s storage.Storage) *ImageHandler {
	return &ImageHandler{storage: s}
}

// Get generates a presigned URL for the given key and redirects the client to it.
// The endpoint is public so <img> tags can load images without a Bearer token.
func (h *ImageHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "missing key")
		return
	}

	url, err := h.storage.PresignGetURL(r.Context(), key, time.Hour)
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("image: presign")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
