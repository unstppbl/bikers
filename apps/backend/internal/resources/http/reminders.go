package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
)

func (s *Server) rideReminders(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, r.Method != "GET")
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var out models.RideReminders
	var err error
	if r.Method == "PUT" {
		var input struct {
			DayBefore  *bool `json:"day_before"`
			HourBefore *bool `json:"hour_before"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.DayBefore == nil || input.HourBefore == nil {
			failure(w, models.Fail("invalid_reminders", "Choose both reminder settings"))
			return
		}
		out, err = s.managers.Reminders.Set(r.Context(), session.User, id, models.RideReminders{DayBefore: *input.DayBefore, HourBefore: *input.HourBefore})
	} else {
		out, err = s.managers.Reminders.Get(r.Context(), session.User, id)
	}
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
