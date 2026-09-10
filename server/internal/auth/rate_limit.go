package auth

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"net/http"
)

type requestLimiter interface {
	AllowRequest(context.Context, string, int64, int64, int64) (bool, error)
}

// AllowPublicRequest isolates authentication families and peers while retaining a
// shared deployment ceiling. Forwarded headers are not trusted as peer identity.
func AllowPublicRequest(ctx context.Context, s requestLimiter, r *http.Request, family string) (bool, error) {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if ip := net.ParseIP(peer); ip != nil {
		peer = ip.String()
	}
	digest := sha256.Sum256([]byte(peer))
	subject := int64(binary.BigEndian.Uint64(digest[:8]) & ((1 << 63) - 1))
	allowed, err := s.AllowRequest(ctx, "public:"+family, subject, 60, 120)
	if err != nil || !allowed {
		return allowed, err
	}
	return s.AllowRequest(ctx, "public_global:"+family, 0, 60, 1200)
}
