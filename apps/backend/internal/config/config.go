package config

import (
	"errors"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config contains deployment configuration; none of it is included in responses.
type Config struct {
	DatabaseURL       string
	BaseURL           string
	Port              string
	MediaDir          string
	WebDistDir        string
	TelegramToken     string
	TrustedProxyCIDRs string
	OTPDailyBudget    int
	PushPublicKey     string
	PushPrivateKey    string
	PushSubject       string
	PhotonBaseURL     string
	ValhallaBaseURL   string
	MapsArchiveURL    string
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), BaseURL: os.Getenv("BASE_URL"), Port: os.Getenv("PORT"), MediaDir: os.Getenv("MEDIA_DIR"), WebDistDir: os.Getenv("WEB_DIST_DIR"), TelegramToken: os.Getenv("TELEGRAM_GATEWAY_TOKEN"), TrustedProxyCIDRs: os.Getenv("TRUSTED_PROXY_CIDRS"), OTPDailyBudget: 1000}
	c.PushPublicKey = os.Getenv("WEB_PUSH_PUBLIC_KEY")
	c.PushPrivateKey = os.Getenv("WEB_PUSH_PRIVATE_KEY")
	c.PushSubject = os.Getenv("WEB_PUSH_SUBJECT")
	c.PhotonBaseURL = os.Getenv("PHOTON_BASE_URL")
	c.ValhallaBaseURL = os.Getenv("VALHALLA_BASE_URL")
	c.MapsArchiveURL = os.Getenv("MAPS_ARCHIVE_URL")
	if err := ValidateMaps(c.PhotonBaseURL, c.ValhallaBaseURL, c.MapsArchiveURL); err != nil {
		return c, err
	}
	if err := ValidatePush(c.PushPublicKey, c.PushPrivateKey, c.PushSubject); err != nil {
		return c, err
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.MediaDir == "" {
		c.MediaDir = "./data/media"
	}
	if c.WebDistDir == "" {
		c.WebDistDir = "../web/dist"
	}
	if v := os.Getenv("OTP_DAILY_BUDGET"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 {
			return c, errors.New("OTP_DAILY_BUDGET must be positive")
		}
		c.OTPDailyBudget = n
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.BaseURL == "" {
		return c, errors.New("BASE_URL is required (use http://localhost:8080 explicitly for local development)")
	}
	if _, e := ValidateBaseURL(c.BaseURL); e != nil {
		return c, e
	}
	return c, nil
}

func ValidateBaseURL(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("BASE_URL must be an absolute origin")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "[::1]" || u.Hostname() == "::1")) {
		return nil, errors.New("BASE_URL requires HTTPS except explicit loopback development")
	}
	u.Path = ""
	u.RawPath = ""
	return u, nil
}

func ParseTrustedProxies(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, errors.New("TRUSTED_PROXY_CIDRS must contain comma-separated IP prefixes")
		}
		out = append(out, prefix)
	}
	return out, nil
}
