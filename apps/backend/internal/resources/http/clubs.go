package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
)

func (s *Server) listClubs(w http.ResponseWriter, r *http.Request) {
	v, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	params := r.URL.Query()
	page, err := s.managers.Clubs.List(r.Context(), v, models.ClubListInput{
		View: params.Get("view"), City: params.Get("city"), Search: params.Get("q"), Cursor: params.Get("cursor"),
	})
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, page)
}
func (s *Server) createClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	var input models.ClubInput
	if !decode(w, r, &input) {
		return
	}
	club, err := s.managers.Clubs.Create(r.Context(), v.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"club": club})
}
func (s *Server) getClub(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	v, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	club, err := s.managers.Clubs.View(r.Context(), id, v)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"club": club})
}
func (s *Server) patchClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input models.ClubPatch
	if !decode(w, r, &input) {
		return
	}
	club, err := s.managers.Clubs.Update(r.Context(), v.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"club": club})
}
func (s *Server) closeClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		Version int64 `json:"version"`
	}
	if !decode(w, r, &input) {
		return
	}
	club, err := s.managers.Clubs.Close(r.Context(), v.User, id, input.Version)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"club": club})
}
func (s *Server) transferClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input models.ClubTransfer
	if !decode(w, r, &input) {
		return
	}
	club, err := s.managers.Clubs.Transfer(r.Context(), v.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"club": club})
}
func (s *Server) joinClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	if !clubEmptyInput(w, r) {
		return
	}
	state, err := s.managers.Clubs.Join(r.Context(), v.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) leaveClub(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	if !clubEmptyInput(w, r) {
		return
	}
	state, err := s.managers.Clubs.Leave(r.Context(), v.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) clubMembers(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, false)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	members, err := s.managers.Clubs.Members(r.Context(), v.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"members": members})
}
func (s *Server) decideClubMember(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	target, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var input struct {
		Decision string `json:"decision"`
	}
	if !decode(w, r, &input) {
		return
	}
	state, err := s.managers.Clubs.Decide(r.Context(), v.User, id, target, input.Decision)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) clubRides(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	v, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	rides, err := s.managers.Clubs.Rides(r.Context(), id, v)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"rides": rides})
}
func (s *Server) linkClubRide(w http.ResponseWriter, r *http.Request) { s.changeClubRide(w, r, true) }
func (s *Server) unlinkClubRide(w http.ResponseWriter, r *http.Request) {
	s.changeClubRide(w, r, false)
}
func (s *Server) changeClubRide(w http.ResponseWriter, r *http.Request, link bool) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	ride, ok := idParam(w, r, "rideID")
	if !ok {
		return
	}
	if !clubEmptyInput(w, r) {
		return
	}
	var err error
	if link {
		err = s.managers.Clubs.Link(r.Context(), v.User, id, ride)
	} else {
		err = s.managers.Clubs.Unlink(r.Context(), v.User, id, ride)
	}
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

// These operations need no body. An optional JSON object still rejects unknown
// fields instead of accepting a caller-supplied role or membership state.
func clubEmptyInput(w http.ResponseWriter, r *http.Request) bool {
	if r.ContentLength == 0 {
		return true
	}
	var input struct{}
	return decode(w, r, &input)
}
