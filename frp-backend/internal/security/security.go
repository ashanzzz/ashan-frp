package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const saltSize = 16

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil { return "", err }
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, ":")
	if len(parts) != 2 { return false }
	salt, err := hex.DecodeString(parts[0]); if err != nil { return false }
	expected, err := hex.DecodeString(parts[1]); if err != nil { return false }
	actual := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "tok_" + hex.EncodeToString(sum[:])
}

// DeriveEncryptionKey derives a 32-byte AES-256 key from an arbitrary-length
// passphrase using SHA-256. If the passphrase is already 32 bytes, it is used
// as-is; otherwise it is hashed.
func DeriveEncryptionKey(passphrase string) []byte {
	if len(passphrase) == 32 {
		return []byte(passphrase)
	}
	sum := sha256.Sum256([]byte(passphrase))
	return sum[:]
}

// Encrypt encrypts plaintext with AES-256-GCM using the given key.
// Returns hex(nonce || ciphertext).
// The key should be 32 bytes — use DeriveEncryptionKey if it isn't.
func Encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded AES-256-GCM payload produced by Encrypt.
func Decrypt(encoded string, key []byte) ([]byte, error) {
	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}