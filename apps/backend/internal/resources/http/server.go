package httpapi

import (
	"bikers.kz/backend/internal/managers/accounts"
	"bikers.kz/backend/internal/managers/clubs"
	"bikers.kz/backend/internal/managers/garage"
	"bikers.kz/backend/internal/managers/geo"
	"bikers.kz/backend/internal/managers/media"
	"bikers.kz/backend/internal/managers/notifications"
	"bikers.kz/backend/internal/managers/places"
	"bikers.kz/backend/internal/managers/reminders"
	"bikers.kz/backend/internal/managers/rides"
	"bikers.kz/backend/internal/managers/rooms"
	"bikers.kz/backend/internal/managers/routes"
	"bikers.kz/backend/internal/managers/system"
	"bikers.kz/backend/internal/managers/trust"
	"bikers.kz/backend/internal/models"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Settings struct {
	Origin         *url.URL
	TrustedProxies []netip.Prefix
	WebDistDir     string
}
type Managers struct {
	Accounts      *accounts.Manager
	Rides         *rides.Manager
	Rooms         *rooms.Manager
	System        *system.Manager
	Events        *rooms.EventBus
	Garage        *garage.Manager
	Geo           *geo.Manager
	Clubs         *clubs.Manager
	Media         *media.Manager
	Notifications *notifications.Manager
	Reminders     *reminders.Manager
	Trust         *trust.Manager
	Places        *places.Manager
	Routes        *routes.Manager
}
type Server struct {
	settings       Settings
	managers       Managers
	origin         *url.URL
	trustedProxies []netip.Prefix
	handler        http.Handler
}

func New(settings Settings, managers Managers) *Server {
	s := &Server{settings: settings, managers: managers, origin: settings.Origin, trustedProxies: settings.TrustedProxies}
	m := http.NewServeMux()
	m.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /health/ready", s.ready)
	m.HandleFunc("GET /api/v1/cities", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"cities": models.Cities()})
	})
	m.HandleFunc("GET /api/v1/session", s.getSession)
	m.HandleFunc("GET /api/v1/geo/config", s.geoConfig)
	m.HandleFunc("POST /api/v1/geo/search", s.geoSearch)
	m.HandleFunc("POST /api/v1/geo/reverse", s.geoReverse)
	m.HandleFunc("POST /api/v1/geo/route", s.geoRoute)
	m.HandleFunc("GET /api/v1/places/config", s.placeConfig)
	m.HandleFunc("GET /api/v1/places", s.listPlaces)
	m.HandleFunc("GET /api/v1/places/{placeID}", s.getPlace)
	m.HandleFunc("POST /api/v1/places/submissions", s.submitPlace)
	m.HandleFunc("GET /api/v1/me/place-submissions", s.ownPlaceSubmissions)
	m.HandleFunc("GET /api/v1/admin/place-submissions", s.adminPlaceSubmissions)
	m.HandleFunc("POST /api/v1/admin/place-submissions/{submissionID}/review", s.reviewPlaceSubmission)
	s.communityRouteEndpoints(m)
	m.HandleFunc("DELETE /api/v1/session", s.logout)
	m.HandleFunc("POST /api/v1/auth/challenges", s.challenge)
	m.HandleFunc("POST /api/v1/auth/challenges/verify", s.verify)
	m.HandleFunc("GET /api/v1/me/profile", s.getProfile)
	m.HandleFunc("PATCH /api/v1/me/profile", s.patchProfile)

	m.HandleFunc("GET /api/v1/me/preferences", s.preferences)
	m.HandleFunc("PATCH /api/v1/me/preferences", s.preferences)
	m.HandleFunc("GET /api/v1/me/notifications", s.notifications)
	m.HandleFunc("GET /api/v1/me/notifications/unread-count", s.unreadNotificationCount)
	m.HandleFunc("PUT /api/v1/me/notifications/read", s.readAllNotifications)
	m.HandleFunc("PUT /api/v1/me/notifications/{notificationID}/read", s.readNotification)
	m.HandleFunc("GET /api/v1/me/push/settings", s.pushSettings)
	m.HandleFunc("GET /api/v1/me/push/subscriptions", s.pushSubscriptions)
	m.HandleFunc("POST /api/v1/me/push/subscriptions", s.pushSubscriptions)
	m.HandleFunc("DELETE /api/v1/me/push/subscriptions/{subscriptionID}", s.deletePushSubscription)
	m.HandleFunc("GET /api/v1/me/avatar", s.meAvatar)
	m.HandleFunc("POST /api/v1/me/avatar", s.meAvatar)
	m.HandleFunc("DELETE /api/v1/me/avatar", s.meAvatar)
	m.HandleFunc("GET /api/v1/me/avatar/{imageID}/content", s.avatarContent)
	m.HandleFunc("GET /api/v1/rides/{scopeID}/avatars/{userID}", s.contextAvatar)
	m.HandleFunc("GET /api/v1/rides/{scopeID}/avatars/{userID}/{imageID}/content", s.contextAvatarContent)
	m.HandleFunc("GET /api/v1/clubs/{scopeID}/avatars/{userID}", func(w http.ResponseWriter, r *http.Request) { r.SetPathValue("scope", "clubs"); s.contextAvatar(w, r) })
	m.HandleFunc("GET /api/v1/clubs/{scopeID}/avatars/{userID}/{imageID}/content", func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("scope", "clubs")
		s.contextAvatarContent(w, r)
	})
	m.HandleFunc("GET /api/v1/rides/{rideID}/album", s.rideAlbum)
	m.HandleFunc("POST /api/v1/rides/{rideID}/album/photos", s.uploadRidePhoto)
	m.HandleFunc("DELETE /api/v1/rides/{rideID}/album/photos/{photoID}", s.deleteRidePhoto)
	m.HandleFunc("GET /api/v1/rides/{rideID}/album/photos/{photoID}/content", s.ridePhotoContent)
	m.HandleFunc("POST /api/v1/rides/{rideID}/album/videos", s.addRideVideo)
	m.HandleFunc("DELETE /api/v1/rides/{rideID}/album/videos/{videoID}", s.deleteRideVideo)
	m.HandleFunc("GET /api/v1/rides/{id}/announcements", s.announcements)
	m.HandleFunc("POST /api/v1/rides/{id}/announcements", s.announcements)
	m.HandleFunc("GET /api/v1/me/trust", s.meTrust)
	m.HandleFunc("GET /api/v1/me/blocks", s.listBlocks)
	m.HandleFunc("PUT /api/v1/me/blocks/{userID}", s.setBlock)
	m.HandleFunc("DELETE /api/v1/me/blocks/{userID}", s.removeBlock)
	m.HandleFunc("POST /api/v1/reports", s.reportContent)
	m.HandleFunc("GET /api/v1/users/{userID}/badge", s.userBadge)
	m.HandleFunc("GET /api/v1/admin/reports", s.adminReports)
	m.HandleFunc("GET /api/v1/admin/reports/{reportID}", s.adminReport)
	m.HandleFunc("GET /api/v1/admin/reports/{reportID}/content", s.adminReportContent)
	m.HandleFunc("PATCH /api/v1/admin/reports/{reportID}", s.adminUpdateReport)
	m.HandleFunc("POST /api/v1/admin/actions", s.adminAction)
	m.HandleFunc("GET /api/v1/admin/audit", s.adminAudit)

	m.HandleFunc("GET /api/v1/rides", s.listRides)
	m.HandleFunc("POST /api/v1/rides", s.createRide)
	m.HandleFunc("GET /api/v1/rides/{id}", s.getRide)
	m.HandleFunc("GET /api/v1/rides/{id}/calendar", s.rideCalendar)
	m.HandleFunc("GET /api/v1/rides/{id}/reminders", s.rideReminders)
	m.HandleFunc("PUT /api/v1/rides/{id}/reminders", s.rideReminders)
	m.HandleFunc("PATCH /api/v1/rides/{id}", s.patchRide)
	m.HandleFunc("POST /api/v1/rides/{id}/cancel", s.cancelRide)
	m.HandleFunc("PUT /api/v1/rides/{id}/participation", s.joinRide)
	m.HandleFunc("DELETE /api/v1/rides/{id}/participation", s.leaveRide)
	m.HandleFunc("GET /api/v1/rides/{id}/participants", s.participants)
	m.HandleFunc("POST /api/v1/rides/{id}/participants/{userID}/decision", s.decide)
	m.HandleFunc("GET /api/v1/rides/{id}/messages", s.getMessages)
	m.HandleFunc("POST /api/v1/rides/{id}/messages", s.postMessage)
	m.HandleFunc("GET /api/v1/events", s.events)
	m.HandleFunc("GET /api/v1/bike-models", s.searchBikeModels)
	m.HandleFunc("GET /api/v1/bikes", s.publicBikes)
	m.HandleFunc("GET /api/v1/me/bikes", s.listBikes)
	m.HandleFunc("POST /api/v1/me/bikes", s.createBike)
	m.HandleFunc("GET /api/v1/bikes/{id}", s.getBike)
	m.HandleFunc("PATCH /api/v1/me/bikes/{id}", s.patchBike)
	m.HandleFunc("DELETE /api/v1/me/bikes/{id}", s.deleteBike)
	m.HandleFunc("GET /api/v1/bikes/{bikeID}/photos", s.bikePhotos)
	m.HandleFunc("POST /api/v1/me/bikes/{bikeID}/photos", s.uploadBikePhoto)
	m.HandleFunc("PUT /api/v1/me/bikes/{bikeID}/photo-cover", s.setBikePhotoCover)
	m.HandleFunc("DELETE /api/v1/me/bikes/{bikeID}/photos/{photoID}", s.deleteBikePhoto)
	m.HandleFunc("GET /api/v1/bikes/{bikeID}/photos/{photoID}/content", s.bikePhotoContent)
	m.HandleFunc("GET /api/v1/me/bikes/{bikeID}/maintenance", s.listMaintenance)
	m.HandleFunc("POST /api/v1/me/bikes/{bikeID}/maintenance", s.createMaintenance)
	m.HandleFunc("PATCH /api/v1/me/bikes/{bikeID}/maintenance/{entryID}", s.patchMaintenance)
	m.HandleFunc("DELETE /api/v1/me/bikes/{bikeID}/maintenance/{entryID}", s.deleteMaintenance)
	m.HandleFunc("GET /api/v1/clubs", s.listClubs)
	m.HandleFunc("POST /api/v1/clubs", s.createClub)
	m.HandleFunc("GET /api/v1/clubs/{id}", s.getClub)
	m.HandleFunc("PATCH /api/v1/clubs/{id}", s.patchClub)
	m.HandleFunc("GET /api/v1/clubs/{clubID}/image", s.clubImage)
	m.HandleFunc("POST /api/v1/clubs/{clubID}/image", s.uploadClubImage)
	m.HandleFunc("DELETE /api/v1/clubs/{clubID}/image", s.removeClubImage)
	m.HandleFunc("GET /api/v1/clubs/{clubID}/image/{imageID}/content", s.clubImageContent)
	m.HandleFunc("POST /api/v1/clubs/{id}/close", s.closeClub)
	m.HandleFunc("PUT /api/v1/clubs/{id}/ownership", s.transferClub)
	m.HandleFunc("PUT /api/v1/clubs/{id}/membership", s.joinClub)
	m.HandleFunc("DELETE /api/v1/clubs/{id}/membership", s.leaveClub)
	m.HandleFunc("GET /api/v1/clubs/{id}/members", s.clubMembers)
	m.HandleFunc("POST /api/v1/clubs/{id}/members/{userID}/decision", s.decideClubMember)
	m.HandleFunc("GET /api/v1/clubs/{id}/rides", s.clubRides)
	m.HandleFunc("PUT /api/v1/clubs/{id}/rides/{rideID}", s.linkClubRide)
	m.HandleFunc("DELETE /api/v1/clubs/{id}/rides/{rideID}", s.unlinkClubRide)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { problem(w, 404, "not_found", "API endpoint not found") })
	m.HandleFunc("/", s.static)
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			if r.URL.Path != "/api/v1/events" {
				timeout := 20 * time.Second
				if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/photos") || strings.HasSuffix(r.URL.Path, "/image") || strings.HasSuffix(r.URL.Path, "/avatar")) {
					timeout = 45 * time.Second
				}
				ctx, cancel := context.WithTimeout(r.Context(), timeout)
				defer cancel()
				r = r.WithContext(ctx)
			}
		}
		if !strings.HasPrefix(r.URL.Path, "/health/") && !strings.EqualFold(r.Host, s.origin.Host) {
			problem(w, 400, "invalid_host", "Unexpected host")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && !s.sameOrigin(r) {
			problem(w, 403, "invalid_origin", "Request origin not allowed")
			return
		}
		m.ServeHTTP(w, r)
	})
	return s
}
func (s *Server) sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o != "" {
		return o == s.origin.String()
	}
	// Non-browser clients can omit Origin, but browsers carrying cookies must pass CSRF.
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	return decodeBounded(w, r, out, 64<<10)
}
func decodeBounded(w http.ResponseWriter, r *http.Request, out any, limit int64) bool {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		problem(w, 415, "json_required", "Use application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		var oversized *http.MaxBytesError
		if errors.As(e, &oversized) {
			problem(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request exceeds the upload limit")
			return false
		}
		problem(w, 400, "invalid_json", "Invalid JSON request")
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		problem(w, 400, "invalid_json", "Expected one JSON object")
		return false
	}
	return true
}
func idParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id, e := uuid.Parse(r.PathValue(name))
	if e != nil {
		problem(w, 404, "not_found", "Record not found")
		return "", false
	}
	return id.String(), true
}
func internal(w http.ResponseWriter) {
	problem(w, 503, "service_unavailable", "Service temporarily unavailable")
}

