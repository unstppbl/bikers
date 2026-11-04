package httpapi

import (
	"net/http"
	"strconv"
)

func (s *Server) clubImage(w http.ResponseWriter, r *http.Request) {
	clubID, ok := idParam(w, r, "clubID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	image, err := s.managers.Clubs.Image(r.Context(), clubID, session)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, image)
}

func (s *Server) uploadClubImage(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	clubID, ok := idParam(w, r, "clubID")
	if !ok {
		return
	}
	values := r.URL.Query()["version"]
	if len(values) != 1 {
		problem(w, 422, "invalid_photo_choice", "Choose the current club image version")
		return
	}
	version, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || version < 1 || strconv.FormatInt(version, 10) != values[0] {
		problem(w, 422, "invalid_photo_choice", "Choose the current club image version")
		return
	}
	ctx, cancel, ok := photoUploadBody(w, r)
	if !ok {
		return
	}
	defer cancel()
	image, err := s.managers.Clubs.UploadImage(ctx, session.User, clubID, version, r.Body)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusCreated, image)
}

func (s *Server) removeClubImage(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	clubID, ok := idParam(w, r, "clubID")
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
		problem(w, 422, "invalid_photo_choice", "Choose the current club image version")
		return
	}
	image, err := s.managers.Clubs.RemoveImage(r.Context(), session.User, clubID, input.Version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, image)
}

func (s *Server) clubImageContent(w http.ResponseWriter, r *http.Request) {
	clubID, ok := idParam(w, r, "clubID")
	if !ok {
		return
	}
	imageID, ok := idParam(w, r, "imageID")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	content, err := s.managers.Clubs.ImageContent(r.Context(), clubID, imageID, session)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, content)
}
