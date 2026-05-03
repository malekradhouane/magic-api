// identity_store.go
package gatekeeper

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/malekradhouane/magic/pkg/gatekeeper/oidc"
)

// identityStore implements the IdentityStore interface
type identityStore struct {
	store        map[string]SessionData // In-memory store
	sessionTTL   time.Duration
	secret       string
	cookieName   string
	cookieConfig CookieConfig
	claimsConfig ClaimsConfig
}

// NewIdentityStore creates a new IdentityStore instance
func NewIdentityStore(
	sessionTTL time.Duration,
	secret string,
	cookieName string,
	claimsConfig ClaimsConfig,
	cookieConfig CookieConfig,
) IdentityStore {
	return &identityStore{
		store:        make(map[string]SessionData),
		sessionTTL:   sessionTTL,
		secret:       secret,
		cookieName:   cookieName,
		cookieConfig: cookieConfig,
		claimsConfig: claimsConfig,
	}
}

// SaveIdentity saves the identity to the response (as a cookie)
func (s *identityStore) SaveIdentity(
	w http.ResponseWriter,
	r *http.Request,
	subject string,
	tok Token,
	claims map[string]any,
) error {

	// Create session data
	var userInfo oidc.UserInfo
	if username, ok := claims[s.claimsConfig.UsernameClaim].(string); ok {
		userInfo.Username = username
	}
	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}
	if role, ok := claims[s.claimsConfig.RoleClaim].(string); ok {
		userInfo.Role = role
	}

	// Determine expiration
	expiry := time.Now().Add(s.sessionTTL)
	if !tok.Expiry.IsZero() && tok.Expiry.Before(expiry) {
		expiry = tok.Expiry
	}

	// Generate session token
	sessionToken, err := s.generateToken(subject)
	if err != nil {
		return fmt.Errorf("failed to generate session token: %w", err)
	}

	// Create session data
	sessionData := SessionData{
		UserInfo:     userInfo,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenExpiry:  expiry,
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
		UserAgent:    r.UserAgent(),
		ClientIP:     getClientIP(r),
	}

	// Store session
	if err := s.Set(r.Context(), sessionToken, sessionData, time.Until(expiry)); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    sessionToken,
		Expires:  expiry,
		Domain:   s.cookieConfig.Domain,
		Path:     "/",
		Secure:   s.cookieConfig.Secure,
		HttpOnly: s.cookieConfig.HTTPOnly,
		SameSite: getSameSite(s.cookieConfig.SameSite),
	})

	return nil
}

// ClearIdentity clears the identity from the response
func (s *identityStore) ClearIdentity(w http.ResponseWriter, r *http.Request) error {
	// Get session cookie
	cookie, err := r.Cookie(s.cookieName)
	if err == nil {
		// Delete from store
		_ = s.Delete(r.Context(), cookie.Value)

		// Clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     s.cookieName,
			Value:    "",
			Expires:  time.Unix(0, 0),
			Domain:   s.cookieConfig.Domain,
			Path:     "/",
			Secure:   s.cookieConfig.Secure,
			HttpOnly: s.cookieConfig.HTTPOnly,
			SameSite: getSameSite(s.cookieConfig.SameSite),
		})
	}

	return nil
}

// SubjectFromRequest extracts the subject from the request
func (s *identityStore) SubjectFromRequest(r *http.Request) (string, bool) {
	// Get session cookie
	cookie, err := r.Cookie(s.cookieName)
	if err != nil {
		return "", false
	}

	// Get session data
	sessionData, err := s.Get(r.Context(), cookie.Value)
	if err != nil {
		return "", false
	}

	return sessionData.UserInfo.Subject, true
}

// Get retrieves session data by token
func (s *identityStore) Get(ctx context.Context, token string) (*SessionData, error) {
	// Verify token signature
	if err := s.verifyToken(token); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	data, exists := s.store[token]
	if !exists {
		return nil, errors.New("session not found")
	}

	// Check if session is expired
	if time.Now().After(data.TokenExpiry) {
		return nil, errors.New("session expired")
	}

	return &data, nil
}

// Set stores session data with the given token
func (s *identityStore) Set(ctx context.Context, token string, data SessionData, ttl time.Duration) error {
	// Verify token signature
	if err := s.verifyToken(token); err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	s.store[token] = data
	return nil
}

// Delete removes session data
func (s *identityStore) Delete(ctx context.Context, token string) error {
	delete(s.store, token)
	return nil
}

// verifyToken verifies the token signature
func (s *identityStore) verifyToken(token string) error {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("failed to decode token: %w", err)
	}

	if len(decoded) < 64 {
		return errors.New("invalid token format")
	}

	payload := decoded[:len(decoded)-32]
	expectedSignature := decoded[len(decoded)-32:]

	mac := hmac.New(sha256.New, []byte(s.secret))
	_, err = mac.Write(payload)
	if err != nil {
		return fmt.Errorf("failed to verify HMAC: %w", err)
	}
	actualSignature := mac.Sum(nil)

	if !hmac.Equal(expectedSignature, actualSignature) {
		return errors.New("invalid token signature")
	}

	return nil
}

// generateToken generates a new signed token
func (s *identityStore) generateToken(subject string) (string, error) {
	payload := fmt.Sprintf("%s:%d", subject, time.Now().Unix())

	mac := hmac.New(sha256.New, []byte(s.secret))
	_, err := mac.Write([]byte(payload))
	if err != nil {
		return "", fmt.Errorf("failed to generate HMAC: %w", err)
	}
	signature := mac.Sum(nil)

	token := append([]byte(payload), signature...)
	return base64.URLEncoding.EncodeToString(token), nil
}

// Helper functions
func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
	}
	return ip
}

func getSameSite(value string) http.SameSite {
	switch value {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}
