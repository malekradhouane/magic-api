package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/malekradhouane/magic/middleware"
	"github.com/malekradhouane/magic/pkg/storage/r2"
	"github.com/malekradhouane/magic/utils/httpresp"
)

// allowedImageContentTypes is the small set of MIME types we accept for
// product images. Anything else is rejected at presign time so that a
// rogue caller cannot upload arbitrary binaries to our bucket.
var allowedImageContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/jpg":  "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
	"image/avif": "avif",
}

// PresignUploadRequest is the JSON body sent by the admin frontend.
type PresignUploadRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	// Folder is an optional logical prefix (e.g. "products", "categories").
	// Defaults to "uploads".
	Folder string `json:"folder"`
}

// UploadHandler exposes upload-related endpoints.
type UploadHandler struct {
	r2   *r2.Client
	auth gin.HandlerFunc
}

// NewUploadHandler constructs an UploadHandler. r2 may be nil, in which
// case the routes will return 503.
func NewUploadHandler(r2Client *r2.Client, auth gin.HandlerFunc) *UploadHandler {
	return &UploadHandler{r2: r2Client, auth: auth}
}

// SetupRoutes registers upload routes:
//
//	POST /uploads/presign (admin)
func (h *UploadHandler) SetupRoutes(g *gin.RouterGroup) {
	grp := g.Group("/uploads")
	grp.Use(h.auth)
	grp.Use(middleware.RequireAdmin())
	grp.POST("/presign", h.Presign)
}

// Presign returns a presigned PUT URL the frontend can use to upload an
// image directly to Cloudflare R2.
//
// @Summary  Generate a presigned upload URL for R2
// @Tags     uploads
// @Accept   json
// @Produce  json
// @Param    Authorization header string                true "Bearer token"
// @Param    body          body   PresignUploadRequest  true "Upload metadata"
// @Success  200 {object} r2.PresignedUpload
// @Router   /uploads/presign [post]
func (h *UploadHandler) Presign(c *gin.Context) {
	if h.r2 == nil {
		httpresp.NewErrorMessage(c, http.StatusServiceUnavailable, "object storage is not configured")
		return
	}

	var req PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	canonicalCT, ext, err := validateContentType(req.ContentType)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	key, err := buildObjectKey(req.Folder, req.Filename, ext)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}

	// Sign the URL with the canonical (normalized) content type so that the
	// value returned to the frontend is byte-for-byte identical to the one
	// embedded in the SigV4 signature. The frontend MUST reuse
	// presigned.Headers["Content-Type"] verbatim on the PUT request,
	// otherwise the signature will not match and R2 returns 403.
	presigned, err := h.r2.PresignPut(c.Request.Context(), key, canonicalCT)
	if err != nil {
		httpresp.NewErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	httpresp.NewResult(c, http.StatusOK, presigned)
}

// validateContentType normalizes and validates an image MIME type. It returns
// the canonical content type (lowercased, trimmed, without parameters) that
// must be used for signing AND sent back to the client, together with the
// canonical file extension.
//
// Returning the canonical content type is what guarantees the SigV4 signature
// stays consistent: whatever casing/whitespace/charset the client sends, the
// signed value is always the normalized one.
func validateContentType(ct string) (string, string, error) {
	ct = strings.ToLower(strings.TrimSpace(ct))
	// Drop any media type parameters such as "; charset=utf-8" that browsers
	// or HTTP clients may append; they would otherwise break the signature.
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	// "image/jpg" is not a registered MIME type; browsers emit "image/jpeg".
	// Canonicalize it so the signed value matches what the browser will send.
	if ct == "image/jpg" {
		ct = "image/jpeg"
	}
	ext, ok := allowedImageContentTypes[ct]
	if !ok {
		return "", "", fmt.Errorf("unsupported content type: %s", ct)
	}
	return ct, ext, nil
}

// buildObjectKey produces a collision-resistant object key under a folder.
// The original filename is only used to derive a slug; the actual file
// extension is determined by the validated content type to avoid spoofing.
func buildObjectKey(folder, filename, ext string) (string, error) {
	folder = sanitizeFolder(folder)
	base := slugifyBase(filename)
	if base == "" {
		base = "file"
	}

	suffix, err := randomHex(8)
	if err != nil {
		return "", err
	}

	stamp := time.Now().UTC().Format("20060102")
	name := fmt.Sprintf("%s-%s-%s.%s", stamp, base, suffix, ext)
	return path.Join(folder, name), nil
}

func sanitizeFolder(f string) string {
	f = strings.Trim(f, "/ ")
	if f == "" {
		return "uploads"
	}
	// Only allow simple alphanumeric segments separated by "/".
	parts := strings.Split(f, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := slugifyBase(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return "uploads"
	}
	return strings.Join(out, "/")
}

// slugifyBase strips the extension and keeps only [a-z0-9-] characters.
func slugifyBase(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	var b strings.Builder
	b.Grow(len(name))
	prevDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == ' ':
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomHex(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", errors.New("invalid random size")
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
