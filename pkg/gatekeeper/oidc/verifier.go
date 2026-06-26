package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrJWKSFetchFailed   = errors.New("failed to fetch JWKS")
	ErrNoMatchingKey     = errors.New("no matching key found")
	ErrTokenVerification = errors.New("token verification failed")
	ErrTokenExpired      = errors.New("token expired")
	ErrInvalidIssuer     = errors.New("invalid issuer")
	ErrInvalidAudience   = errors.New("invalid audience")
	ErrInvalidNonce      = errors.New("invalid nonce")
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	Kid string   `json:"kid"`
	X5c []string `json:"x5c,omitempty"`
	N   string   `json:"n,omitempty"`
	E   string   `json:"e,omitempty"`
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWTVerifier handles JWT verification
type JWTVerifier struct {
	jwksURL      string
	httpClient   *http.Client
	issuer       string
	clientID     string
	skipIssuer   bool
	skipClientID bool
	jwksCache    map[string]JWK
	cacheMutex   sync.Mutex
}

// NewJWTVerifier creates a new JWT verifier
func NewJWTVerifier(jwksURL, issuer, clientID string, skipIssuer, skipClientID bool) *JWTVerifier {
	return &JWTVerifier{
		jwksURL:      jwksURL,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		issuer:       issuer,
		clientID:     clientID,
		skipIssuer:   skipIssuer,
		skipClientID: skipClientID,
		jwksCache:    make(map[string]JWK),
	}
}

// Verify verifies the JWT
func (v *JWTVerifier) Verify(ctx context.Context, tokenString string, nonce string) error {
	// Parse the token without verification
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTokenVerification, err)
	}

	// Get the key ID from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return fmt.Errorf("%w: no kid in token header", ErrTokenVerification)
	}

	// Get the signing algorithm
	alg, ok := token.Header["alg"].(string)
	if !ok {
		return fmt.Errorf("%w: no alg in token header", ErrTokenVerification)
	}

	// Get the key from JWKS
	key, err := v.getKey(ctx, kid, alg)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTokenVerification, err)
	}

	// Parse and verify the token
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the algorithm
		if token.Method.Alg() != alg {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the key
		return key, nil
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTokenVerification, err)
	}

	// Check if token is valid
	if !parsedToken.Valid {
		return fmt.Errorf("%w: token is not valid", ErrTokenVerification)
	}

	// Get claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("%w: invalid claims type", ErrTokenVerification)
	}

	// Verify issuer
	if !v.skipIssuer {
		iss, ok := claims["iss"].(string)
		if !ok || iss != v.issuer {
			return fmt.Errorf("%w: expected %s, got %v", ErrInvalidIssuer, v.issuer, iss)
		}
	}

	// Verify audience
	if !v.skipClientID {
		aud, ok := claims["aud"].(string)
		if !ok {
			// Try array of audiences
			auds, ok := claims["aud"].([]interface{})
			if !ok {
				return fmt.Errorf("%w: no audience in token", ErrInvalidAudience)
			}
			found := false
			for _, a := range auds {
				if aStr, ok := a.(string); ok && aStr == v.clientID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%w: client ID not in audience", ErrInvalidAudience)
			}
		} else if aud != v.clientID {
			return fmt.Errorf("%w: expected %s, got %s", ErrInvalidAudience, v.clientID, aud)
		}
	}

	// Verify expiration
	exp, ok := claims["exp"].(float64)
	if !ok {
		return fmt.Errorf("%w: no expiration in token", ErrTokenExpired)
	}
	if time.Now().Unix() > int64(exp) {
		return fmt.Errorf("%w: token expired at %v", ErrTokenExpired, time.Unix(int64(exp), 0))
	}

	// Verify nonce if provided
	if nonce != "" {
		tokenNonce, ok := claims["nonce"].(string)
		if !ok || tokenNonce != nonce {
			return fmt.Errorf("%w: expected %s, got %v", ErrInvalidNonce, nonce, tokenNonce)
		}
	}

	return nil
}

// getKey retrieves the key from JWKS
func (v *JWTVerifier) getKey(ctx context.Context, kid, alg string) (interface{}, error) {
	// Check cache first
	v.cacheMutex.Lock()
	if key, exists := v.jwksCache[kid]; exists {
		v.cacheMutex.Unlock()
		return v.jwkToKey(key)
	}
	v.cacheMutex.Unlock()

	// Fetch JWKS
	jwks, err := v.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}

	// Find the key
	var key JWK
	for _, k := range jwks.Keys {
		if k.Kid == kid && k.Use == "sig" {
			// Check if algorithm matches
			if (alg == "RS256" && k.Kty == "RSA") ||
				(alg == "ES256" && k.Kty == "EC") {
				key = k
				break
			}
		}
	}

	if key.Kid == "" {
		return nil, ErrNoMatchingKey
	}

	// Cache the key
	v.cacheMutex.Lock()
	v.jwksCache[kid] = key
	v.cacheMutex.Unlock()

	return v.jwkToKey(key)
}

// fetchJWKS fetches the JWKS from the provider
func (v *JWTVerifier) fetchJWKS(ctx context.Context) (*JWKS, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}

	return &jwks, nil
}

// jwkToKey converts a JWK to a crypto key
func (v *JWTVerifier) jwkToKey(jwk JWK) (interface{}, error) {
	switch jwk.Kty {
	case "RSA":
		return v.rsaPublicKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

// rsaPublicKey creates an RSA public key from JWK
func (v *JWTVerifier) rsaPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %v", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %v", err)
	}

	if len(eBytes) < 4 {
		eBytes = append(make([]byte, 4-len(eBytes)), eBytes...)
	}

	e := int(big.NewInt(0).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{
		N: big.NewInt(0).SetBytes(nBytes),
		E: e,
	}, nil
}
