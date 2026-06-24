package authdiag

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

type tokenClaims struct {
	ExpiresAt int64 `json:"exp"`
	IssuedAt  int64 `json:"iat"`
}

// Fingerprint returns a short, irreversible identifier suitable for
// correlating token issuance and verification without logging credentials.
func Fingerprint(value string) string {
	if value == "" {
		return "empty"
	}

	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:6])
}

func tokenFingerprint(token string) string {
	if token == "" {
		return "missing"
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(token))
	return fmt.Sprintf("%08x", hash.Sum32())
}

// TokenSummary describes a JWT without exposing any part of the token itself.
func TokenSummary(token string) string {
	parts := strings.Split(token, ".")
	claims := tokenClaims{}
	if len(parts) == 3 {
		if payload, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			_ = json.Unmarshal(payload, &claims)
		}
	}

	return fmt.Sprintf(
		"fingerprint=%s length=%d segments=%d iat=%d exp=%d now=%d",
		tokenFingerprint(token),
		len(token),
		len(parts),
		claims.IssuedAt,
		claims.ExpiresAt,
		time.Now().Unix(),
	)
}
