package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
	"strconv"
)

func imageVersion(w http.ResponseWriter, r *http.Request) (int64, bool) {
	values := r.URL.Query()["version"]
	if len(values) != 1 {
		problem(w, 422, "invalid_photo_choice", "Choose the current image version")
		return 0, false
	}
	v, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || v < 1 || strconv.FormatInt(v, 10) != values[0] {
		problem(w, 422, "invalid_photo_choice", "Choose the current image version")
		return 0, false
	}
	return v, true
}
func deleteImageVersion(w http.ResponseWriter, r *http.Request) (int64, bool) {
	var input struct {
		Version int64 `json:"version"`
	}
	if !decode(w, r, &input) {
		return 0, false
	}
	if input.Version < 1 {
		problem(w, 422, "invalid_photo_choice", "Choose the current image version")
		return 0, false
	}
	return input.Version, true
}
func (s *Server) mediaReader(w http.ResponseWriter, r *http.Request) *models.Session {
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return nil
	}
	if session == nil {
		problem(w, 404, "not_found", "Record not found")
	}
	return session
}
func (s *Server) meAvatar(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, r.Method != "GET")
	if session == nil {
		return
	}
	switch r.Method {
	case "GET":
		out, err := s.managers.Media.Avatar(r.Context(), session.User)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, out)
	case "POST":
		version, ok := imageVersion(w, r)
		if !ok {
			return
		}
		ctx, cancel, ok := photoUploadBody(w, r)
		if !ok {
			return
		}
		defer cancel()
		out, err := s.managers.Media.UploadAvatar(ctx, session.User, version, r.Body)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 201, out)
	case "DELETE":
		version, ok := deleteImageVersion(w, r)
		if !ok {
			return
		}
		out, err := s.managers.Media.RemoveAvatar(r.Context(), session.User, version)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, out)
	}
}
func (s *Server) avatarContent(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "imageID")
	if !ok {
		return
	}
	out, err := s.managers.Media.AvatarContent(r.Context(), session.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, out)
}
func (s *Server) contextAvatar(w http.ResponseWriter, r *http.Request) {
	session := s.mediaReader(w, r)
	if session == nil {
		return
	}
	scope, ok := idParam(w, r, "scopeID")
	if !ok {
		return
	}
	target, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var out models.Avatar
	var err error
	if r.PathValue("scope") == "clubs" {
		out, err = s.managers.Media.ClubAvatar(r.Context(), session.User, scope, target)
	} else {
		out, err = s.managers.Media.RideAvatar(r.Context(), session.User, scope, target)
	}
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) contextAvatarContent(w http.ResponseWriter, r *http.Request) {
	session := s.mediaReader(w, r)
	if session == nil {
		return
	}
	scope, ok := idParam(w, r, "scopeID")
	if !ok {
		return
	}
	target, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	image, ok := idParam(w, r, "imageID")
	if !ok {
		return
	}
	var out models.PhotoContent
	var err error
	if r.PathValue("scope") == "clubs" {
		out, err = s.managers.Media.ClubAvatarContent(r.Context(), session.User, scope, target, image)
	} else {
		out, err = s.managers.Media.RideAvatarContent(r.Context(), session.User, scope, target, image)
	}
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, out)
}
func (s *Server) rideAlbum(w http.ResponseWriter, r *http.Request) {
	session := s.mediaReader(w, r)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	out, err := s.managers.Media.Album(r.Context(), session.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) uploadRidePhoto(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	ctx, cancel, ok := photoUploadBody(w, r)
	if !ok {
		return
	}
	defer cancel()
	out, err := s.managers.Media.UploadRidePhoto(ctx, session.User, id, r.Body)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, out)
}
func (s *Server) deleteRidePhoto(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	photo, ok := idParam(w, r, "photoID")
	if !ok {
		return
	}
	version, ok := deleteImageVersion(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Media.DeleteRidePhoto(r.Context(), session.User, id, photo, version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) ridePhotoContent(w http.ResponseWriter, r *http.Request) {
	session := s.mediaReader(w, r)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	photo, ok := idParam(w, r, "photoID")
	if !ok {
		return
	}
	out, err := s.managers.Media.RidePhotoContent(r.Context(), session.User, id, photo)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, out)
}
func (s *Server) addRideVideo(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	var input models.RideVideoInput
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Media.AddRideVideo(r.Context(), session.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, out)
}
func (s *Server) deleteRideVideo(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	video, ok := idParam(w, r, "videoID")
	if !ok {
		return
	}
	version, ok := deleteImageVersion(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Media.DeleteRideVideo(r.Context(), session.User, id, video, version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
