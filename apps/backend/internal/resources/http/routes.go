package httpapi

import (
	"net/http"
	"strconv"

	"bikers.kz/backend/internal/managers/routes"
	"bikers.kz/backend/internal/models"
)

func (s *Server) communityRouteEndpoints(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/routes", s.communityRoutes)
	m.HandleFunc("POST /api/v1/routes", s.createCommunityRoute)
	m.HandleFunc("GET /api/v1/routes/{routeID}", s.communityRoute)
	m.HandleFunc("PUT /api/v1/routes/{routeID}", s.updateCommunityRoute)
	m.HandleFunc("DELETE /api/v1/routes/{routeID}", s.deleteCommunityRoute)
	m.HandleFunc("GET /api/v1/route-collections", s.routeCollections)
	m.HandleFunc("POST /api/v1/route-collections", s.createRouteCollection)
	m.HandleFunc("PUT /api/v1/route-collections/{collectionID}", s.updateRouteCollection)
	m.HandleFunc("DELETE /api/v1/route-collections/{collectionID}", s.deleteRouteCollection)
	m.HandleFunc("PUT /api/v1/route-collections/{collectionID}/routes/{routeID}", s.setCollectionRoute)
	m.HandleFunc("DELETE /api/v1/route-collections/{collectionID}/routes/{routeID}", s.setCollectionRoute)
	m.HandleFunc("GET /api/v1/tracks", s.routeTracks)
	m.HandleFunc("POST /api/v1/tracks", s.createRouteTrack)
	m.HandleFunc("GET /api/v1/tracks/{trackID}", s.routeTrack)
	m.HandleFunc("DELETE /api/v1/tracks/{trackID}", s.deleteRouteTrack)
	m.HandleFunc("GET /api/v1/routes/{routeID}/conditions", s.routeConditions)
	m.HandleFunc("POST /api/v1/routes/{routeID}/conditions", s.createRouteCondition)
	m.HandleFunc("PUT /api/v1/routes/{routeID}/conditions/{conditionID}", s.updateRouteCondition)
	m.HandleFunc("DELETE /api/v1/routes/{routeID}/conditions/{conditionID}", s.deleteRouteCondition)
}

func routeReply(w http.ResponseWriter, status int, key string, result any, err error) {
	if err != nil {
		failure(w, err)
		return
	}
	if key != "" {
		result = map[string]any{key: result}
	}
	respond(w, status, result)
}

func routeLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	if !r.URL.Query().Has("limit") {
		return 0, true
	}
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n < 1 || n > 100 {
		failure(w, models.Fail("invalid_query", "Choose a page size from 1 to 100"))
		return 0, false
	}
	return n, true
}

// A zero user is an anonymous public reader; a broken session is never silently
// retried as anonymous, which would bypass the signed-in block context.
func (s *Server) routeReader(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	v, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return models.User{}, false
	}
	if v != nil {
		return v.User, true
	}
	return models.User{}, true
}

func (s *Server) communityRoutes(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.routeReader(w, r)
	if !ok {
		return
	}
	limit, ok := routeLimit(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	out, err := s.managers.Routes.List(r.Context(), actor, routes.ListInput{Scope: q.Get("scope"), Search: q.Get("q"), City: q.Get("city"), After: q.Get("after"), CollectionID: q.Get("collection"), Limit: limit})
	routeReply(w, 200, "", out, err)
}

func (s *Server) communityRoute(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.routeReader(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Routes.View(r.Context(), actor, r.PathValue("routeID"))
	routeReply(w, 200, "route", out, err)
}

func (s *Server) createCommunityRoute(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.CommunityRouteInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.Create(r.Context(), actor.User, in)
	routeReply(w, 201, "route", out, err)
}

func (s *Server) updateCommunityRoute(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.CommunityRouteInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.Update(r.Context(), actor.User, r.PathValue("routeID"), in)
	routeReply(w, 200, "route", out, err)
}

type routeVersion struct {
	Version int64 `json:"version"`
}

func (s *Server) deleteCommunityRoute(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in routeVersion
	if !decode(w, r, &in) {
		return
	}
	err := s.managers.Routes.Delete(r.Context(), actor.User, r.PathValue("routeID"), in.Version)
	routeReply(w, 200, "ok", true, err)
}

func (s *Server) routeCollections(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, false)
	if actor == nil {
		return
	}
	limit, ok := routeLimit(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Routes.Collections(r.Context(), actor.User, r.URL.Query().Get("after"), limit)
	routeReply(w, 200, "", out, err)
}

func (s *Server) createRouteCollection(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.RouteCollectionInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.CreateCollection(r.Context(), actor.User, in)
	routeReply(w, 201, "collection", out, err)
}

func (s *Server) updateRouteCollection(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.RouteCollectionInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.UpdateCollection(r.Context(), actor.User, r.PathValue("collectionID"), in)
	routeReply(w, 200, "collection", out, err)
}

func (s *Server) deleteRouteCollection(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in routeVersion
	if !decode(w, r, &in) {
		return
	}
	err := s.managers.Routes.DeleteCollection(r.Context(), actor.User, r.PathValue("collectionID"), in.Version)
	routeReply(w, 200, "ok", true, err)
}

func (s *Server) setCollectionRoute(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	err := s.managers.Routes.SetCollectionRoute(r.Context(), actor.User, r.PathValue("collectionID"), r.PathValue("routeID"), r.Method == http.MethodPut)
	routeReply(w, 200, "ok", true, err)
}

func (s *Server) routeTracks(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, false)
	if actor == nil {
		return
	}
	limit, ok := routeLimit(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Routes.Tracks(r.Context(), actor.User, r.URL.Query().Get("after"), limit)
	routeReply(w, 200, "", out, err)
}

func (s *Server) routeTrack(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, false)
	if actor == nil {
		return
	}
	out, err := s.managers.Routes.Track(r.Context(), actor.User, r.PathValue("trackID"))
	routeReply(w, 200, "track", out, err)
}

func (s *Server) createRouteTrack(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.TrackInput
	if !decodeBounded(w, r, &in, 2<<20) {
		return
	}
	out, err := s.managers.Routes.CreateTrack(r.Context(), actor.User, in)
	routeReply(w, 201, "track", out, err)
}

func (s *Server) deleteRouteTrack(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in routeVersion
	if !decode(w, r, &in) {
		return
	}
	err := s.managers.Routes.DeleteTrack(r.Context(), actor.User, r.PathValue("trackID"), in.Version)
	routeReply(w, 200, "ok", true, err)
}

func (s *Server) routeConditions(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.routeReader(w, r)
	if !ok {
		return
	}
	limit, ok := routeLimit(w, r)
	if !ok {
		return
	}
	out, err := s.managers.Routes.Conditions(r.Context(), actor, r.PathValue("routeID"), r.URL.Query().Get("after"), limit)
	routeReply(w, 200, "", out, err)
}

func (s *Server) createRouteCondition(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.RouteConditionInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.CreateCondition(r.Context(), actor.User, r.PathValue("routeID"), in)
	routeReply(w, 201, "condition", out, err)
}

func (s *Server) updateRouteCondition(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in models.RouteConditionInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.managers.Routes.UpdateCondition(r.Context(), actor.User, r.PathValue("routeID"), r.PathValue("conditionID"), in)
	routeReply(w, 200, "condition", out, err)
}

func (s *Server) deleteRouteCondition(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var in routeVersion
	if !decode(w, r, &in) {
		return
	}
	err := s.managers.Routes.DeleteCondition(r.Context(), actor.User, r.PathValue("routeID"), r.PathValue("conditionID"), in.Version)
	routeReply(w, 200, "ok", true, err)
}
