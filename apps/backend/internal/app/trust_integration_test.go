package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/managers/trust"
	"github.com/google/uuid"
)

func grantTestAdmin(t *testing.T, f *fixture, a *actor) {
	t.Helper()
	m := trust.New(postgres.New(f.db), nil, func() time.Time { return f.now })
	if err := m.Role(context.Background(), a.id, true, "Explicit isolated test administrator"); err != nil {
		t.Fatal(err)
	}
}
func trustAction(f *fixture, a *actor, action, kind, id, why, clientID string) response {
	if clientID == "" {
		clientID = uuid.NewString()
	}
	return f.request("POST", "/api/v1/admin/actions", map[string]any{"client_action_id": clientID, "action": action, "target_type": kind, "target_id": id, "reason": why}, a)
}
func trustReport(f *fixture, a *actor, kind, id string) response {
	return f.request("POST", "/api/v1/reports", map[string]any{"target_type": kind, "target_id": id, "reason": "harassment", "details": "Reported from an authorized view"}, a)
}

func trustRawImage(t *testing.T, f *fixture, path string, a *actor) response {
	t.Helper()
	req, err := http.NewRequest("POST", f.ts.URL+path, bytes.NewReader(photoPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Origin", f.ts.URL)
	req.Header.Set("X-CSRF-Token", a.csrf)
	req.AddCookie(a.cookie)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := response{status: r.StatusCode, raw: string(data)}
	if err = json.Unmarshal(data, &out.body); err != nil {
		t.Fatal(err)
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("image API cached")
	}
	return out
}

func TestTrustAdminBadgeAuditRestrictionAndRoleRevocation(t *testing.T) {
	f := newFixture(t)
	admin := f.login("+77011234567", "Administrator")
	rider := f.login("+77021234567", "Rider")
	expect(t, f.request("GET", "/api/v1/admin/reports", nil, admin), 403)
	status := f.request("GET", "/api/v1/me/trust", nil, admin)
	expect(t, status, 200)
	if status.body["is_admin"] != false {
		t.Fatal("signup unexpectedly granted administrator role")
	}
	badge := f.request("GET", "/api/v1/users/"+rider.id+"/badge", nil, nil)
	expect(t, badge, 200)
	if badge.body["badge"] != nil {
		t.Fatal("phone verification awarded a badge")
	}
	grantTestAdmin(t, f, admin)
	status = f.request("GET", "/api/v1/me/trust", nil, admin)
	expect(t, status, 200)
	if status.body["is_admin"] != true {
		t.Fatal("persisted CLI grant not reflected")
	}
	missingCSRF := *admin
	missingCSRF.csrf = ""
	expect(t, trustAction(f, &missingCSRF, "assign_badge", "user", rider.id, "Reviewed community participation", ""), 403)
	expect(t, trustAction(f, admin, "assign_badge", "user", rider.id, " ", ""), 422)
	client := uuid.NewString()
	assigned := trustAction(f, admin, "assign_badge", "user", rider.id, "Reviewed community participation", client)
	expect(t, assigned, 200)
	retry := trustAction(f, admin, "assign_badge", "user", rider.id, "Reviewed community participation", client)
	expect(t, retry, 200)
	if retry.body["action"].(map[string]any)["id"] != assigned.body["action"].(map[string]any)["id"] {
		t.Fatal("idempotent action duplicated audit")
	}
	expect(t, trustAction(f, admin, "assign_badge", "user", rider.id, "Changed reason", client), 409)
	badge = f.request("GET", "/api/v1/users/"+rider.id+"/badge", nil, nil)
	expect(t, badge, 200)
	safe := badge.body["badge"].(map[string]any)
	if len(safe) != 3 || safe["label"] != "Community verified" || !strings.Contains(safe["disclaimer"].(string), "not identity, licence, or safety verification") {
		t.Fatalf("unsafe badge projection: %s", badge.raw)
	}
	if strings.Contains(badge.raw, "Reviewed community") || strings.Contains(badge.raw, admin.id) {
		t.Fatal("badge exposed private review information")
	}
	request := trustReport(f, rider, "user", admin.id)
	expect(t, request, 201)
	receipt := request.body["report"].(map[string]any)
	if len(receipt) != 3 {
		t.Fatal("report submission exposed review internals")
	}
	duplicate := trustReport(f, rider, "user", admin.id)
	expect(t, duplicate, 201)
	if duplicate.body["report"].(map[string]any)["id"] != receipt["id"] {
		t.Fatal("open report was duplicated")
	}
	view := f.request("GET", "/api/v1/admin/reports/"+receipt["id"].(string), nil, admin)
	expect(t, view, 200)
	if strings.Contains(view.raw, "phone") || strings.Contains(view.raw, "csrf") || strings.Contains(view.raw, "token") {
		t.Fatal("review leaked authentication data")
	}
	expect(t, trustAction(f, admin, "restrict_account", "user", admin.id, "Do not lock out this operator", ""), 422)
	expect(t, trustAction(f, admin, "restrict_account", "user", rider.id, "Abusive account activity", ""), 200)
	session := f.request("GET", "/api/v1/session", nil, rider)
	expect(t, session, 200)
	if session.body["user"] != nil {
		t.Fatal("restricted account kept its session")
	}
	var state string
	var liveSessions, auditCount int
	if err := f.db.QueryRow(context.Background(), `SELECT status,(SELECT count(*) FROM sessions WHERE user_id=$1 AND revoked_at IS NULL),(SELECT count(*) FROM trust_audit WHERE action='restrict_account' AND target_id=$1) FROM users WHERE id=$1`, rider.id).Scan(&state, &liveSessions, &auditCount); err != nil {
		t.Fatal(err)
	}
	if state != "suspended" || liveSessions != 0 || auditCount != 1 {
		t.Fatalf("restriction/audit/session atomicity: %s %d %d", state, liveSessions, auditCount)
	}
	expect(t, trustAction(f, admin, "restore_account", "user", rider.id, "Review completed", ""), 200)
	session = f.request("GET", "/api/v1/session", nil, rider)
	expect(t, session, 200)
	if session.body["user"] != nil {
		t.Fatal("restoration revived a revoked token")
	}
	if _, err := f.db.Exec(context.Background(), `UPDATE users SET status='deleted' WHERE id=$1`, rider.id); err != nil {
		t.Fatal(err)
	}
	expect(t, trustAction(f, admin, "restore_account", "user", rider.id, "Must not resurrect deleted account", ""), 422)
	m := trust.New(postgres.New(f.db), nil, func() time.Time { return f.now })
	if err := m.Role(context.Background(), admin.id, false, "Explicit isolated test revoke"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("GET", "/api/v1/admin/audit", nil, admin), 403)
	var roles int
	if err := f.db.QueryRow(context.Background(), `SELECT count(*) FROM trust_admins`).Scan(&roles); err != nil || roles != 0 {
		t.Fatalf("role revoke not persisted: %d %v", roles, err)
	}
}

func TestTrustPrivateReportImageReviewAndModerationRetainsOriginal(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Photo owner")
	other := f.login("+77021234567", "Other")
	admin := f.login("+77051234567", "Moderator")
	grantTestAdmin(t, f, admin)
	bike := testBike(t, f, owner)
	bikeID := bike["id"].(string)
	uploaded := photoUpload(t, f, bikeID, photoPNG(t), owner, true)
	expect(t, uploaded, 201)
	photo := uploaded.body["photo"].(map[string]any)
	photoID := photo["id"].(string)
	expect(t, trustReport(f, other, "bike", bikeID), 404)
	expect(t, trustReport(f, other, "bike_photo", photoID), 404)
	reported := trustReport(f, owner, "bike_photo", photoID)
	expect(t, reported, 201)
	reportID := reported.body["report"].(map[string]any)["id"].(string)
	expect(t, f.request("GET", "/api/v1/admin/reports/"+reportID, nil, other), 403)
	before := photoRead(t, f, photo["url"].(string), owner, "GET", 200)
	reviewed := photoRead(t, f, "/api/v1/admin/reports/"+reportID+"/content", admin, "GET", 200)
	if string(reviewed) != string(before) {
		t.Fatal("review did not return reported JPEG")
	}
	expect(t, trustAction(f, admin, "hide_content", "bike_photo", photoID, "Abusive photograph", ""), 200)
	photoRead(t, f, photo["url"].(string), owner, "GET", 404)
	reviewed = photoRead(t, f, "/api/v1/admin/reports/"+reportID+"/content", admin, "HEAD", 200)
	if len(reviewed) != 0 {
		t.Fatal("HEAD sent image body")
	}
	var length int
	if err := f.db.QueryRow(context.Background(), `SELECT octet_length(content) FROM bike_photos WHERE id=$1`, photoID).Scan(&length); err != nil || length != len(before) {
		t.Fatalf("moderation deleted original: %d %v", length, err)
	}
	patch := map[string]any{"version": 1, "status": "reviewing", "reason": "Review begun"}
	expect(t, f.request("PATCH", "/api/v1/admin/reports/"+reportID, patch, admin), 200)
	expect(t, f.request("PATCH", "/api/v1/admin/reports/"+reportID, patch, admin), 409)
	expect(t, trustAction(f, admin, "restore_content", "bike_photo", photoID, "Content restored after review", ""), 200)
	photoRead(t, f, photo["url"].(string), owner, "GET", 200)
	gallery := photoGallery(t, f, bikeID, owner)
	expect(t, f.request("DELETE", "/api/v1/me/bikes/"+bikeID+"/photos/"+photoID, map[string]any{"version": gallery["version"]}, owner), 200)
	unavailable := f.request("GET", "/api/v1/admin/reports/"+reportID+"/content", nil, admin)
	expect(t, unavailable, 404)
	if unavailable.body["error"].(map[string]any)["code"] != "report_content_unavailable" {
		t.Fatal("removed evidence has no explicit unavailable state")
	}
	retained := f.request("GET", "/api/v1/admin/reports/"+reportID, nil, admin)
	expect(t, retained, 200)
	if retained.body["target"].(map[string]any)["id"] != photoID {
		t.Fatal("limited report snapshot disappeared with owner deletion")
	}
}

func TestTrustMutualBlocksFilterHistoryButKeepApprovedCoordination(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Organizer")
	one := f.login("+77021234567", "One")
	two := f.login("+77051234567", "Two")
	pending := f.login("+77071234567", "Pending")
	ride := f.ride(owner, 10)
	f.join(ride, one)
	f.join(ride, two)
	f.join(ride, pending)
	expect(t, f.decision(ride, owner, one, "approve"), 200)
	expect(t, f.decision(ride, owner, two, "approve"), 200)
	message := f.request("POST", "/api/v1/rides/"+ride+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": "Blocked author ordinary chat"}, two)
	expect(t, message, 201)
	announcement := f.request("POST", "/api/v1/rides/"+ride+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Important organizer coordination"}, owner)
	expect(t, announcement, 201)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+two.id, nil, one), 200)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+two.id, nil, one), 200)
	blocks := f.request("GET", "/api/v1/me/blocks", nil, one)
	expect(t, blocks, 200)
	if len(blocks.body["blocks"].([]any)) != 1 {
		t.Fatal("idempotent blocking duplicated own list")
	}
	otherList := f.request("GET", "/api/v1/me/blocks", nil, two)
	expect(t, otherList, 200)
	if len(otherList.body["blocks"].([]any)) != 0 {
		t.Fatal("another user's blocks leaked")
	}
	history := f.request("GET", "/api/v1/rides/"+ride+"/messages?after=0", nil, one)
	expect(t, history, 200)
	if strings.Contains(history.raw, "Blocked author ordinary chat") {
		t.Fatal("blocked author leaked through history")
	}
	after := int64(history.body["next_sequence"].(float64))
	later := f.request("POST", "/api/v1/rides/"+ride+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": "Visible message after filtered tail"}, one)
	expect(t, later, 201)
	catchup := f.request("GET", fmt.Sprintf("/api/v1/rides/%s/messages?after=%d", ride, after), nil, one)
	expect(t, catchup, 200)
	if !strings.Contains(catchup.raw, "Visible message after filtered tail") || strings.Contains(catchup.raw, "Blocked author ordinary chat") {
		t.Fatal("filtered history cursor lost later visible messages")
	}
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+owner.id, nil, one), 200)
	plan := f.request("GET", "/api/v1/rides/"+ride, nil, one)
	expect(t, plan, 200)
	if plan.body["ride"].(map[string]any)["meeting_point"] != "SECRET MEETING" {
		t.Fatal("block removed approved ride coordination")
	}
	expect(t, f.request("GET", "/api/v1/rides/"+ride+"/participants", nil, one), 200)
	announcements := f.request("GET", "/api/v1/rides/"+ride+"/announcements", nil, one)
	expect(t, announcements, 200)
	if !strings.Contains(announcements.raw, "Important organizer coordination") {
		t.Fatal("organizer announcement was incorrectly filtered")
	}
	announcementID := announcement.body["announcement"].(map[string]any)["id"].(string)
	expect(t, trustReport(f, one, "announcement", announcementID), 201)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+pending.id, nil, owner), 200)
	expect(t, f.decision(ride, owner, pending, "approve"), 409)
	expect(t, f.request("DELETE", "/api/v1/rides/"+ride+"/participation", nil, pending), 200)
	expect(t, f.request("PUT", "/api/v1/rides/"+ride+"/participation", nil, pending), 409)
	preview := f.request("GET", "/api/v1/rides/"+ride, nil, nil)
	expect(t, preview, 200)
	if len(preview.body["ride"].(map[string]any)) != 5 {
		t.Fatal("anonymous preview changed privacy projection")
	}
	expect(t, f.request("DELETE", "/api/v1/me/blocks/"+two.id, nil, one), 200)
	expect(t, f.request("DELETE", "/api/v1/me/blocks/"+two.id, nil, one), 200)
	history = f.request("GET", "/api/v1/rides/"+ride+"/messages?after=0", nil, one)
	expect(t, history, 200)
	if !strings.Contains(history.raw, "Blocked author ordinary chat") {
		t.Fatal("unblocking did not restore visible ordinary history")
	}
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+one.id, nil, one), 422)
}

