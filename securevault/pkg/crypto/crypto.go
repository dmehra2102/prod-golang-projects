package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keyLen    = 32
	saltLen   = 16
	nonceLen  = 12
	iterCount = 100_000
)

type Encryptor struct {
	key []byte
}

func NewEncryptor(masterKey string) (*Encryptor, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("crypto: master key must not be empty")
	}

	saltSource := sha256.Sum256([]byte("securevault:field-enc:v1"))
	derived := pbkdf2.Key([]byte(masterKey), saltSource[:saltLen], iterCount, keyLen, sha256.New)
	return &Encryptor{key: derived}, nil
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value previously produced by Encrypt.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}
	if len(data) < nonceLen {
		return "", errors.New("crypto: ciphertext too short")
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonce, ciphertext := data[:nonceLen], data[nonceLen:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plaintext), nil
}

func Hash(value string) string {
	h := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", h)
}
