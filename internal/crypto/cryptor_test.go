package crypto

import (
	"bytes"
	"testing"
)

func TestNewCryptor(t *testing.T) {
	_, err := NewCryptor("mychannel", "mypasskey")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = NewCryptor("", "mypasskey")
	if err == nil {
		t.Fatal("expected error for empty channel, got nil")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	cryptor, err := NewCryptor("testchannel", "supersecret")
	if err != nil {
		t.Fatalf("failed to create cryptor: %v", err)
	}

	plaintext := []byte("hello world this is a test payload")
	ciphertext, err := cryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if len(ciphertext) <= len(plaintext) {
		t.Fatalf("ciphertext too short: %d", len(ciphertext))
	}

	decrypted, err := cryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text does not match original plaintext. got: %s, want: %s", string(decrypted), string(plaintext))
	}
}

func TestDecryptInvalidPacketSize(t *testing.T) {
	cryptor, _ := NewCryptor("testchannel", "supersecret")

	// XChaCha20-Poly1305 nonce size is 24 bytes
	shortPacket := make([]byte, 10)
	_, err := cryptor.Decrypt(shortPacket)

	if err != ErrInvalidPacketSize {
		t.Fatalf("expected ErrInvalidPacketSize, got: %v", err)
	}
}

func TestDecryptInvalidKey(t *testing.T) {
	cryptor1, _ := NewCryptor("testchannel", "secret1")
	cryptor2, _ := NewCryptor("testchannel", "secret2")

	plaintext := []byte("sensitive data")
	ciphertext, _ := cryptor1.Encrypt(plaintext)

	_, err := cryptor2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected decryption to fail with incorrect key, but it succeeded")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	cryptor, _ := NewCryptor("testchannel", "secret")

	plaintext := []byte("important message")
	ciphertext, _ := cryptor.Encrypt(plaintext)

	// Tamper with the ciphertext (last byte is part of the MAC or ciphertext)
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := cryptor.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected decryption to fail for tampered ciphertext, but it succeeded")
	}
}
