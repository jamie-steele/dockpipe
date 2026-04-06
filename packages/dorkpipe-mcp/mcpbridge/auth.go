package mcpbridge

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const (
	headerAuthorization = "Authorization"
	headerXAPIKey       = "X-API-Key"
)

// ExpectedAPIKeyFromRequest extracts a bearer token or X-API-Key for comparison.
// Accepts: Authorization: Bearer <token>, Authorization: ApiKey <token>, X-API-Key: <token>
func ExpectedAPIKeyFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get(headerXAPIKey)); v != "" {
		return v
	}
	auth := strings.TrimSpace(r.Header.Get(headerAuthorization))
	if auth == "" {
		return ""
	}
	lower := strings.ToLower(auth)
	if strings.HasPrefix(lower, "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if strings.HasPrefix(lower, "apikey ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

// ConstantTimeEqualString compares two API keys in constant time when lengths match.
func ConstantTimeEqualString(a, b string) bool {
	if len(a) != len(b) {
		// Still run a dummy compare to reduce timing on length (best-effort).
		if len(a) > 0 && len(b) > 0 {
			_ = subtle.ConstantTimeCompare([]byte(a[:1]), []byte(b[:1]))
		}
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
