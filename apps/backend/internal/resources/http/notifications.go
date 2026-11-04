package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
)

func (s *Server) preferences(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, r.Method != "GET")
	if session == nil {
		return
	}
	var out models.Preferences
	var err error
	if r.Method == "PATCH" {
		var input models.PreferencesPatch
		if !decode(w, r, &input) {
			return
		}
		out, err = s.managers.Notifications.UpdatePreferences(r.Context(), session.User, input)
	} else {
		out, err = s.managers.Notifications.Preferences(r.Context(), session.User)
	}
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, false)
	if session == nil {
		return
	}
	out, err := s.managers.Notifications.List(r.Context(), session.User, r.URL.Query().Get("before"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "notificationID")
	if !ok {
		return
	}
	out, err := s.managers.Notifications.Read(r.Context(), session.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"notification": out})
}
func (s *Server) readAllNotifications(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	if err := s.managers.Notifications.ReadAll(r.Context(), session.User); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) pushSettings(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, false) == nil {
		return
	}
	enabled, key := s.managers.Notifications.PushSettings()
	respond(w, 200, map[string]any{"enabled": enabled, "public_key": key})
}
func (s *Server) pushSubscriptions(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, r.Method != "GET")
	if session == nil {
		return
	}
	if r.Method == "GET" {
		out, err := s.managers.Notifications.Subscriptions(r.Context(), session.User)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, map[string]any{"subscriptions": out})
		return
	}
	var input models.PushSubscriptionInput
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Notifications.Subscribe(r.Context(), *session, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"subscription": out})
}
func (s *Server) deletePushSubscription(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, true)
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "subscriptionID")
	if !ok {
		return
	}
	if err := s.managers.Notifications.Unsubscribe(r.Context(), session.User, id); err != nil {
		failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) announcements(w http.ResponseWriter, r *http.Request) {
	session := s.require(w, r, r.Method != "GET")
	if session == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	if r.Method == "GET" {
		out, err := s.managers.Rides.Announcements(r.Context(), session.User, id, r.URL.Query().Get("before"))
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, out)
		return
	}
	var input models.AnnouncementInput
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Rides.Announce(r.Context(), session.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	status := 200
	if out.Created {
		status = 201
	}
	respond(w, status, out)
}
