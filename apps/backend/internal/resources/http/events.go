package httpapi

import (
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"net/http"
	"time"
)

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	if r.Header.Get("Origin") != s.origin.String() {
		problem(w, 403, "invalid_origin", "Request origin not allowed")
		return
	}
	id, e := uuid.Parse(r.URL.Query().Get("ride_id"))
	if e != nil {
		problem(w, 422, "invalid_ride", "A ride ID is required")
		return
	}
	ride := id.String()
	allowed, e := s.managers.Rooms.Access(r.Context(), ride, u.User.ID)
	if e != nil {
		internal(w)
		return
	}
	if !allowed {
		problem(w, 403, "participant_required", "Approved participation required")
		return
	}
	c, e := websocket.Accept(w, r, nil)
	if e != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1024)
	ctx := c.CloseRead(r.Context())
	sub := s.managers.Events.Subscribe(ride)
	defer s.managers.Events.Unsubscribe(sub)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		var event models.Event
		select {
		case <-ctx.Done():
			return
		case event = <-sub.Events:
		case <-ticker.C:
		}
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		session, e := s.readSession(checkCtx, r, false)
		valid := false
		if e == nil && session != nil {
			valid, e = s.managers.Rooms.Access(checkCtx, ride, session.User.ID)
		}
		cancel()
		if e != nil || !valid {
			_ = c.Close(websocket.StatusPolicyViolation, "Access expired")
			return
		}
		if event.Type != "" {
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			e = wsjson.Write(writeCtx, c, event)
			cancel()
			if e != nil {
				return
			}
		}
	}
}
