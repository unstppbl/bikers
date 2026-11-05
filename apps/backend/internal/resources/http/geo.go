package httpapi

import (
	"net/http"

	"bikers.kz/backend/internal/models"
)

func (s *Server) geoConfig(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, false) == nil {
		return
	}
	respond(w, http.StatusOK, s.managers.Geo.Config())
}

func (s *Server) geoSearch(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	var input models.MapSearch
	if !decode(w, r, &input) {
		return
	}
	places, err := s.managers.Geo.Search(r.Context(), session.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"places": places})
}

func (s *Server) geoReverse(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	var input struct {
		Point struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"point"`
		Lang string `json:"lang"`
	}
	if !decode(w, r, &input) {
		return
	}
	places, err := s.managers.Geo.Reverse(r.Context(), session.User, models.MapPoint{Lat: input.Point.Lat, Lon: input.Point.Lon}, input.Lang)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"places": places})
}

func (s *Server) geoRoute(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	var input models.MapRouteInput
	if !decode(w, r, &input) {
		return
	}
	plan, err := s.managers.Geo.Route(r.Context(), session.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"plan": plan})
}
