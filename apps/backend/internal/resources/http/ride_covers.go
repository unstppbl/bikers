package httpapi

import (
	"net/http"
	"strconv"
)

func (s *Server) rideCover(w http.ResponseWriter, r *http.Request) {
	rideID, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	cover, err := s.managers.Rides.Cover(r.Context(), rideID, session)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, cover)
}

func (s *Server) uploadRideCover(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	rideID, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	values := r.URL.Query()["version"]
	if len(values) != 1 {
		problem(w, 422, "invalid_photo_choice", "Choose the current ride cover version")
		return
	}
	version, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || version < 1 || strconv.FormatInt(version, 10) != values[0] {
		problem(w, 422, "invalid_photo_choice", "Choose the current ride cover version")
		return
	}
	ctx, cancel, ok := photoUploadBody(w, r)
	if !ok {
		return
	}
	defer cancel()
	cover, err := s.managers.Rides.UploadCover(ctx, session.User, rideID, version, r.Body)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusCreated, cover)
}

func (s *Server) removeRideCover(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	rideID, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	var input struct {
		Version int64 `json:"version"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Version < 1 {
		problem(w, 422, "invalid_photo_choice", "Choose the current ride cover version")
		return
	}
	cover, err := s.managers.Rides.RemoveCover(r.Context(), session.User, rideID, input.Version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, cover)
}

func (s *Server) rideCoverContent(w http.ResponseWriter, r *http.Request) {
	rideID, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	coverID, ok := idParam(w, r, "coverID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	content, err := s.managers.Rides.CoverContent(r.Context(), rideID, coverID, session)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, content)
}
