package gateway

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
)

func configCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("upstream encryption requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}
func SealConfig(key, raw []byte) ([]byte, error) {
	aead, err := configCipher(key)
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, errors.New("invalid upstream config")
	}
	sealed := aead.Seal(nil, nil, raw, []byte("pluginpocket-upstream-v1"))
	return json.Marshal(map[string]string{"ciphertext": base64.StdEncoding.EncodeToString(sealed)})
}
func OpenConfig(key, sealed []byte) ([]byte, error) {
	aead, err := configCipher(key)
	if err != nil {
		return nil, err
	}
	var config struct {
		Ciphertext string `json:"ciphertext"`
	}
	if err = json.Unmarshal(sealed, &config); err != nil {
		return nil, errors.New("invalid encrypted config")
	}
	raw, err := base64.StdEncoding.DecodeString(config.Ciphertext)
	if err != nil {
		return nil, errors.New("invalid encrypted config")
	}
	plain, err := aead.Open(nil, nil, raw, []byte("pluginpocket-upstream-v1"))
	if err != nil {
		return nil, errors.New("upstream config could not be decrypted")
	}
	return plain, nil
}