func (s *Server) Handler() http.Handler { return s.handler }
func failure(w http.ResponseWriter, err error) {
	var fault *models.Fault
	if !errors.As(err, &fault) {
		internal(w)
		return
	}
	status := http.StatusServiceUnavailable
	switch fault.Code {
	case "invalid_code", "authentication_required":
		status = 401
	case "invalid_host", "invalid_json":
		status = 400
	case "invalid_origin", "invalid_csrf", "forbidden", "participant_required", "account_unavailable", "club_member_required", "admin_required", "account_restricted":
		status = 403
	case "not_found", "report_content_unavailable":
		status = 404
	case "route_limited", "track_limited", "collection_limited":
		status = 409
	case "profile_required", "ride_cancelled", "ride_unavailable", "stale_plan", "capacity_too_small", "organizer_cannot_leave", "participation_changed", "ride_full", "message_id_conflict", "stale_version", "club_closed", "owner_transfer_required", "membership_changed", "invalid_owner", "photo_limit", "photo_storage_limit", "photo_upload_expired", "club_image_busy", "avatar_upload_busy", "album_photo_limit", "album_video_limit", "block_limit", "action_id_conflict", "announcement_id_conflict", "user_blocked", "push_subscription_limit", "push_subscription_in_use":
		status = 409
	case "invalid_phone", "invalid_profile", "invalid_ride", "invalid_decision", "invalid_message", "invalid_cursor", "invalid_bike", "invalid_maintenance", "invalid_query", "invalid_club", "invalid_photo", "invalid_photo_choice", "invalid_video_link", "invalid_report", "invalid_block", "invalid_admin_action", "invalid_announcement", "invalid_reminders", "invalid_preferences", "invalid_push_subscription", "invalid_map", "route_not_found", "invalid_place", "invalid_submission", "invalid_route", "invalid_track", "invalid_collection", "invalid_condition", "publication_consent_required":
		status = 422
	case "photo_too_large":
		status = 413
	case "unsupported_photo":
		status = 415
	case "send_limited", "photo_upload_busy", "photo_rate_limited", "report_rate_limited", "maps_busy", "submission_limited", "condition_limited":
		status = 429
		w.Header().Set("Retry-After", "60")
	case "delivery_unavailable", "verification_unavailable":
		status = 502
	}
	problem(w, status, fault.Code, fault.Message)
}
func (s *Server) cookieName() string {
	if s.origin.Scheme == "https" {
		return "__Host-bikerskz_session"
	}
	return "bikerskz_session"
}
func (s *Server) readSession(ctx context.Context, r *http.Request, touch bool) (*models.Session, error) {
	cookie, e := r.Cookie(s.cookieName())
	if e != nil {
		return nil, nil
	}
	return s.managers.Accounts.Session(ctx, cookie.Value, touch)
}
func (s *Server) require(w http.ResponseWriter, r *http.Request, write bool) *models.Session {
	v, e := s.readSession(r.Context(), r, true)
	if e != nil {
		failure(w, e)
		return nil
	}
	if v == nil {
		problem(w, 401, "authentication_required", "Sign in to continue")
		return nil
	}
	if write && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(v.CSRF)) != 1 {
		problem(w, 403, "invalid_csrf", "Refresh your session and retry")
		return nil
	}
	return v
}
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	v, e := s.readSession(r.Context(), r, true)
	if e != nil {
		failure(w, e)
		return
	}
	if v == nil {
		respond(w, 200, map[string]any{"user": nil})
		return
	}
	respond(w, 200, map[string]any{"user": v.User, "csrf_token": v.CSRF})
}
func (s *Server) challenge(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone string `json:"phone"`
	}
	if !decode(w, r, &input) {
		return
	}
	result, e := s.managers.Accounts.Challenge(r.Context(), input.Phone, s.sourceKey(r))
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 201, map[string]any{"challenge_id": result.ID, "expires_at": result.ExpiresAt, "resend_after": result.ResendAfter})
}
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	var input models.VerificationInput
	if !decode(w, r, &input) {
		return
	}
	oldToken := ""
	if c, e := r.Cookie(s.cookieName()); e == nil {
		oldToken = c.Value
	}
	result, e := s.managers.Accounts.Verify(r.Context(), input, oldToken)
	if e != nil {
		failure(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: result.Token, Path: "/", HttpOnly: true, Secure: s.origin.Scheme == "https", SameSite: http.SameSiteLaxMode, Expires: result.ExpiresAt, MaxAge: int(result.ExpiresAt.Sub(result.CreatedAt).Seconds())})
	respond(w, 200, map[string]any{"user": result.User, "csrf_token": result.CSRF})
}
func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, false)
	if v == nil {
		return
	}
	respond(w, 200, map[string]any{"user": v.User})
}
func (s *Server) patchProfile(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	var input models.ProfilePatch
	if !decode(w, r, &input) {
		return
	}
	user, e := s.managers.Accounts.UpdateProfile(r.Context(), v.User, input)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]any{"user": user})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	if e := s.managers.Accounts.Logout(r.Context(), *v); e != nil {
		failure(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: "", Path: "/", HttpOnly: true, Secure: s.origin.Scheme == "https", SameSite: http.SameSiteLaxMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if e := s.managers.System.Ready(r.Context()); e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]string{"status": "ready"})
}
func (s *Server) listRides(w http.ResponseWriter, r *http.Request) {
	v, e := s.readSession(r.Context(), r, true)
	if e != nil {
		failure(w, e)
		return
	}
	query := r.URL.Query()
	result, e := s.managers.Rides.List(r.Context(), v, models.RideListInput{
		View: query.Get("view"), City: query.Get("city"), From: query.Get("from"),
		To: query.Get("to"), Search: query.Get("q"), Cursor: query.Get("cursor"),
	})
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, result)
}
func (s *Server) getRide(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	v, e := s.readSession(r.Context(), r, true)
	if e != nil {
		failure(w, e)
		return
	}
	ride, e := s.managers.Rides.View(r.Context(), id, v)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]any{"ride": ride})
}
func (s *Server) createRide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	var input models.RideInput
	if !decode(w, r, &input) {
		return
	}
	ride, e := s.managers.Rides.Create(r.Context(), v.User, input)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 201, map[string]any{"ride": ride})
}
func (s *Server) patchRide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input models.RidePatch
	if !decode(w, r, &input) {
		return
	}
	ride, e := s.managers.Rides.Update(r.Context(), v.User, id, input)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]any{"ride": ride})
}
func (s *Server) cancelRide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		PlanVersion int64 `json:"plan_version"`
	}
	if !decode(w, r, &input) {
		return
	}
	ride, e := s.managers.Rides.Cancel(r.Context(), v.User, id, input.PlanVersion)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]any{"ride": ride})
}
func (s *Server) joinRide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	state, e := s.managers.Rides.Join(r.Context(), v.User, id)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) leaveRide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	state, e := s.managers.Rides.Leave(r.Context(), v.User, id)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, true)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	target, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var input struct {
		Decision string `json:"decision"`
	}
	if !decode(w, r, &input) {
		return
	}
	state, e := s.managers.Rides.Decide(r.Context(), v.User, id, target, input.Decision)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]string{"state": state})
}
func (s *Server) participants(w http.ResponseWriter, r *http.Request) {
	v := s.require(w, r, false)
	if v == nil {
		return
	}
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	people, e := s.managers.Rides.Participants(r.Context(), v.User, id)
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, 200, map[string]any{"participants": people})
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		problem(w, 405, "method_not_allowed", "Method not allowed")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/health") {
		problem(w, 404, "not_found", "Endpoint not found")
		return
	}
	previewID := strings.TrimPrefix(r.URL.Path, "/rides/")
	if _, e := uuid.Parse(previewID); strings.HasPrefix(r.URL.Path, "/rides/") && e == nil && !strings.Contains(previewID, "/") {
		s.publicHTML(w, r)
		return
	}
	for _, prefix := range []string{"/clubs/", "/bikes/"} {
		id := strings.TrimPrefix(r.URL.Path, prefix)
		if _, err := uuid.Parse(id); strings.HasPrefix(r.URL.Path, prefix) && err == nil && !strings.Contains(id, "/") {
			if s.publicEntityHTML(w, r, prefix, id) {
				return
			}
		}
	}
	rel := filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/"))
	if rel != "" && !filepath.IsLocal(rel) {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.settings.WebDistDir, rel)
	if st, e := os.Stat(p); e == nil && !st.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	if filepath.Ext(rel) != "" || strings.HasPrefix(r.URL.Path, "/assets/") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(s.settings.WebDistDir, "index.html")
	if _, e := os.Stat(index); e != nil {
		http.Error(w, "Web build unavailable", 503)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}
func (s *Server) sourceKey(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		host = r.RemoteAddr
	}
	peer, e := netip.ParseAddr(host)
	if e == nil && s.trustedProxy(peer) {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		// Walk the chain from the actual trusted peer; the edge appends the real
		// client address, so attacker-supplied earlier elements cannot win.
		for i := len(parts) - 1; i >= 0; i-- {
			candidate, e := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if e != nil {
				break
			}
			host = candidate.String()
			if !s.trustedProxy(candidate) {
				break
			}
		}
	}
	d := sha256.Sum256([]byte(host))
	return hex.EncodeToString(d[:])
}
func (s *Server) trustedProxy(addr netip.Addr) bool {
	for _, p := range s.trustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
