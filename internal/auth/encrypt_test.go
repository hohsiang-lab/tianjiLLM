package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	masterKey := "test-master-key-123"
	plaintext := "hello world"

	encrypted, err := Encrypt(plaintext, masterKey)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := Decrypt(encrypted, masterKey)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecrypt_WrongKey(t *testing.T) {
	encrypted, err := Encrypt("secret", "key1")
	require.NoError(t, err)

	_, err = Decrypt(encrypted, "key2")
	assert.Error(t, err)
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!", "key")
	assert.Error(t, err)
}

func TestDecrypt_TooShort(t *testing.T) {
	// base64 of less than 24 bytes
	_, err := Decrypt("AAAA", "key")
	assert.Error(t, err)
}

func TestDeriveKey_Deterministic(t *testing.T) {
	k1 := DeriveKey("test")
	k2 := DeriveKey("test")
	assert.Equal(t, k1, k2)
}

func TestDeriveKey_DifferentInputs(t *testing.T) {
	k1 := DeriveKey("key1")
	k2 := DeriveKey("key2")
	assert.NotEqual(t, k1, k2)
}

func TestEncrypt_DifferentEachTime(t *testing.T) {
	e1, _ := Encrypt("same", "key")
	e2, _ := Encrypt("same", "key")
	assert.NotEqual(t, e1, e2) // random nonce
}
