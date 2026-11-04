package app

import (
	"bikers.kz/backend/internal/clients/telegram"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramDigitOnlyPhoneResponsesKeepChallengeBinding(t *testing.T) {
	for _, tc := range []struct {
		name, sendPhone, verifyPhone string
		sendStatus, verifyStatus     int
	}{
		{"international_plus", "+77011234567", "+77011234567", 201, 200},
		{"digit_only_send_and_verify", "77011234567", "77011234567", 201, 200},
		{"digit_only_verify", "+77011234567", "77011234567", 201, 200},
		{"different_international_send", "77021234567", "77011234567", 502, 0},
		{"different_international_verify", "77011234567", "77021234567", 201, 401},
		{"national_only_is_not_international", "7011234567", "77011234567", 502, 0},
		{"malformed_send", "+not-a-phone", "77011234567", 502, 0},
		{"malformed_verify", "77011234567", "+not-a-phone", 201, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
					t.Error(e)
				}
				phone := tc.sendPhone
				result := map[string]any{"request_id": "provider-bound-request"}
				if r.URL.Path == "/sendVerificationMessage" {
					if req["phone_number"] != "+77011234567" {
						t.Error("outbound phone was not canonical E.164")
					}
				} else {
					phone = tc.verifyPhone
					if req["request_id"] != "provider-bound-request" || req["code"] != "123456" {
						t.Error("verification lost request binding")
					}
					result["verification_status"] = map[string]any{"status": "code_valid"}
				}
				result["phone_number"] = phone
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
			}))
			defer provider.Close()
			gateway := telegram.New("test-only-token", telegram.Options{BaseURL: provider.URL, HTTPClient: provider.Client()})
			f.startServer(gateway)
			sent := f.request("POST", "/api/v1/auth/challenges", map[string]any{"phone": "+77011234567"}, nil)
			expect(t, sent, tc.sendStatus)
			if tc.sendStatus != 201 {
				return
			}
			verified := f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": sent.body["challenge_id"], "code": "123456"}, nil)
			expect(t, verified, tc.verifyStatus)
			var count int
			_ = f.db.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&count)
			want := 0
			if tc.verifyStatus == 200 {
				want = 1
				var stored string
				if e := f.db.QueryRow(context.Background(), "SELECT phone FROM users").Scan(&stored); e != nil || stored != "+77011234567" {
					t.Fatal("account was not bound to canonical stored phone")
				}
			}
			if count != want {
				t.Fatalf("sessions=%d want=%d", count, want)
			}
		})
	}
}
