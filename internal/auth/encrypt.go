package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

// DeriveKey derives a 32-byte NaCl key from the master key using SHA256,
// matching Python LiteLLM's encrypt_value/decrypt_value.
func DeriveKey(masterKey string) [32]byte {
	return sha256.Sum256([]byte(masterKey))
}

// Encrypt encrypts plaintext using NaCl SecretBox with a random nonce.
// Returns base64url-encoded ciphertext (nonce prepended).
func Encrypt(plaintext string, masterKey string) (string, error) {
	key := DeriveKey(masterKey)

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", err
	}

	encrypted := secretbox.Seal(nonce[:], []byte(plaintext), &nonce, &key)
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

// Decrypt decrypts a base64url-encoded NaCl SecretBox ciphertext.
func Decrypt(ciphertext string, masterKey string) (string, error) {
	key := DeriveKey(masterKey)

	decoded, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	if len(decoded) < 24 {
		return "", errors.New("ciphertext too short")
	}

	var nonce [24]byte
	copy(nonce[:], decoded[:24])

	plaintext, ok := secretbox.Open(nil, decoded[24:], &nonce, &key)
	if !ok {
		return "", errors.New("decryption failed")
	}

	return string(plaintext), nil
}
