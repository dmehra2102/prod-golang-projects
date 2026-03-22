package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type Encryptor struct {
	kmsClient    *kms.Client
	kmsKeyID     string
	localKey     []byte
	mu           sync.RWMutex
	dataKeyCache map[string]*dataKeyEntry
}

type dataKeyEntry struct {
	plaintext  []byte
	ciphertext []byte
}

func NewEncryptor(ctx context.Context, region, kmsKeyID string) (*Encryptor, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Encryptor{
		kmsClient:    kms.NewFromConfig(cfg),
		kmsKeyID:     kmsKeyID,
		dataKeyCache: make(map[string]*dataKeyEntry),
	}, nil
}

// NewLocalEncryptor creates an encryptor with a local key (for development).
func NewLocalEncryptor(key string) (*Encryptor, error) {
	hash := sha256.Sum256([]byte(key))
	return &Encryptor{
		localKey:     hash[:],
		dataKeyCache: make(map[string]*dataKeyEntry),
	}, nil
}

func (e *Encryptor) GenerateDataKey(ctx context.Context, keyContext string) (plaintext, ciphertext []byte, err error) {
	if e.kmsClient == nil {
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, nil, fmt.Errorf("failed to generate random key: %w", err)
		}
		return key, key, nil
	}

	encCtx := map[string]string{"context": keyContext}

	output, err := e.kmsClient.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:             aws.String(e.kmsKeyID),
		KeySpec:           kmstypes.DataKeySpecAes256,
		EncryptionContext: encCtx,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate data key: %w", err)
	}

	return output.Plaintext, output.CiphertextBlob, nil
}

// Encrypt encrypts data using AES-256-GCM.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	key := e.localKey
	if key == nil {
		return "", fmt.Errorf("encryption key not set; use envelope encryption for KMS mode")
	}
	return encryptAESGCM(key, plaintext)
}

// Decrypt decrypts data using AES-256-GCM.
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error) {
	key := e.localKey
	if key == nil {
		return nil, fmt.Errorf("encryption key not set; use envelope decryption for KMS mode")
	}
	return decryptAESGCM(key, ciphertext)
}

func (e *Encryptor) EnvelopeEncrypt(ctx context.Context, plaintext []byte, keyContext string) (*EnvelopeData, error) {
	dataKeyPlain, dataKeyCipher, err := e.GenerateDataKey(ctx, keyContext)
	if err != nil {
		return nil, err
	}

	encrypted, err := encryptAESGCM(dataKeyPlain, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Zero out plaintext data key from memory
	for i := range dataKeyPlain {
		dataKeyPlain[i] = 0
	}

	return &EnvelopeData{
		EncryptedDataKey: base64.StdEncoding.EncodeToString(dataKeyCipher),
		Ciphertext:       encrypted,
		KeyContext:       keyContext,
	}, nil
}

// EnvelopeDecrypt decrypts envelope-encrypted data.
func (e *Encryptor) EnvelopeDecrypt(ctx context.Context, data *EnvelopeData) ([]byte, error) {
	encDataKey, err := base64.StdEncoding.DecodeString(data.EncryptedDataKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted data key: %w", err)
	}

	var dataKeyPlain []byte

	if e.kmsClient != nil {
		encCtx := map[string]string{"context": data.KeyContext}
		output, err := e.kmsClient.Decrypt(ctx, &kms.DecryptInput{
			CiphertextBlob:    encDataKey,
			EncryptionContext: encCtx,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt data key: %w", err)
		}
		dataKeyPlain = output.Plaintext
	} else {
		dataKeyPlain = encDataKey // Local mode
	}

	plaintext, err := decryptAESGCM(dataKeyPlain, data.Ciphertext)

	// Zero out plaintext data key
	for i := range dataKeyPlain {
		dataKeyPlain[i] = 0
	}

	return plaintext, err
}

type EnvelopeData struct {
	EncryptedDataKey string `json:"encrypted_data_key"`
	Ciphertext       string `json:"ciphertext"`
	KeyContext       string `json:"key_context"`
}

func HashSensitiveData(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func encryptAESGCM(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAESGCM(key []byte, encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
