package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"album-store/models"
	"album-store/store"
	"album-store/worker"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	db   *store.DynamoDB
	s3   *store.S3Store
	pool *worker.Pool
}

// New constructs a Handler.
func New(db *store.DynamoDB, s3 *store.S3Store, pool *worker.Pool) *Handler {
	return &Handler{db: db, s3: s3, pool: pool}
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// ─── Health ───────────────────────────────────────────────────────────────────

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Albums ───────────────────────────────────────────────────────────────────

// PutAlbum handles PUT /albums/:album_id — create or update (idempotent).
func (h *Handler) PutAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	var req models.Album
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid json"})
		return
	}
	// Always use the path param as canonical album_id.
	req.AlbumID = albumID

	if err := h.db.PutAlbum(r.Context(), req); err != nil {
		log.Printf("ERROR PutAlbum %s: %v", albumID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}

	// 200 for upsert (spec accepts 200 or 201).
	writeJSON(w, http.StatusOK, req)
}

// GetAlbum handles GET /albums/:album_id
func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	album, err := h.db.GetAlbum(r.Context(), albumID)
	if err != nil {
		log.Printf("ERROR GetAlbum %s: %v", albumID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	if album == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	writeJSON(w, http.StatusOK, album)
}

// ListAlbums handles GET /albums — returns every album ever created.
func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.db.ListAlbums(r.Context())
	if err != nil {
		log.Printf("ERROR ListAlbums: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}

	// Return bare array; never null.
	if albums == nil {
		albums = []models.Album{}
	}
	writeJSON(w, http.StatusOK, albums)
}

// ─── Photos ───────────────────────────────────────────────────────────────────

// UploadPhoto handles POST /albums/:album_id/photos
//
// Returns 202 immediately after:
//  1. Reading the file bytes from the multipart body.
//  2. Atomically assigning a seq number (DynamoDB ADD).
//  3. Writing the photo record as status=processing.
//  4. Submitting the upload job to the background worker pool.
func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")

	// Allow up to 256 MB in memory; overflow goes to disk temp files.
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "failed to parse multipart form"})
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "missing 'photo' field"})
		return
	}
	defer file.Close()

	// Read all bytes so the worker goroutine can reference them after
	// this handler returns (and the request body is closed).
	data, err := io.ReadAll(file)
	if err != nil {
		log.Printf("ERROR UploadPhoto read body album=%s: %v", albumID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to read file"})
		return
	}

	// Detect content type from the part header (falls back to binary).
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// ── Assign seq SYNCHRONOUSLY (spec requirement) ──
	seq, err := h.db.IncrementAndGetSeq(r.Context(), albumID)
	if err != nil {
		log.Printf("ERROR UploadPhoto seq album=%s: %v", albumID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}

	photoID := uuid.New().String()
	s3Key := "photos/" + photoID

	// Write the metadata record with status=processing before we return 202.
	photo := models.Photo{
		PhotoID: photoID,
		AlbumID: albumID,
		Seq:     seq,
		Status:  "processing",
	}
	if err := h.db.PutPhoto(r.Context(), photo); err != nil {
		log.Printf("ERROR UploadPhoto PutPhoto album=%s photo=%s: %v", albumID, photoID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}

	// Hand off to the worker pool — does NOT block this handler.
	h.pool.Submit(worker.Job{
		PhotoID:     photoID,
		AlbumID:     albumID,
		Data:        data,
		Key:         s3Key,
		ContentType: contentType,
	})

	writeJSON(w, http.StatusAccepted, models.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})
}

// GetPhoto handles GET /albums/:album_id/photos/:photo_id
func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	photo, err := h.db.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		log.Printf("ERROR GetPhoto album=%s photo=%s: %v", albumID, photoID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	if photo == nil {
		writeJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	writeJSON(w, http.StatusOK, photo)
}

// DeletePhoto handles DELETE /albums/:album_id/photos/:photo_id
//
// Must complete within 5 seconds — both S3 and DynamoDB deletes are
// performed synchronously in this handler (no background worker).
func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	// Check whether the photo exists at all.
	photo, err := h.db.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		log.Printf("ERROR DeletePhoto get album=%s photo=%s: %v", albumID, photoID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}
	// Idempotent: already gone → 204.
	if photo == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Delete the file from S3 first.
	s3Key := "photos/" + photoID
	if err := h.s3.Delete(r.Context(), s3Key); err != nil {
		// Log but don't abort — still remove the DB record so GET returns 404.
		log.Printf("WARN DeletePhoto S3 delete album=%s photo=%s: %v", albumID, photoID, err)
	}

	// Delete the metadata record.
	if err := h.db.DeletePhoto(r.Context(), albumID, photoID); err != nil {
		log.Printf("ERROR DeletePhoto DB delete album=%s photo=%s: %v", albumID, photoID, err)
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}