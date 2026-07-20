package budgethints

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientRefreshVerifiesAndAtomicallyInstallsHints(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hints := []Hint{{Version: CurrentVersion, Profile: "go-test", Direction: DirectionTighten, Samples: 20, ExpiresAt: now.Add(time.Hour), Suggested: Target{MaxLines: 10}}}
	unsigned := unsignedEnvelope{Version: CurrentVersion, IssuedAt: now, Hints: hints}
	body, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	envelope := Envelope{Version: unsigned.Version, IssuedAt: unsigned.IssuedAt, Hints: hints, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, body))}
	client, err := NewClient(ClientConfig{Endpoint: "https://gateway.example/v1/hints", BearerToken: "token", SigningPublicKey: base64.StdEncoding.EncodeToString(publicKey), Store: New(filepath.Join(t.TempDir(), "hints.json")), HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		response, _ := json.Marshal(envelope)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(response)))}, nil
	})}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	count, err := client.Refresh(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("refresh = %d, %v", count, err)
	}
	loaded, err := client.store.Load()
	if err != nil || len(loaded) != 1 || loaded[0].Profile != "go-test" {
		t.Fatalf("load = %#v, %v", loaded, err)
	}
}

func TestClientRejectsBadSignatureWithoutReplacingStore(t *testing.T) {
	now := time.Now().UTC()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(t.TempDir(), "hints.json"))
	old := Hint{Version: CurrentVersion, Profile: "old", Direction: DirectionTighten, Samples: 20, ExpiresAt: now.Add(time.Hour)}
	if err := store.Replace([]Hint{old}); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(ClientConfig{Endpoint: "https://gateway.example/v1/hints", BearerToken: "token", SigningPublicKey: base64.StdEncoding.EncodeToString(publicKey), Store: store, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, _ := json.Marshal(Envelope{Version: CurrentVersion, IssuedAt: now, Signature: base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))})
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(response)))}, nil
	})}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Refresh(context.Background()); err == nil {
		t.Fatal("expected signature error")
	}
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 || loaded[0].Profile != "old" {
		t.Fatalf("existing hints changed: %#v, %v", loaded, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
