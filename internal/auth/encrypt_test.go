package auth

import (
	"crypto/sha256"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	key := DeriveKey("test-master-key")
	expected := sha256.Sum256([]byte("test-master-key"))
	if key != expected {
		t.Fatalf("DeriveKey mismatch")
	}

	// Different keys produce different results
	key2 := DeriveKey("other-key")
	if key == key2 {
		t.Fatal("different inputs should produce different keys")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	masterKey := "my-secret-master-key"
	plaintext := "hello world 你好世界"

	ct, err := Encrypt(plaintext, masterKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(ct, masterKey)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if got != plaintext {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	mk := "key"
	ct1, _ := Encrypt("same", mk)
	ct2, _ := Encrypt("same", mk)
	if ct1 == ct2 {
		t.Fatal("two encryptions of the same plaintext should differ (random nonce)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	ct, _ := Encrypt("secret", "key1")
	_, err := Decrypt(ct, "key2")
	if err == nil {
		t.Fatal("expected decryption failure with wrong key")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!", "key")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptTooShort(t *testing.T) {
	// Valid base64 but too short for nonce
	_, err := Decrypt("AQID", "key")
	if err == nil || err.Error() != "ciphertext too short" {
		t.Fatalf("expected 'ciphertext too short', got %v", err)
	}
}

func TestEncryptEmpty(t *testing.T) {
	mk := "key"
	ct, err := Encrypt("", mk)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	got, err := Decrypt(ct, mk)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
