package httpapi

import (
	"bikers.kz/backend/internal/models"
	"net/http"
)

func (s *Server) meTrust(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	ok, err := s.managers.Trust.IsAdmin(r.Context(), u.User.ID)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"is_admin": ok})
}
func (s *Server) listBlocks(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	out, err := s.managers.Trust.Blocks(r.Context(), u.User, r.URL.Query().Get("after"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) setBlock(w http.ResponseWriter, r *http.Request)    { s.changeBlock(w, r, true) }
func (s *Server) removeBlock(w http.ResponseWriter, r *http.Request) { s.changeBlock(w, r, false) }
func (s *Server) changeBlock(w http.ResponseWriter, r *http.Request, on bool) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	id, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	blocked, err := s.managers.Trust.Block(r.Context(), u.User, id, on)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"blocked": blocked})
}
func (s *Server) userBadge(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	badge, err := s.managers.Trust.Badge(r.Context(), id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"badge": badge})
}
func (s *Server) reportContent(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	var input models.ReportInput
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Trust.Report(r.Context(), u.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 201, map[string]any{"report": out})
}
func (s *Server) adminReports(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	out, err := s.managers.Trust.Reports(r.Context(), u.User, r.URL.Query().Get("status"), r.URL.Query().Get("after"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
func (s *Server) adminReport(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	id, ok := idParam(w, r, "reportID")
	if !ok {
		return
	}
	report, target, err := s.managers.Trust.Review(r.Context(), u.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"report": report, "target": target})
}
func (s *Server) adminReportContent(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	id, ok := idParam(w, r, "reportID")
	if !ok {
		return
	}
	out, err := s.managers.Trust.ReviewContent(r.Context(), u.User, id)
	if err != nil {
		failure(w, err)
		return
	}
	writePhotoContent(w, r, out)
}
func (s *Server) adminUpdateReport(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	id, ok := idParam(w, r, "reportID")
	if !ok {
		return
	}
	var input models.ReportPatch
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Trust.UpdateReport(r.Context(), u.User, id, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"report": out})
}
func (s *Server) adminAction(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, true)
	if u == nil {
		return
	}
	var input models.TrustActionInput
	if !decode(w, r, &input) {
		return
	}
	out, err := s.managers.Trust.Action(r.Context(), u.User, input)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, map[string]any{"action": out})
}
func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	u := s.require(w, r, false)
	if u == nil {
		return
	}
	out, err := s.managers.Trust.Audit(r.Context(), u.User, r.URL.Query().Get("after"))
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, 200, out)
}
