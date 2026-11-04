package config

import "testing"

func TestProductionOriginPolicy(t *testing.T) {
	for _, raw := range []string{"http://bikers.kz", "https://bikers.kz/path", "https://user:secret@bikers.kz", "https://bikers.kz?x=1"} {
		if _, err := ValidateBaseURL(raw); err == nil {
			t.Errorf("invalid origin accepted: %q", raw)
		}
	}
	for _, raw := range []string{"https://bikers.kz", "http://localhost:8080", "http://127.0.0.1:8080"} {
		if _, err := ValidateBaseURL(raw); err != nil {
			t.Errorf("valid origin rejected: %q", raw)
		}
	}
	if _, err := ParseTrustedProxies("192.0.2.1/32, 2001:db8::1/128"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTrustedProxies("192.0.2.1"); err == nil {
		t.Fatal("proxy allowlist accepted a non-prefix value")
	}
}