func TestTrustReportLimitAndAuditCursorStayBoundedAcrossRestart(t *testing.T) {
	f := newFixture(t)
	admin := f.login("+77011234567", "Administrator")
	reporter := f.login("+77021234567", "Reporter")
	grantTestAdmin(t, f, admin)
	for i := 0; i < 11; i++ {
		id := uuid.NewString()
		if _, err := f.db.Exec(context.Background(), `INSERT INTO users(id,phone,status,display_name,created_at) VALUES($1,$2,'active',$3,$4)`, id, "test-report-"+id, fmt.Sprintf("Public report target %d", i), f.now); err != nil {
			t.Fatal(err)
		}
		r := trustReport(f, reporter, "user", id)
		if i < 10 {
			expect(t, r, 201)
		} else {
			expect(t, r, 429)
		}
	}
	f.startServer(f.gateway)
	r := trustReport(f, reporter, "user", admin.id)
	expect(t, r, 429)
	for i := 0; i < 52; i++ {
		expect(t, trustAction(f, admin, "assign_badge", "user", reporter.id, "Community review audit pagination", ""), 200)
	}
	first := f.request("GET", "/api/v1/admin/audit", nil, admin)
	expect(t, first, 200)
	rows := first.body["actions"].([]any)
	if len(rows) != 50 || first.body["next_cursor"] == "" {
		t.Fatal("audit page is not bounded")
	}
	second := f.request("GET", "/api/v1/admin/audit?after="+first.body["next_cursor"].(string), nil, admin)
	expect(t, second, 200)
	seen := map[string]bool{}
	for _, a := range rows {
		seen[a.(map[string]any)["id"].(string)] = true
	}
	for _, a := range second.body["actions"].([]any) {
		if seen[a.(map[string]any)["id"].(string)] {
			t.Fatal("audit cursor duplicated an entry")
		}
	}
	expect(t, f.request("GET", "/api/v1/admin/audit?after=not-a-cursor", nil, admin), 422)
}

