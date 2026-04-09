package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func decodeSyncKey(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil {
		return nil, fmt.Errorf("decode sync key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("decode sync key: expected 32 bytes, got %d", len(key))
	}
	return key, nil
}

func encryptOAuthKey(key []byte, token *OAuthKey) (string, error) {
	if token == nil {
		return "", fmt.Errorf("encrypt oauth key: token is nil")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	plaintext, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("marshal oauth key: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptOAuthKey(key []byte, tokenData string) (*OAuthKey, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tokenData))
	if err != nil {
		return nil, fmt.Errorf("decode token data: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("decode token data: too short")
	}

	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt oauth key: %w", err)
	}

	var token OAuthKey
	if err := json.Unmarshal(plaintext, &token); err != nil {
		return nil, fmt.Errorf("unmarshal oauth key: %w", err)
	}
	return &token, nil
}
