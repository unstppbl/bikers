package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"bikers.kz/backend/internal/managers/polls"
	"bikers.kz/backend/internal/models"
)

const ridePollPath = "/api/v1/ride-polls"
const ridePollVoterCookie = "bikerskz_poll_voter"

func (s *Server) ridePollEndpoints(m *http.ServeMux) {
	m.HandleFunc("POST "+ridePollPath, s.createRidePoll)
	m.HandleFunc("GET "+ridePollPath+"/{id}", s.getRidePoll)
	m.HandleFunc("PUT "+ridePollPath+"/{id}/vote", s.voteRidePoll)
	m.HandleFunc("PUT "+ridePollPath+"/{id}/selection", s.selectRidePoll)
}

func (s *Server) createRidePoll(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var input models.RidePollInput
	if !decode(w, r, &input) {
		return
	}
	poll, err := s.managers.Polls.Create(r.Context(), actor.User, input)
	ridePollReply(w, http.StatusCreated, poll, err)
}

func (s *Server) getRidePoll(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	actor, ok := s.pollViewer(w, r)
	if !ok {
		return
	}
	token := readRidePollToken(r)
	poll, err := s.managers.Polls.View(r.Context(), actor, id, token)
	if err == nil && token == nil && poll.SelectedOptionID == "" && r.Method == http.MethodGet {
		var ok bool
		token, ok = newRidePollToken(w)
		if !ok {
			return
		}
		s.setRidePollToken(w, id, token)
	}
	ridePollReply(w, http.StatusOK, poll, err)
}

func (s *Server) voteRidePoll(w http.ResponseWriter, r *http.Request) {
	// Guests have no session CSRF token. Require the exact browser origin even
	// though the shared origin guard allows non-browser writes without Origin.
	if r.Header.Get("Origin") != s.origin.String() {
		problem(w, http.StatusForbidden, "invalid_origin", "Request origin not allowed")
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		OptionIDs []string `json:"option_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	actor, ok := s.pollViewer(w, r)
	if !ok {
		return
	}
	token := readRidePollToken(r)
	newToken := token == nil
	if newToken {
		token, ok = newRidePollToken(w)
		if !ok {
			return
		}
	}
	poll, err := s.managers.Polls.Vote(r.Context(), actor, id, token, input.OptionIDs)
	if err == nil && newToken {
		s.setRidePollToken(w, id, token)
	}
	ridePollReply(w, http.StatusOK, poll, err)
}

func (s *Server) selectRidePoll(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, true)
	if actor == nil {
		return
	}
	var input struct {
		OptionID string `json:"option_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	poll, err := s.managers.Polls.Select(r.Context(), actor.User, r.PathValue("id"), readRidePollToken(r), input.OptionID)
	ridePollReply(w, http.StatusOK, poll, err)
}

func (s *Server) pollViewer(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	session, err := s.readSession(r.Context(), r, false)
	if err != nil {
		failure(w, err)
		return models.User{}, false
	}
	if session == nil {
		return models.User{}, true
	}
	return session.User, true
}

func ridePollReply(w http.ResponseWriter, status int, poll models.RidePoll, err error) {
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, status, map[string]models.RidePoll{"poll": poll})
}

func readRidePollToken(r *http.Request) []byte {
	cookie, err := r.Cookie(ridePollVoterCookie)
	if err != nil || len(cookie.Value) != base64.RawURLEncoding.EncodedLen(polls.VoterTokenBytes) {
		return nil
	}
	token, err := base64.RawURLEncoding.Strict().DecodeString(cookie.Value)
	if err != nil || len(token) != polls.VoterTokenBytes {
		return nil
	}
	return token
}

func newRidePollToken(w http.ResponseWriter) ([]byte, bool) {
	token := make([]byte, polls.VoterTokenBytes)
	if _, err := rand.Read(token); err != nil {
		internal(w)
		return nil, false
	}
	return token, true
}

func (s *Server) setRidePollToken(w http.ResponseWriter, id string, token []byte) {
	http.SetCookie(w, &http.Cookie{
		Name: ridePollVoterCookie, Value: base64.RawURLEncoding.EncodeToString(token), Path: ridePollPath + "/" + id,
		HttpOnly: true, Secure: s.origin.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: 365 * 24 * 60 * 60,
	})
}
