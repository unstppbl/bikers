package httpapi

import (
	"net/http"

	"bikers.kz/backend/internal/models"
)

func (s *Server) searchBikeModels(w http.ResponseWriter, r *http.Request) {
	entries, err := s.managers.Garage.SearchModels(r.URL.Query().Get("query"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"models": entries})
}
func (s *Server) listBikes(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	page, err := s.managers.Garage.List(r.Context(), session.User, r.URL.Query().Get("cursor"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, page)
}

func (s *Server) publicBikes(w http.ResponseWriter, r *http.Request) {
	page, err := s.managers.Garage.Catalogue(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, page)
}

func (s *Server) createBike(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	var input models.BikeInput
	if !decode(w, r, &input) {
		return
	}
	bike, err := s.managers.Garage.Create(r.Context(), session.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"bike": bike})
}
func (s *Server) getBike(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	session, err := s.readSession(r.Context(), r, true)
	if err != nil {
		failure(w, err)
		return
	}
	bike, err := s.managers.Garage.View(r.Context(), id, session)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"bike": bike})
}
func (s *Server) patchBike(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input models.BikePatch
	if !decode(w, r, &input) {
		return
	}
	bike, err := s.managers.Garage.Update(r.Context(), session.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"bike": bike})
}
func (s *Server) deleteBike(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
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
	if err := s.managers.Garage.Delete(r.Context(), session.User, id, input.Version); err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) listMaintenance(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	page, err := s.managers.Garage.Maintenance(r.Context(), session.User, id, r.URL.Query().Get("cursor"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, page)
}
func (s *Server) createMaintenance(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	var input models.MaintenanceInput
	if !decode(w, r, &input) {
		return
	}
	entry, err := s.managers.Garage.CreateMaintenance(r.Context(), session.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"entry": entry})
}
func (s *Server) patchMaintenance(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	entryID, ok := idParam(w, r, "entryID")
	if !ok {
		return
	}
	var input models.MaintenancePatch
	if !decode(w, r, &input) {
		return
	}
	entry, err := s.managers.Garage.UpdateMaintenance(r.Context(), session.User, bikeID, entryID, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"entry": entry})
}
func (s *Server) deleteMaintenance(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	bikeID, ok := idParam(w, r, "bikeID")
	if !ok {
		return
	}
	entryID, ok := idParam(w, r, "entryID")
	if !ok {
		return
	}
	var input struct {
		Version int64 `json:"version"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.managers.Garage.DeleteMaintenance(r.Context(), session.User, bikeID, entryID, input.Version); err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
