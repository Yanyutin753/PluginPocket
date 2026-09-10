package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func publicRequest(h interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "http://example.com"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDeviceAuthorizationRequiresExplicitApprovalAndIsOneTime(t *testing.T) {
	s, h := setup(t)
	user := register(t, h, "alice")
	ctx := context.Background()
	w := publicRequest(h, "/api/v1/device/authorize", `{}`)
	if w.Code != 200 {
		t.Fatalf("authorize without Origin %d %s", w.Code, w.Body)
	}
	var codes struct {
		Device string `json:"device_code"`
		User   string `json:"user_code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &codes)
	body, _ := json.Marshal(map[string]string{"device_code": codes.Device})
	w = publicRequest(h, "/api/v1/device/token", string(body))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "authorization_pending") {
		t.Fatalf("pending %d %s", w.Code, w.Body)
	}
	w = publicRequest(h, "/api/v1/device/token", string(body))
	if w.Code != 429 || !strings.Contains(w.Body.String(), "slow_down") {
		t.Fatalf("poll throttle %d %s", w.Code, w.Body)
	}
	approval, _ := json.Marshal(map[string]string{"user_code": codes.User})
	if w = request(h, "POST", "/api/v1/account/devices/approve", string(approval), nil); w.Code != 401 {
		t.Fatalf("anonymous approval %d", w.Code)
	}
	if w = request(h, "POST", "/api/v1/account/devices/approve", string(approval), user); w.Code != 204 {
		t.Fatalf("approve %d %s", w.Code, w.Body)
	}
	_, e := s.Pool.Exec(ctx, "UPDATE device_authorizations SET next_poll_at=now()-interval '1 second'")
	if e != nil {
		t.Fatal(e)
	}
	w = publicRequest(h, "/api/v1/device/token", string(body))
	if w.Code != 200 {
		t.Fatalf("authorized poll %d %s", w.Code, w.Body)
	}
	var token struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &token)
	p, e := s.AuthToken(ctx, token.Token)
	if e != nil || p.Username != "alice" {
		t.Fatalf("authorized principal %#v %v", p, e)
	}
	w = publicRequest(h, "/api/v1/device/token", string(body))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_grant") {
		t.Fatalf("device replay %d %s", w.Code, w.Body)
	}
}
func TestDeviceExpirationAndApprovalCannotBeRebound(t *testing.T) {
	s, h := setup(t)
	a := register(t, h, "alice")
	b := register(t, h, "bob")
	w := publicRequest(h, "/api/v1/device/authorize", `{}`)
	if w.Code != 200 {
		t.Fatalf("authorize %d", w.Code)
	}
	var codes struct {
		Device string `json:"device_code"`
		User   string `json:"user_code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &codes)
	approval, _ := json.Marshal(map[string]string{"user_code": codes.User})
	if w = request(h, "POST", "/api/v1/account/devices/approve", string(approval), a); w.Code != 204 {
		t.Fatalf("approve %d", w.Code)
	}
	if w = request(h, "POST", "/api/v1/account/devices/approve", string(approval), b); w.Code != 409 {
		t.Fatalf("rebind %d", w.Code)
	}
	_, _ = s.Pool.Exec(context.Background(), "UPDATE device_authorizations SET expires_at=now()-interval '1 second'")
	body, _ := json.Marshal(map[string]string{"device_code": codes.Device})
	w = publicRequest(h, "/api/v1/device/token", string(body))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "expired_token") {
		t.Fatalf("expired %d %s", w.Code, w.Body)
	}
}
