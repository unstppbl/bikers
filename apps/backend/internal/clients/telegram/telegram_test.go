package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramAdapterGeneratedCodesAndStatusEnvelope(t *testing.T) {
	status := "code_invalid"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only-token" {
			t.Error("missing authorization")
		}
		var p map[string]any
		if e := json.NewDecoder(r.Body).Decode(&p); e != nil {
			t.Error(e)
		}
		if r.URL.Path == "/sendVerificationMessage" {
			if p["code_length"] != float64(6) || p["ttl"] != float64(300) || p["code"] != nil {
				t.Error("must use Telegram generated six-digit codes with TTL300")
			}
			writeResponse(w, map[string]any{"ok": true, "result": map[string]any{"request_id": "provider-request", "phone_number": "+77011234567"}})
			return
		}
		if p["request_id"] != "provider-request" || p["code"] != "123456" {
			t.Error("verification request not bound")
		}
		writeResponse(w, map[string]any{"ok": true, "result": map[string]any{"request_id": "provider-request", "phone_number": "+77011234567", "verification_status": map[string]any{"status": status}}})
	}))
	defer ts.Close()
	g := New("test-only-token", Options{BaseURL: ts.URL, HTTPClient: ts.Client()})
	v, e := g.Send(context.Background(), "+77011234567")
	if e != nil || v.RequestID != "provider-request" {
		t.Fatalf("send=%v %v", v, e)
	}
	v, e = g.Verify(context.Background(), v.RequestID, "123456")
	if e != nil || v.VerificationStatus == nil || v.VerificationStatus.Status != "code_invalid" {
		t.Fatalf("adapter discarded verification state: %v %v", v, e)
	}
}

func writeResponse(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
