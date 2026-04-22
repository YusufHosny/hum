package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrInvalidPacketSize = errors.New("packet is too small to contain a nonce")
)

type Cryptor struct {
	aead cipher.AEAD
}

func NewCryptor(channel string, passkey string) (*Cryptor, error) {
	const (
		time    = 1         // Number of iterations
		memory  = 64 * 1024 // 64 MB
		threads = 4         // Number of concurrent threads
		keyLen  = 32        // 32 bytes (256 bits) for XChaCha20
	)

	salt := []byte(channel)
	if len(salt) == 0 {
		return nil, errors.New("channel name (salt) cannot be empty")
	}

	key := argon2.IDKey([]byte(passkey), salt, time, memory, uint8(threads), keyLen)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &Cryptor{
		aead: aead,
	}, nil
}

// output format: [24-byte nonce][ciphertext + MAC]
func (c *Cryptor) Encrypt(packet []byte, nonce []byte) ([]byte, error) {
	var plaintext []byte
	var buffer []byte
	if nonce == nil {
		nonce = make([]byte, c.aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return nil, fmt.Errorf("failed to generate random nonce: %w", err)
		}

		plaintext = make([]byte, len(nonce), len(nonce)+len(packet)+c.aead.Overhead())
		copy(plaintext, nonce)
		buffer = plaintext[:len(nonce)]

	} else if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("Invalid Nonce Length")
	} else {
		buffer = make([]byte, 0, len(packet)+c.aead.Overhead())
	}

	ciphertext := c.aead.Seal(buffer, nonce, packet, nil)

	return ciphertext, nil
}

func (c *Cryptor) Decrypt(packet []byte, nonce []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()

	if len(packet) < nonceSize {
		return nil, ErrInvalidPacketSize
	}

	ciphertext := packet
	if nonce == nil {
		nonce = packet[:nonceSize]
		ciphertext = packet[nonceSize:]
	} else if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("Invalid Nonce Length")
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt packet: %w", err)
	}

	return plaintext, nil
}
