package budgethints

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client refreshes gateway hints only when explicitly invoked by a caller.
// It is deliberately not used from the command execution path.
type Client struct {
	endpoint  string
	token     string
	publicKey ed25519.PublicKey
	store     *Store
	http      *http.Client
	now       func() time.Time
}

type ClientConfig struct {
	Endpoint         string
	BearerToken      string
	SigningPublicKey string // base64 encoded Ed25519 public key
	Store            *Store
	HTTPClient       *http.Client
	Now              func() time.Time
}

// Envelope is the signed gateway response. The signature is Ed25519 over the
// JSON encoding of Version, IssuedAt, and Hints in that order.
type Envelope struct {
	Version   int       `json:"version"`
	IssuedAt  time.Time `json:"issued_at"`
	Hints     []Hint    `json:"hints"`
	Signature string    `json:"signature"`
}

type unsignedEnvelope struct {
	Version  int       `json:"version"`
	IssuedAt time.Time `json:"issued_at"`
	Hints    []Hint    `json:"hints"`
}

func NewClient(cfg ClientConfig) (*Client, error) {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, errors.New("gateway hint client requires an explicit HTTPS endpoint")
	}
	if strings.TrimSpace(cfg.BearerToken) == "" {
		return nil, errors.New("gateway hint client requires a bearer token")
	}
	key, err := base64.StdEncoding.DecodeString(cfg.SigningPublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("gateway hint client requires a base64 Ed25519 public key")
	}
	if cfg.Store == nil {
		return nil, errors.New("gateway hint client requires a local store")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Client{endpoint: endpoint.String(), token: cfg.BearerToken, publicKey: ed25519.PublicKey(key), store: cfg.Store, http: client, now: now}, nil
}

// Refresh fetches, verifies, validates, then atomically installs a complete
// replacement set. A failed refresh leaves existing hints untouched.
func (c *Client) Refresh(ctx context.Context) (int, error) {
	if c == nil {
		return 0, errors.New("gateway hint client is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("gateway hint refresh returned HTTP %d", response.StatusCode)
	}
	var envelope Envelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return 0, fmt.Errorf("decode gateway hints: %w", err)
	}
	if err := c.verify(envelope); err != nil {
		return 0, err
	}
	for _, hint := range envelope.Hints {
		if err := Validate(hint); err != nil {
			return 0, err
		}
		if !hint.ExpiresAt.After(c.now()) {
			return 0, errors.New("gateway hint is already expired")
		}
		if hint.ExpiresAt.After(c.now().Add(7 * 24 * time.Hour)) {
			return 0, errors.New("gateway hint expiry exceeds seven-day limit")
		}
	}
	if err := c.store.Replace(envelope.Hints); err != nil {
		return 0, err
	}
	return len(envelope.Hints), nil
}

func (c *Client) verify(envelope Envelope) error {
	if envelope.Version != CurrentVersion || envelope.IssuedAt.IsZero() {
		return errors.New("unsupported or incomplete gateway hint envelope")
	}
	if envelope.IssuedAt.After(c.now().Add(5*time.Minute)) || envelope.IssuedAt.Before(c.now().Add(-24*time.Hour)) {
		return errors.New("gateway hint envelope timestamp is outside the accepted window")
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("gateway hint envelope has an invalid signature")
	}
	body, err := json.Marshal(unsignedEnvelope{Version: envelope.Version, IssuedAt: envelope.IssuedAt, Hints: envelope.Hints})
	if err != nil || !ed25519.Verify(c.publicKey, bytes.TrimSpace(body), signature) {
		return errors.New("gateway hint envelope signature verification failed")
	}
	return nil
}
