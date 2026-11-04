package config

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"errors"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
)

func ValidatePush(public, private, subject string) error {
	if public == "" && private == "" && subject == "" {
		return nil
	}
	pub, e1 := base64.RawURLEncoding.DecodeString(public)
	key, e2 := base64.RawURLEncoding.DecodeString(private)
	if e1 != nil || e2 != nil || len(pub) != 65 || len(key) != 32 {
		return errors.New("WEB_PUSH_PUBLIC_KEY and WEB_PUSH_PRIVATE_KEY must be a matching VAPID pair")
	}
	curve := elliptic.P256()
	scalar := new(big.Int).SetBytes(key)
	if scalar.Sign() == 0 || scalar.Cmp(curve.Params().N) >= 0 {
		return errors.New("Invalid VAPID private key")
	}
	x, y := curve.ScalarBaseMult(key)
	if x == nil || !bytes.Equal(elliptic.Marshal(curve, x, y), pub) {
		return errors.New("WEB_PUSH_PUBLIC_KEY and WEB_PUSH_PRIVATE_KEY must be a matching VAPID pair")
	}
	u, err := url.Parse(subject)
	valid := err == nil && len(subject) <= 2048 && !strings.ContainsAny(subject, "\r\n\t\\") && u.User == nil && u.Fragment == ""
	if valid && u.Scheme == "mailto" {
		address, parseErr := mail.ParseAddress(u.Opaque)
		valid = parseErr == nil && address.Name == "" && address.Address == u.Opaque
	} else if valid {
		valid = u.Scheme == "https" && u.Host != ""
	}
	if !valid {
		return errors.New("WEB_PUSH_SUBJECT must be a contact HTTPS URL or mailto address")
	}
	return nil
}
