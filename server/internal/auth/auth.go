package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/scrypt"
)

var ErrBusy = errors.New("auth_busy")
var passwordSlots = make(chan struct{}, 4)

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password_length")
	}
	select {
	case passwordSlots <- struct{}{}:
		defer func() { <-passwordSlots }()
	default:
		return "", ErrBusy
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return "", err
	}
	return "scrypt$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(key), nil
}

func CheckPassword(hash, password string) bool { ok, _ := VerifyPassword(hash, password); return ok }
func VerifyPassword(hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "scrypt" || len(password) > 1024 {
		return false, nil
	}
	salt, e1 := hex.DecodeString(parts[1])
	want, e2 := hex.DecodeString(parts[2])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(want) != 32 {
		return false, nil
	}
	select {
	case passwordSlots <- struct{}{}:
		defer func() { <-passwordSlots }()
	default:
		return false, ErrBusy
	}
	got, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1, err
}

func Secret(prefix string) (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(data), nil
}

func Digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
