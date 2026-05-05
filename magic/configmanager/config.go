package configmanager

const (
	ModeProd = "prod"
	ModeTest = "test"
	ModeDev  = "dev"
)

type Magic struct {
	Startup    Startup        `koanf:"startup"`
	BaseURL    string         `koanf:"base_url"`
	Mode       string         `koanf:"mode"` // operation mode for magic-api enum of "prod", "test", "dev", etc…
	Customer   map[string]any `koanf:"customer"`
	HttpServer HttpServer     `koanf:"httpserver"`
	Logging    Logging        `koanf:"logging"`
	Storage    Storage        `koanf:"storages"`
	Auth       AuthConfig     `yaml:"auth" koanf:"auth"`
	Email      EmailConfig    `yaml:"email" koanf:"email"`
	R2         R2Config       `yaml:"r2" koanf:"r2"`
}

// R2Config holds Cloudflare R2 (S3-compatible) configuration.
// Secret values (AccessKeyID, SecretAccessKey) MUST come from environment
// variables, not from the YAML file.
type R2Config struct {
	// Endpoint overrides the default R2 endpoint. Use this to point at
	// MinIO or any other S3-compatible service (e.g. "http://localhost:9000").
	Endpoint        string `yaml:"endpoint" koanf:"endpoint"`
	Region          string `yaml:"region" koanf:"region"`
	AccountID       string `yaml:"account_id" koanf:"account_id"`
	Bucket          string `yaml:"bucket" koanf:"bucket"`
	AccessKeyID     string `yaml:"access_key_id" koanf:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key" koanf:"secret_access_key"`
	// PublicBaseURL is the public-facing URL where uploaded objects are served
	// (e.g. "https://cdn.example.com" or "https://pub-xxx.r2.dev").
	PublicBaseURL string `yaml:"public_base_url" koanf:"public_base_url"`
	// PresignTTLSeconds controls how long the upload URL stays valid.
	PresignTTLSeconds int `yaml:"presign_ttl_seconds" koanf:"presign_ttl_seconds"`
}

type OIDCConfig struct {
	Enabled         bool              `yaml:"enabled" koanf:"enabled"`
	ProviderName    string            `yaml:"provider_name" koanf:"provider_name"`
	IssuerURL       string            `yaml:"issuer_url" koanf:"issuer_url"`
	ClientID        string            `yaml:"client_id" koanf:"client_id"`
	ClientSecret    string            `yaml:"client_secret" koanf:"client_secret"`
	RedirectURL     string            `yaml:"redirect_url" koanf:"redirect_url"`
	Scopes          []string          `yaml:"scopes" koanf:"scopes"`
	Audience        string            `yaml:"audience" koanf:"audience"`
	ExpectedIssuer  string            `yaml:"expected_issuer" koanf:"expected_issuer"`
	ExtraAuthParams map[string]string `koanf:"extraAuthParams"`
	PublicBaseURL   string            `koanf:"public_base_url"`
	Claims          OIDCClaims        `yaml:"claims" koanf:"claims"`
}

type OIDCClaims struct {
	UsernameClaim string `yaml:"username_claim" koanf:"username_claim"`
	RoleClaim     string `yaml:"role_claim" koanf:"role_claim"`
}

type CookieConfig struct {
	Domain        string `yaml:"domain" koanf:"domain"`
	Secure        bool   `yaml:"secure" koanf:"secure"`
	HTTPOnly      bool   `yaml:"http_only" koanf:"http_only"`
	SameSite      string `yaml:"same_site" koanf:"same_site"`
	SessionSecret string `yaml:"session_secret" koanf:"session_secret"`
	SessionTTL    int    `yaml:"session_ttl" koanf:"session_ttl"`
}

type AuthConfig struct {
	OIDC   OIDCConfig   `yaml:"oidc" koanf:"oidc"`
	Cookie CookieConfig `yaml:"cookie" koanf:"cookie"`
}

// EmailConfig holds email provider configurations.
type EmailConfig struct {
	Mailjet MailjetConfig `yaml:"mailjet" koanf:"mailjet"`
}

// MailjetConfig contains Mailjet API credentials and default sender.
type MailjetConfig struct {
	APIKeyPublic  string `yaml:"api_key_public" koanf:"api_key_public"`
	APIKeyPrivate string `yaml:"api_key_private" koanf:"api_key_private"`
	FromName      string `yaml:"from_name" koanf:"from_name"`
	FromEmail     string `yaml:"from_email" koanf:"from_email"`
}

type (
	Startup struct {
		ExecPre []StartupExecPre `koanf:"execPre"`
	}

	StartupExecPre struct {
		Name              string   `koanf:"name"`
		Desc              string   `koanf:"desc"`
		Disabled          bool     `koanf:"disabled"`
		ContinueOnFailure bool     `koanf:"continueOnFailure"`
		WorkDir           string   `koanf:"workDir"`
		Command           string   `koanf:"command"`
		Args              []string `koanf:"args"`
		Env               []string `koanf:"env"`
	}
)

type (
	Storage struct {
		PostgreSQL StoragePostgreSQL `koanf:"postgresql"`
	}
	StoragePostgreSQL struct {
		Disabled   bool   `koanf:"disabled"`
		Host       string `koanf:"host"`
		Port       uint   `koanf:"port"`
		MagicDB    string `koanf:"magicDB"`
		MagicLogin string `koanf:"magicLogin"`
		MagicPass  string `koanf:"magicPass"`
		AdminDB    string `koanf:"adminDB"`
		AdminLogin string `koanf:"adminLogin"`
		AdminPass  string `koanf:"adminPass"`
	}
)

type Logging struct {
	Level     string `koanf:"level"`
	Formatter string `koanf:"formatter"`
	Verbose   bool   `koanf:"verbose"`
}

type HttpServer struct {
	Listen    string `koanf:"listen"`
	Port      uint   `koanf:"port"`
	TLS       bool   `koanf:"tls"`
	CertFile  string `koanf:"certFile"`
	KeyFile   string `koanf:"keyFile"`
	JWTSecret string `koanf:"jwtSecret"` //
}
