package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
	"strconv"
)

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, false)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	after := int64(0)
	if value := r.URL.Query().Get("after"); value != "" {
		var err error
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 {
			problem(w, 422, "invalid_cursor", "Invalid message cursor")
			return
		}
	}
	result, err := s.managers.Rooms.Messages(r.Context(), id, v.User.ID, after)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"messages": result.Messages, "next_sequence": result.NextSequence})
}
func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input models.MessageInput
	if !decode(w, r, &input) {
		return
	}
	result, err := s.managers.Rooms.Send(r.Context(), id, v.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	status := 200
	if result.Created {
		status = 201
	}
	respond(w, status, map[string]any{"message": result.Message})
}
