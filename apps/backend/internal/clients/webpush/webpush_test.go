package webpush

import (
	"bikers.kz/backend/internal/models"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type pushHTTP func(*http.Request) (*http.Response, error)

func (f pushHTTP) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestPushEncryptionAndVAPID(t *testing.T) {
	server, _ := ecdh.P256().GenerateKey(rand.Reader)
	browser, _ := ecdh.P256().GenerateKey(rand.Reader)
	encode := base64.RawURLEncoding.EncodeToString
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	called := false
	client := New(Options{PublicKey: encode(server.PublicKey().Bytes()), PrivateKey: encode(server.Bytes()), Subject: "mailto:operator@example.com", HTTPClient: pushHTTP(func(r *http.Request) (*http.Response, error) {
		called = true
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("You have a new update")) || len(data) < 100 {
			t.Fatal("payload must be encrypted")
		}
		if r.Method != "POST" || r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(r.Header.Get("Authorization"), "vapid ") || r.Header.Get("TTL") != "300" || len(r.Header.Get("Topic")) != 32 {
			t.Fatal("Web Push protocol headers missing")
		}
		// The gateway library accepts an email without its URI prefix; guard
		// against accidentally sending mailto:mailto: in the signed subject.
		token := strings.TrimPrefix(strings.SplitN(r.Header.Get("Authorization"), ",", 2)[0], "vapid t=")
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatal("VAPID token missing")
		}
		claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal("VAPID claims invalid")
		}
		var claims map[string]any
		if json.Unmarshal(claimsBytes, &claims) != nil || claims["sub"] != "mailto:operator@example.com" || claims["aud"] != "https://fcm.googleapis.com" {
			t.Fatal("VAPID contact or provider binding invalid")
		}

		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	input := models.PushSubscriptionInput{Endpoint: "https://fcm.googleapis.com/fcm/send/test", Keys: models.PushKeys{P256dh: encode(browser.PublicKey().Bytes()), Auth: encode(auth)}}
	status, err := client.Send(context.Background(), input, models.PushPayload{Title: "BIKERS.KZ", Body: "You have a new update", URL: "/inbox", NotificationID: "test"})
	if err != nil || status != 201 || !called {
		t.Fatal("valid subscription was not delivered")
	}
	called = false
	input.Endpoint = "https://127.0.0.1/private"
	if _, err = client.Send(context.Background(), input, models.PushPayload{}); err == nil || called {
		t.Fatal("non-provider endpoint reached the transport")
	}
}

func TestPushRejectsPrivateAndSpecialAddresses(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.1.2", "192.0.2.1", "2001:db8::1", "fc00::1", "0.0.0.0", "224.0.0.1", "240.0.0.1", "192.88.99.1", "fec0::1"} {
		if publicIP(netip.MustParseAddr(value)) {
			t.Fatalf("non-public address accepted: %s", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicIP(netip.MustParseAddr(value)) {
			t.Fatal("public address rejected")
		}
	}
	for _, value := range []string{"http://fcm.googleapis.com/a", "https://fcm.googleapis.com:443/a", "https://fcm.googleapis.com.evil.test/a", "https://user@fcm.googleapis.com/a", "https://fcm.googleapis.com/a#fragment"} {
		if allowedEndpoint(value) {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
