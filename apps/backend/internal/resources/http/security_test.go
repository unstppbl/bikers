package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
)

func TestSourceLimitsTrustOnlyConfiguredProxy(t *testing.T) {
	r := httptest.NewRequest("POST", "https://bikers.kz/api/v1/auth/challenges", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("X-Forwarded-For", "198.51.100.8, 203.0.113.7")
	s := &Server{}
	peer := sha256.Sum256([]byte("192.0.2.10"))
	if s.sourceKey(r) != hex.EncodeToString(peer[:]) {
		t.Fatal("untrusted forwarded header affected rate key")
	}
	s.trustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.10/32")}
	client := sha256.Sum256([]byte("203.0.113.7"))
	if s.sourceKey(r) != hex.EncodeToString(client[:]) {
		t.Fatal("spoofed earlier XFF won over true client appended by proxy")
	}
}

func TestProductionOriginAndCookiePolicy(t *testing.T) {
	u, e := url.Parse("https://bikers.kz")
	if e != nil {
		t.Fatal(e)
	}
	s := &Server{origin: u}
	if s.cookieName() != "__Host-bikerskz_session" {
		t.Fatal("production must use host-only secure cookie name")
	}
	r := httptest.NewRequest("POST", "https://bikers.kz", nil)
	r.Header.Set("Origin", "https://attacker.example")
	if s.sameOrigin(r) {
		t.Fatal("foreign origin allowed")
	}
}
