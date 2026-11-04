// Package webpush owns Web Push encryption, VAPID and bounded provider transport.
package webpush

import (
	"bikers.kz/backend/internal/models"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	protocol "github.com/SherClockHolmes/webpush-go"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}
type Options struct {
	PublicKey, PrivateKey, Subject string
	HTTPClient                     HTTPClient
}
type Client struct {
	options Options
	http    HTTPClient
}

func New(options Options) *Client {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: safeDial, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 16 << 10, IdleConnTimeout: 30 * time.Second}}
	}
	return &Client{options: options, http: client}
}
func allowedEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || u.Host != strings.ToLower(u.Host) {
		return false
	}
	h := u.Hostname()
	return h == "fcm.googleapis.com" || h == "updates.push.services.mozilla.com" || h == "web.push.apple.com" || strings.HasSuffix(h, ".notify.windows.com")
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, raw := range []string{"100.64.0.0/10", "198.18.0.0/15", "192.0.0.0/24", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32", "240.0.0.0/4", "192.88.99.0/24", "fec0::/10"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}
func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, errors.New("push endpoint unavailable")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("push endpoint unavailable")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errors.New("push endpoint unavailable")
		}
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	var last error
	for _, ip := range ips {
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
		last = err
	}
	return nil, last
}
func (c *Client) Send(ctx context.Context, subscription models.PushSubscriptionInput, payload models.PushPayload) (int, error) {
	if !allowedEndpoint(subscription.Endpoint) {
		return 0, errors.New("push endpoint unavailable")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, errors.New("push payload unavailable")
	}
	digest := sha256.Sum256([]byte(payload.NotificationID))
	response, err := protocol.SendNotificationWithContext(ctx, data, &protocol.Subscription{Endpoint: subscription.Endpoint, Keys: protocol.Keys{P256dh: subscription.Keys.P256dh, Auth: subscription.Keys.Auth}}, &protocol.Options{HTTPClient: c.http, Subscriber: strings.TrimPrefix(c.options.Subject, "mailto:"), VAPIDPublicKey: c.options.PublicKey, VAPIDPrivateKey: c.options.PrivateKey, TTL: 300, Topic: base64.RawURLEncoding.EncodeToString(digest[:24])})
	if err != nil {
		return 0, errors.New("push delivery unavailable")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode, nil
}
