package httpapi

import (
	"bikers.kz/backend/internal/managers/places"
	"bikers.kz/backend/internal/models"
	"net/http"
	"strconv"
)

func (s *Server) placeConfig(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, s.managers.Places.Config())
}
func (s *Server) listPlaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := places.ListInput{Search: q.Get("q"), Category: q.Get("category"), City: q.Get("city"), After: q.Get("after")}
	if q.Has("limit") {
		v, e := strconv.Atoi(q.Get("limit"))
		if e != nil || v < 1 {
			failure(w, models.Fail("invalid_query", "Invalid place page size"))
			return
		}
		in.Limit = v
	}
	keys := []string{"west", "south", "east", "north"}
	present := 0
	coords := [4]float64{}
	for i, key := range keys {
		if q.Has(key) {
			present++
			v, e := strconv.ParseFloat(q.Get(key), 64)
			if e != nil {
				failure(w, models.Fail("invalid_query", "Invalid place map bounds"))
				return
			}
			coords[i] = v
		}
	}
	if present != 0 && present != 4 {
		failure(w, models.Fail("invalid_query", "All four place map bounds are required"))
		return
	}
	if present == 4 {
		in.Bounds = &models.PlaceBounds{West: coords[0], South: coords[1], East: coords[2], North: coords[3]}
	}
	out, err := s.managers.Places.List(r.Context(), in)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) getPlace(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "placeID")
	if !ok {
		return
	}
	out, err := s.managers.Places.View(r.Context(), id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"place": out})
}
func (s *Server) submitPlace(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	var in models.PlaceSubmissionInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Places.Submit(r.Context(), u.User, in)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"submission": out})
}
func (s *Server) ownPlaceSubmissions(w http.ResponseWriter, r *http.Request) {
	s.placeSubmissions(w, r, false)
}
func (s *Server) adminPlaceSubmissions(w http.ResponseWriter, r *http.Request) {
	s.placeSubmissions(w, r, true)
}
func (s *Server) placeSubmissions(w http.ResponseWriter, r *http.Request, admin bool) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	out, err := s.managers.Places.Submissions(r.Context(), u.User, admin, r.URL.Query().Get("status"), r.URL.Query().Get("after"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) reviewPlaceSubmission(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	var in models.PlaceReviewInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Places.Review(r.Context(), u.User, id, in)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"submission": out})
}
