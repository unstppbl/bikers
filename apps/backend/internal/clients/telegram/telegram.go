package telegram

import (
	"bikers.kz/backend/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type Client struct {
	token   string
	client  *http.Client
	baseURL string
}

type Options struct {
	HTTPClient *http.Client
	BaseURL    string
}

func New(token string, options Options) *Client {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	endpoint := options.BaseURL
	if endpoint == "" {
		endpoint = "https://gatewayapi.telegram.org"
	}
	return &Client{token: token, client: client, baseURL: endpoint}
}

func (g *Client) call(ctx context.Context, method string, payload any) (models.Verification, error) {
	var empty models.Verification
	if g.token == "" {
		return empty, errors.New("telegram not configured")
	}
	b, e := json.Marshal(payload)
	if e != nil {
		return empty, e
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/"+method, bytes.NewReader(b))
	if e != nil {
		return empty, e
	}
	r.Header.Set("Authorization", "Bearer "+g.token)
	r.Header.Set("Content-Type", "application/json")
	resp, e := g.client.Do(r)
	if e != nil {
		return empty, errors.New("telegram unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return empty, errors.New("telegram request failed")
	}
	var envelope struct {
		OK     bool                `json:"ok"`
		Result models.Verification `json:"result"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&envelope); e != nil || !envelope.OK {
		return empty, errors.New("telegram response invalid")
	}
	return envelope.Result, nil
}
func (g *Client) Send(ctx context.Context, phone string) (models.Verification, error) {
	return g.call(ctx, "sendVerificationMessage", struct {
		Phone      string `json:"phone_number"`
		CodeLength int    `json:"code_length"`
		TTL        int    `json:"ttl"`
	}{phone, 6, 300})
}
func (g *Client) Verify(ctx context.Context, id, code string) (models.Verification, error) {
	return g.call(ctx, "checkVerificationStatus", struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
	}{id, code})
}
