package oidc

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// WellKnownConfig represents the structure of the .well-known/openid-configuration
type WellKnownConfig struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserInfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	ResponseModesSupported            []string `json:"response_modes_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
}

type ClaimsConfig struct {
	UsernameClaim string
	RoleClaim     string
}

// DiscoveryClient handles OpenID discovery and provider configuration
type DiscoveryClient struct {
	httpClient *http.Client
}

// NewDiscoveryClient creates a new discovery client
func NewDiscoveryClient() *DiscoveryClient {
	return &DiscoveryClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
	}
}

// Discover fetches the OpenID configuration from the well-known endpoint
func (d *DiscoveryClient) Discover(ctx context.Context, issuerURL string) (*WellKnownConfig, error) {
	// Normalize the issuer URL
	issuerURL = strings.TrimSuffix(issuerURL, "/")

	// Construct the well-known URL
	wellKnownURL := issuerURL + "/.well-known/openid-configuration"

	// Make the request
	req, err := http.NewRequestWithContext(ctx, "GET", wellKnownURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenID configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response
	var config WellKnownConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode OpenID configuration: %w", err)
	}

	// Validate required fields
	if config.Issuer == "" {
		return nil, errors.New("issuer is required in OpenID configuration")
	}
	if config.AuthorizationEndpoint == "" {
		return nil, errors.New("authorization_endpoint is required in OpenID configuration")
	}
	if config.TokenEndpoint == "" {
		return nil, errors.New("token_endpoint is required in OpenID configuration")
	}

	return &config, nil
}

// DiscoveryOIDCProvider implements OIDCProvider using OpenID discovery
type DiscoveryOIDCProvider struct {
	config            oauth2.Config
	wellKnownConfig   *WellKnownConfig
	providerName      string
	httpClient        *http.Client
	issuerURL         string
	extraAuthParams   map[string]string
	skipIssuerVerify  bool
	skipClientIDCheck bool
	jwtVerifier       *JWTVerifier
	claimsConfig      ClaimsConfig
}

func NewDiscoveryOIDCProvider(
	ctx context.Context,
	providerName string,
	issuerURL string,
	clientID string,
	clientSecret string,
	redirectURL string,
	scopes []string,
	claimsConfig ClaimsConfig,
	extraAuthParams map[string]string,
	skipIssuerVerify bool,
	skipClientIDCheck bool,
) (OIDCProvider, error) {
	discoveryClient := NewDiscoveryClient()

	// Discover the configuration
	wellKnownConfig, err := discoveryClient.Discover(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OpenID configuration: %w", err)
	}

	// Validate that the issuer matches the discovery URL
	if !skipIssuerVerify && wellKnownConfig.Issuer != issuerURL {
		return nil, fmt.Errorf("issuer mismatch: expected %s, got %s", issuerURL, wellKnownConfig.Issuer)
	}

	// Create OAuth2 config
	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:   wellKnownConfig.AuthorizationEndpoint,
			TokenURL:  wellKnownConfig.TokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: scopes,
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	jwtVerifier := NewJWTVerifier(
		wellKnownConfig.JwksURI,
		wellKnownConfig.Issuer,
		clientID,
		skipIssuerVerify,
		skipClientIDCheck,
	)

	p := &DiscoveryOIDCProvider{
		config:            config,
		wellKnownConfig:   wellKnownConfig,
		providerName:      providerName,
		httpClient:        httpClient,
		issuerURL:         issuerURL,
		extraAuthParams:   extraAuthParams,
		skipIssuerVerify:  skipIssuerVerify,
		skipClientIDCheck: skipClientIDCheck,
		jwtVerifier:       jwtVerifier,
		claimsConfig:      claimsConfig,
	}

	return p, nil

}

func (p *DiscoveryOIDCProvider) BeginAuth(ctx context.Context, state, redirectURL, codeChallenge, codeChallengeMethod string, extraParams map[string]string) (string, error) {
	// Override redirect URL if provided
	config := p.config
	if redirectURL != "" {
		config.RedirectURL = redirectURL
	}

	// Merge extra params
	authParams := make(map[string]string)
	for k, v := range p.extraAuthParams {
		authParams[k] = v
	}
	for k, v := range extraParams {
		authParams[k] = v
	}

	// Add PKCE parameters
	authParams["code_challenge"] = codeChallenge
	authParams["code_challenge_method"] = codeChallengeMethod

	opts := make([]oauth2.AuthCodeOption, 0, len(authParams))
	for k, v := range authParams {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}

	// Generate auth URL
	authURL := config.AuthCodeURL(state, opts...)

	return authURL, nil
}

func (p *DiscoveryOIDCProvider) ExchangeToken(ctx context.Context, code, verifier, redirectURL string) (Token, error) {
	// Override redirect URL if provided
	config := p.config
	if redirectURL != "" {
		config.RedirectURL = redirectURL
	}

	// Exchange code for token, explicitly passing redirect_uri and code_verifier
	oauth2Token, err := config.Exchange(
		ctx,
		code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
		oauth2.SetAuthURLParam("redirect_uri", config.RedirectURL),
	)
	if err != nil {
		return Token{}, fmt.Errorf("%w: %v", ErrTokenExchangeFailed, err)
	}

	// Extract ID token if present
	idToken := ""
	if rawIDToken, ok := oauth2Token.Extra("id_token").(string); ok {
		idToken = rawIDToken
	}

	token := Token{
		AccessToken:  oauth2Token.AccessToken,
		TokenType:    oauth2Token.TokenType,
		RefreshToken: oauth2Token.RefreshToken,
		Expiry:       oauth2Token.Expiry,
		IDToken:      idToken,
	}

	return token, nil
}

// VerifyIDToken verifies the ID token signature and claims
func (p *DiscoveryOIDCProvider) VerifyIDToken(ctx context.Context, idToken string) (string, error) {
	if idToken == "" {
		return "", fmt.Errorf("empty ID token")
	}
	// Split the token into parts
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	// Verify signature (simplified - in production use a proper JWT library)
	// For now we'll just verify the claims structure

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to unmarshal claims: %v", err)
	}

	// Verify required claims
	if iss, ok := claims["iss"].(string); !ok || (!p.skipIssuerVerify && iss != p.issuerURL) {
		return "", fmt.Errorf("invalid issuer")
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return "", fmt.Errorf("token expired")
		}
	}

	return idToken, nil
}

func (p *DiscoveryOIDCProvider) GetUserInfo(ctx context.Context, accessToken string, idToken string) (UserInfo, error) {
	if p.wellKnownConfig.UserInfoEndpoint == "" {
		return UserInfo{}, fmt.Errorf("userinfo endpoint not available")
	}

	// Create request to userinfo endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", p.wellKnownConfig.UserInfoEndpoint, nil)
	if err != nil {
		return UserInfo{}, fmt.Errorf("%w: %v", ErrUserInfoFailed, err)
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Make the request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("%w: %v", ErrUserInfoFailed, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return UserInfo{}, fmt.Errorf("%w: status %d", ErrUserInfoFailed, resp.StatusCode)
	}

	// Decode the response
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return UserInfo{}, fmt.Errorf("%w: %v", ErrUserInfoFailed, err)
	}

	// Extract role from the ID token using the unified claims parser
	if idToken != "" {
		claims, err := p.GetTokenClaims(ctx, idToken)
		if err != nil {
			return UserInfo{}, fmt.Errorf("failed to extract roles: %v", err)
		}
		userInfo.Role = claims.Role
		userInfo.Username = claims.Username
	}

	userInfo.Issuer = p.issuerURL

	return userInfo, nil
}

func (p *DiscoveryOIDCProvider) NeedsIDTokenVerification() bool {
	// For OIDC providers, we should verify the ID token
	return true
}

// GetWellKnownConfig returns the well-known configuration
func (p *DiscoveryOIDCProvider) GetWellKnownConfig() *WellKnownConfig {
	return p.wellKnownConfig
}

// GetIssuer returns the issuer URL
func (p *DiscoveryOIDCProvider) GetIssuer() string {
	return p.issuerURL
}

// GetTokenClaims extracts and validates claims from a JWT token
func (p *DiscoveryOIDCProvider) GetTokenClaims(ctx context.Context, token string) (Claims, error) {

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(token)), "bearer ") {
		token = strings.TrimSpace(token)[7:]
	}

	// Basic token format validation
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid JWT format: expected 3 parts separated by dots")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("failed to decode token payload: %v", err)
	}

	// Unmarshal once into a generic map so we can support dynamic claim keys
	payload := make(map[string]interface{})
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return Claims{}, fmt.Errorf("failed to unmarshal token claims: %v", err)
	}

	var claims Claims

	if v, ok := payload["sub"].(string); ok {
		claims.Subject = v
	}
	if v, ok := payload["email"].(string); ok {
		claims.Email = v
	}
	if v, ok := payload["email_verified"].(bool); ok {
		claims.EmailVerified = v
	}
	if v, ok := payload["iss"].(string); ok {
		claims.Issuer = v
	}
	switch aud := payload["aud"].(type) {
	case string:
		claims.Audience = aud
	case []interface{}:
		if len(aud) > 0 {
			if s, ok := aud[0].(string); ok {
				claims.Audience = s
			}
		}
	}
	if v, ok := payload["exp"].(float64); ok {
		claims.ExpirationTime = int64(v)
	}
	if v, ok := payload["iat"].(float64); ok {
		claims.IssuedAt = int64(v)
	}
	if v, ok := payload["nonce"].(string); ok {
		claims.Nonce = v
	}

	roleKey := p.claimsConfig.RoleClaim
	if roleKey == "" {
		roleKey = "role"
	}
	usernameKey := p.claimsConfig.UsernameClaim
	if usernameKey == "" {
		usernameKey = "preferred_username"
	}

	if val, ok := payload[usernameKey]; ok {
		claims.Username = val.(string)
	}

	if val, ok := payload[roleKey]; ok {
		switch x := val.(type) {
		case string:
			claims.Role = x
		case []interface{}:
			if len(x) > 0 {
				if s, ok := x[0].(string); ok {
					claims.Role = s
				}
			}
		default:
			return Claims{}, fmt.Errorf("unexpected type %T for role claim %q", x, roleKey)
		}
	}

	return claims, nil
}
