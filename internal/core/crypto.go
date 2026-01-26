// Package core provides cryptographic and storage primitives for API key management.
package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/fernet/fernet-go"
	"github.com/zalando/go-keyring"
)

const (
	// ServiceName is the keyring service name for storing the master key.
	ServiceName = "apikey-manager"
	// MasterKeyAccount is the keyring account name for the master key.
	MasterKeyAccount = "master_key"
)

// KeyEncryption handles encryption/decryption with Fernet and system keychain.
type KeyEncryption struct {
	masterKey *fernet.Key
	mu        sync.RWMutex
}

var (
	cryptoInstance *KeyEncryption
	cryptoOnce     sync.Once
)

// GetCrypto returns the singleton KeyEncryption instance.
func GetCrypto() (*KeyEncryption, error) {
	var initErr error
	cryptoOnce.Do(func() {
		cryptoInstance = &KeyEncryption{}
		initErr = cryptoInstance.Initialize()
	})
	if initErr != nil {
		return nil, initErr
	}
	return cryptoInstance, nil
}

// Initialize loads or generates the master key from system keychain.
func (k *KeyEncryption) Initialize() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Try to get master key from keychain
	masterKeyB64, err := keyring.Get(ServiceName, MasterKeyAccount)
	if err == nil && masterKeyB64 != "" {
		// Decode and parse existing key
		keyBytes, err := base64.StdEncoding.DecodeString(masterKeyB64)
		if err != nil {
			return fmt.Errorf("failed to decode master key: %w", err)
		}
		key, err := fernet.DecodeKey(string(keyBytes))
		if err != nil {
			return fmt.Errorf("failed to parse master key: %w", err)
		}
		k.masterKey = key
		return nil
	}

	// Generate new master key
	key := fernet.Key{}
	if err := key.Generate(); err != nil {
		return fmt.Errorf("failed to generate master key: %w", err)
	}

	// Store in keychain (base64 of the key bytes)
	keyStr := key.Encode()
	masterKeyB64 = base64.StdEncoding.EncodeToString([]byte(keyStr))
	if err := keyring.Set(ServiceName, MasterKeyAccount, masterKeyB64); err != nil {
		return fmt.Errorf("failed to store master key in keychain: %w", err)
	}

	k.masterKey = &key
	return nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
func (k *KeyEncryption) Encrypt(plaintext string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.masterKey == nil {
		return "", fmt.Errorf("encryption system not initialized")
	}

	ciphertext, err := fernet.EncryptAndSign([]byte(plaintext), k.masterKey)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext.
func (k *KeyEncryption) Decrypt(encrypted string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.masterKey == nil {
		return "", fmt.Errorf("encryption system not initialized")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	plaintext := fernet.VerifyAndDecrypt(ciphertext, 0, []*fernet.Key{k.masterKey})
	if plaintext == nil {
		return "", fmt.Errorf("decryption failed: invalid token or key")
	}

	return string(plaintext), nil
}

// SignMessage creates an HMAC-SHA256 signature of the message.
func (k *KeyEncryption) SignMessage(message string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.masterKey == nil {
		return "", fmt.Errorf("encryption system not initialized")
	}

	// Use the raw key bytes for HMAC
	keyBytes := []byte(k.masterKey.Encode())
	h := hmac.New(sha256.New, keyBytes)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySignature verifies an HMAC-SHA256 signature.
func (k *KeyEncryption) VerifySignature(message, signature string) (bool, error) {
	expected, err := k.SignMessage(message)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(expected), []byte(signature)), nil
}

// ResetMasterKey deletes the master key from keychain (dangerous operation).
func (k *KeyEncryption) ResetMasterKey() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := keyring.Delete(ServiceName, MasterKeyAccount); err != nil {
		return fmt.Errorf("failed to delete master key: %w", err)
	}
	k.masterKey = nil
	return nil
}