func TestTrustAvatarReportsRequireCurrentApprovedContext(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Avatar owner")
	viewer := f.login("+77021234567", "Viewer")
	admin := f.login("+77051234567", "Moderator")
	grantTestAdmin(t, f, admin)
	upload := trustRawImage(t, f, "/api/v1/me/avatar?version=1", owner)
	expect(t, upload, 201)
	avatarID := upload.body["image"].(map[string]any)["id"].(string)
	expect(t, trustReport(f, viewer, "avatar", avatarID), 404)
	ride := f.ride(owner, 5)
	f.join(ride, viewer)
	expect(t, trustReport(f, viewer, "avatar", avatarID), 404)
	expect(t, f.decision(ride, owner, viewer, "approve"), 200)
	reported := trustReport(f, viewer, "avatar", avatarID)
	expect(t, reported, 201)
	id := reported.body["report"].(map[string]any)["id"].(string)
	photoRead(t, f, "/api/v1/admin/reports/"+id+"/content", admin, "GET", 200)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+owner.id, nil, viewer), 200)
	expect(t, trustReport(f, viewer, "avatar", avatarID), 404)
	expect(t, f.request("DELETE", "/api/v1/me/blocks/"+owner.id, nil, viewer), 200)
	expect(t, f.request("DELETE", "/api/v1/rides/"+ride+"/participation", nil, viewer), 200)
	expect(t, trustReport(f, viewer, "avatar", avatarID), 404)
	club := testClub(t, f, owner)
	clubID := club["id"].(string)
	expect(t, f.request("PUT", "/api/v1/clubs/"+clubID+"/membership", nil, viewer), 200)
	expect(t, clubDecision(f, clubID, owner, viewer, "approve"), 200)
	expect(t, trustReport(f, viewer, "avatar", avatarID), 201)
	expect(t, f.request("GET", "/api/v1/users/"+owner.id+"/avatar", nil, nil), 404)
}

