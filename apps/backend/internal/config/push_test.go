package config

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidatePush(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	public, private := encode(key.PublicKey().Bytes()), encode(key.Bytes())
	for _, subject := range []string{"mailto:operator@example.com", "https://example.com/contact"} {
		if err := ValidatePush(public, private, subject); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidatePush("", "", ""); err != nil {
		t.Fatal("push must remain optional")
	}
	for _, values := range [][3]string{
		{public, "", "mailto:a@example.com"},
		{public, private, "http://example.com"},
		{public, private, "https://user:password@example.com"},
		{public, private, "https://example.com/#fragment"},
		{public, private, "mailto:@"},
		{public, private, "mailto:operator@"},
		{public, private, "mailto:Operator <operator@example.com>"},
		{public, private, "https://example.com/" + strings.Repeat("a", 2048)},
		{public, private, "https://example.com/a\\path"},
		{public, encode(make([]byte, 32)), "mailto:a@example.com"},
		{public, encode([]byte(strings.Repeat("\xff", 32))), "mailto:a@example.com"},
	} {
		if ValidatePush(values[0], values[1], values[2]) == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
