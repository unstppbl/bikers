package httpapi

import (
	"net/http"
	"net/url"
	"strconv"

	"bikers.kz/backend/internal/models"
)

func (s *Server) geoConfig(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, false) == nil {
		return
	}
	respond(w, http.StatusOK, s.managers.Geo.Config())
}

func (s *Server) geoSearch(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	query := r.URL.Query()
	var near *models.MapPoint
	if query.Has("lat") || query.Has("lon") {
		point, err := mapCoordinate(query)
		if err != nil {
			failure(w, err)
			return
		}
		near = &point
	}
	places, err := s.managers.Geo.Search(r.Context(), session.User, models.MapSearch{Query: query.Get("q"), Lang: query.Get("lang"), Near: near})
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"places": places})
}

func (s *Server) geoReverse(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	query := r.URL.Query()
	point, err := mapCoordinate(query)
	if err != nil {
		failure(w, err)
		return
	}
	places, err := s.managers.Geo.Reverse(r.Context(), session.User, point, query.Get("lang"))
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

func mapCoordinate(query url.Values) (models.MapPoint, error) {
	lat, latErr := strconv.ParseFloat(query.Get("lat"), 64)
	lon, lonErr := strconv.ParseFloat(query.Get("lon"), 64)
	if latErr != nil || lonErr != nil || !models.ValidCoordinate(lat, lon) {
		return models.MapPoint{}, models.Fail("invalid_map", "Provide valid latitude and longitude")
	}
	return models.MapPoint{Lat: lat, Lon: lon}, nil
}
