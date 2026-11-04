package models

import (
	"errors"
	"github.com/nyaruka/phonenumbers"
	"strings"
	_ "time/tzdata"
)

func ValidLocale(v string) bool { return v == "en" || v == "ru" || v == "kk" }
func NormalizePhone(raw, region string) (string, error) {
	if len(raw) > 64 {
		return "", errors.New("invalid phone")
	}
	p, e := phonenumbers.Parse(strings.TrimSpace(raw), region)
	if e != nil || !phonenumbers.IsValidNumber(p) {
		return "", errors.New("invalid phone")
	}
	return phonenumbers.Format(p, phonenumbers.E164), nil
}

// NormalizeInternationalPhone accepts an optional '+' without guessing a national prefix.
func NormalizeInternationalPhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > 64 {
		return "", errors.New("invalid international phone")
	}
	digits := strings.TrimPrefix(raw, "+")
	if digits == "" {
		return "", errors.New("invalid international phone")
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return "", errors.New("invalid international phone")
		}
	}
	return NormalizePhone("+"+digits, "")
}
