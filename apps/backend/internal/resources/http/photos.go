package httpapi

import (
	"context"
	"mime"
	"net/http"
	"strconv"
	"time"

	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
)

func (s *Server) bikePhotos(w http.ResponseWriter, r *http.Request) {
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	gallery, err := s.managers.Garage.Photos(r.Context(), bikeID, session)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, gallery)
}

func (s *Server) uploadBikePhoto(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	ctx, cancel, ok := photoUploadBody(w, r)
	if !ok {
		return
	}
	defer cancel()
	result, err := s.managers.Garage.UploadPhoto(ctx, session.User, bikeID, r.Body)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, result)
}

func photoUploadBody(w http.ResponseWriter, r *http.Request) (context.Context, context.CancelFunc, bool) {
	if r.ContentLength > photos.MaxPhotoUploadBytes {
		problem(w, 413, "photo_too_large", "Photo exceeds the 12 MiB upload limit")
		return nil, nil, false
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" && contentType != "application/octet-stream") {
		problem(w, 415, "unsupported_photo", "Upload JPEG, PNG or WebP as a raw image body")
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	// The processor observes one overflow byte and maps it to photo_too_large.
	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxPhotoUploadBytes+1)
	return ctx, cancel, true
}

func (s *Server) setBikePhotoCover(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	var input models.PhotoCoverInput
	if !decode(w, r, &input) {
		return
	}
	gallery, err := s.managers.Garage.SetPhotoCover(r.Context(), session.User, bikeID, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"gallery": gallery})
}

func (s *Server) deleteBikePhoto(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	photoID, ok := idParam(w, r, "photoID")
	if !ok {
		return
	}
	var input struct {
		Version int64 `json:"version"`
	}
	if !decode(w, r, &input) {
		return
	}
	gallery, err := s.managers.Garage.DeletePhoto(r.Context(), session.User, bikeID, photoID, input.Version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"gallery": gallery})
}

func (s *Server) bikePhotoContent(w http.ResponseWriter, r *http.Request) {
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	photoID, ok := idParam(w, r, "photoID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	content, err := s.managers.Garage.PhotoContent(r.Context(), bikeID, photoID, session)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, content)
}

func writePhotoContent(w http.ResponseWriter, r *http.Request, content models.PhotoContent) {
	// Authorize before emitting bytes, including conditional and HEAD requests.
	// Deliberately omit validators so visibility changes cannot yield a cached 304.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(content.Bytes)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content.Bytes)
	}
}
