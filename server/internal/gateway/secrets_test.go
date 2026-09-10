package gateway

import (
	"bytes"
	"testing"
)

func TestEncryptedUpstreamConfig(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	raw := []byte(`{"headers":{"Authorization":"Bearer sensitive"}}`)
	a, err := SealConfig(key, raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SealConfig(key, raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) || bytes.Contains(a, []byte("sensitive")) {
		t.Fatal("encryption must conceal secrets with unique nonces")
	}
	actual, err := OpenConfig(key, a)
	if err != nil || !bytes.Equal(raw, actual) {
		t.Fatalf("roundtrip %s %v", actual, err)
	}
	if _, err := OpenConfig(bytes.Repeat([]byte{8}, 32), a); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := SealConfig(nil, raw); err == nil {
		t.Fatal("missing key accepted")
	}
}
