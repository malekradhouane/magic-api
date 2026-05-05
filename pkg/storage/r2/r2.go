// Package r2 provides a thin wrapper around the AWS S3 v2 SDK to interact
// with Cloudflare R2 (which is S3-compatible).
//
// It is intentionally minimal: it exposes only what the API needs today,
// namely the ability to generate presigned PUT URLs so that clients can
// upload binary content (images) directly to R2 without streaming through
// the Go server.
package r2

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config carries the values required to talk to a single S3-compatible
// bucket. The same struct works for Cloudflare R2 (prod) and MinIO (local
// dev): only the Endpoint changes.
type Config struct {
	// Endpoint is the full base URL of the S3-compatible service.
	//   - MinIO local: "http://localhost:9000"
	//   - Cloudflare R2: "https://<account_id>.r2.cloudflarestorage.com"
	// If empty, AccountID is used to build an R2 endpoint.
	Endpoint string
	// AccountID is only used when Endpoint is empty (R2 fallback).
	AccountID       string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	// PublicBaseURL is used to construct the public read URL of an object.
	// Example: "https://cdn.example.com" or "http://localhost:9000/<bucket>".
	PublicBaseURL string
	// PresignTTL is the lifetime of generated upload URLs.
	PresignTTL time.Duration
}

// resolvedEndpoint returns the explicit Endpoint or, as a fallback, the
// R2 endpoint derived from AccountID.
func (c Config) resolvedEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	if c.AccountID != "" {
		return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", c.AccountID)
	}
	return ""
}

// Validate ensures the configuration is usable.
func (c Config) Validate() error {
	if c.resolvedEndpoint() == "" {
		return errors.New("r2: endpoint or account_id is required")
	}
	if c.Bucket == "" {
		return errors.New("r2: bucket is required")
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return errors.New("r2: credentials are required")
	}
	if c.PresignTTL <= 0 {
		return errors.New("r2: presign_ttl must be > 0")
	}
	return nil
}

// Client wraps an S3 client and presigner pre-configured for R2.
type Client struct {
	cfg       Config
	s3Client  *s3.Client
	presigner *s3.PresignClient
}

// New builds an R2 client from the provided configuration.
// R2 uses a fixed region "auto" and an account-scoped endpoint.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	endpoint := cfg.resolvedEndpoint()
	region := cfg.Region
	if region == "" {
		// "auto" is the canonical region for R2 and is also accepted by MinIO.
		region = "auto"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		// R2 does not yet support virtual-hosted style addressing for all
		// regions; path-style is the safest default.
		o.UsePathStyle = true
	})

	return &Client{
		cfg:       cfg,
		s3Client:  s3Client,
		presigner: s3.NewPresignClient(s3Client),
	}, nil
}

// PresignedUpload describes the result of generating a presigned PUT URL.
type PresignedUpload struct {
	UploadURL   string            `json:"upload_url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Key         string            `json:"key"`
	PublicURL   string            `json:"public_url"`
	ExpiresIn   int               `json:"expires_in"`
	ContentType string            `json:"content_type"`
}

// PresignPut returns a presigned URL that the client can use to PUT the
// object directly to R2. The caller is responsible for sending the request
// with the same Content-Type that was provided here.
func (c *Client) PresignPut(ctx context.Context, key, contentType string) (*PresignedUpload, error) {
	if key == "" {
		return nil, errors.New("r2: key is required")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	req, err := c.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(c.cfg.PresignTTL))
	if err != nil {
		return nil, fmt.Errorf("r2: presign put: %w", err)
	}

	return &PresignedUpload{
		UploadURL: req.URL,
		Method:    req.Method,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		Key:         key,
		PublicURL:   c.PublicURL(key),
		ExpiresIn:   int(c.cfg.PresignTTL.Seconds()),
		ContentType: contentType,
	}, nil
}

// PublicURL builds the public read URL for the given object key, when a
// public base URL has been configured.
func (c *Client) PublicURL(key string) string {
	if c.cfg.PublicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(c.cfg.PublicBaseURL, "/") + "/" + strings.TrimLeft(key, "/")
}