func TestTrustHiddenUserSuppressesAvatarAndBadgeButKeepsApprovedCoordination(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234741", "Avatar owner")
	viewer := f.login("+77011234742", "Approved viewer")
	admin := f.login("+77011234743", "Moderator")
	grantTestAdmin(t, f, admin)
	upload := trustRawImage(t, f, "/api/v1/me/avatar?version=1", owner)
	expect(t, upload, 201)
	image := upload.body["image"].(map[string]any)
	avatarID := image["id"].(string)
	ride := f.ride(owner, 5)
	approvedRider(t, f, ride, owner, viewer)
	contextPath := "/api/v1/rides/" + ride + "/avatars/" + owner.id
	contextAvatar := f.request("GET", contextPath, nil, viewer)
	expect(t, contextAvatar, 200)
	contextURL := contextAvatar.body["image"].(map[string]any)["url"].(string)
	expect(t, trustAction(f, admin, "assign_badge", "user", owner.id, "Community contribution", ""), 200)
	expect(t, f.request("POST", "/api/v1/rides/"+ride+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Important existing coordination"}, owner), 201)
	expect(t, trustAction(f, admin, "hide_content", "user", owner.id, "Hide abusive profile content", ""), 200)
	photoRead(t, f, image["url"].(string), owner, "GET", 404)
	photoRead(t, f, contextURL, viewer, "GET", 404)
	contextAvatar = f.request("GET", contextPath, nil, viewer)
	expect(t, contextAvatar, 200)
	if contextAvatar.body["image"] != nil {
		t.Fatal("hidden user avatar metadata remained visible")
	}
	badge := f.request("GET", "/api/v1/users/"+owner.id+"/badge", nil, viewer)
	expect(t, badge, 200)
	if badge.body["badge"] != nil {
		t.Fatal("hidden user's community badge remained visible")
	}
	expect(t, trustReport(f, viewer, "avatar", avatarID), 404)
	plan := f.request("GET", "/api/v1/rides/"+ride, nil, viewer)
	expect(t, plan, 200)
	if !strings.Contains(plan.raw, "SECRET MEETING") {
		t.Fatal("profile hiding removed existing ride coordination")
	}
	announcements := f.request("GET", "/api/v1/rides/"+ride+"/announcements", nil, viewer)
	expect(t, announcements, 200)
	if !strings.Contains(announcements.raw, "Important existing coordination") {
		t.Fatal("profile hiding removed organizer announcements")
	}
	ownAvatar := f.request("GET", "/api/v1/me/avatar", nil, owner)
	expect(t, ownAvatar, 200)
	if ownAvatar.body["image"].(map[string]any)["id"] != avatarID {
		t.Fatal("hiding profile removed avatar deletion metadata")
	}
	expect(t, f.request("DELETE", "/api/v1/me/avatar", map[string]any{"version": ownAvatar.body["version"]}, owner), 200)
}

func TestTrustHiddenParentsDoNotAuthorizeGuessedChildReports(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234751", "Owner")
	member := f.login("+77011234752", "Approved member")
	outsider := f.login("+77011234753", "Outsider")
	admin := f.login("+77011234754", "Moderator")
	grantTestAdmin(t, f, admin)
	bike := testBike(t, f, owner)
	bikeID := bike["id"].(string)
	expect(t, f.request("PATCH", "/api/v1/me/bikes/"+bikeID, map[string]any{"version": 1, "visibility": "public"}, owner), 200)
	bikeImage := photoUpload(t, f, bikeID, photoPNG(t), owner, true)
	expect(t, bikeImage, 201)
	photoID := bikeImage.body["photo"].(map[string]any)["id"].(string)
	expect(t, trustAction(f, admin, "hide_content", "bike", bikeID, "Hide public bike content", ""), 200)
	expect(t, trustReport(f, outsider, "bike_photo", photoID), 404)
	expect(t, trustReport(f, owner, "bike_photo", photoID), 201)
	club := testClub(t, f, owner)
	clubID := club["id"].(string)
	expect(t, f.request("PUT", "/api/v1/clubs/"+clubID+"/membership", nil, member), 200)
	expect(t, clubDecision(f, clubID, owner, member, "approve"), 200)
	clubImage := clubImageUpload(t, f, clubID, "1", photoPNG(t), owner, true)
	expect(t, clubImage, 201)
	clubImageID := clubImage.body["image"].(map[string]any)["id"].(string)
	expect(t, trustAction(f, admin, "hide_content", "club", clubID, "Hide public club discovery", ""), 200)
	expect(t, trustReport(f, outsider, "club_image", clubImageID), 404)
	expect(t, trustReport(f, member, "club_image", clubImageID), 201)
	expect(t, f.request("GET", "/api/v1/clubs/"+clubID+"/image", nil, outsider), 404)
	approvedImage := f.request("GET", "/api/v1/clubs/"+clubID+"/image", nil, member)
	expect(t, approvedImage, 200)
	if approvedImage.body["image"] == nil {
		t.Fatal("hidden club lost existing approved-member image access")
	}
}
