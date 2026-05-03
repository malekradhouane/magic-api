package gatekeeper

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
)

func newTestAppWithSecret(secret string) *application {
	return &application{
		cookieConfig: CookieConfig{
			SessionSecret: secret,
		},
	}
}

func TestGenerateAndVerifySessionToken_Success(t *testing.T) {
	app := newTestAppWithSecret("super-secret")

	subject := "user-123"
	token, err := app.generateSessionToken(subject)
	if err != nil {
		t.Fatalf("generateSessionToken error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	sd := &SessionData{UserInfo: oidcUser(subject)}
	if err := app.verifySessionToken(token, sd); err != nil {
		t.Fatalf("verifySessionToken unexpected error: %v", err)
	}
}

func TestVerifySessionToken_InvalidSignature(t *testing.T) {
	app := newTestAppWithSecret("super-secret")

	subject := "user-123"
	token, err := app.generateSessionToken(subject)
	if err != nil {
		t.Fatalf("generateSessionToken error: %v", err)
	}

	// Tamper with the token by flipping a byte in the signature segment
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if len(raw) < 33 { // payload + 32-byte sig
		t.Fatalf("token raw too short")
	}
	raw[len(raw)-1] ^= 0xFF // corrupt last byte of signature
	badToken := base64.URLEncoding.EncodeToString(raw)

	sd := &SessionData{UserInfo: oidcUser(subject)}
	if err := app.verifySessionToken(badToken, sd); err == nil {
		t.Fatalf("expected signature error, got nil")
	}
}

func TestVerifySessionToken_UserMismatch(t *testing.T) {
	app := newTestAppWithSecret("super-secret")

	token, err := app.generateSessionToken("user-A")
	if err != nil {
		t.Fatalf("generateSessionToken error: %v", err)
	}

	// Session belongs to different subject
	sd := &SessionData{UserInfo: oidcUser("user-B")}
	if err := app.verifySessionToken(token, sd); err == nil || !strings.Contains(err.Error(), "token user mismatch") {
		t.Fatalf("expected user mismatch error, got: %v", err)
	}
}

func TestIsValidURL(t *testing.T) {
	app := newTestAppWithSecret("irrelevant")

	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"not-a-url", false},
		{"http://", false},
		{"http://example.com", true},
		{"https://example.com/callback?x=1", true},
	}

	for _, c := range cases {
		if got := app.isValidURL(c.in); got != c.want {
			t.Errorf("isValidURL(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// helper: minimal UserInfo compatible with SessionData
func oidcUser(sub string) oidc.UserInfo {
	return oidc.UserInfo{Subject: sub}
}
