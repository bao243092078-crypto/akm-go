package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/baobao/akm-go/internal/models"
)

var validKeyNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateKeyName checks if a key name is a valid environment variable name.
func ValidateKeyName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	return validKeyNamePattern.MatchString(name)
}

// EscapeDotenvValue escapes a value for .env file format.
func EscapeDotenvValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}

// KeyStorage manages encrypted API key storage.
type KeyStorage struct {
	dataDir   string
	keysFile  string
	auditFile string
	crypto    *KeyEncryption

	keysCache  map[string]*models.APIKey
	loadFailed bool
	mu         sync.RWMutex
}

var (
	storageInstance *KeyStorage
	storageOnce     sync.Once
)

// GetStorage returns the singleton KeyStorage instance.
func GetStorage() (*KeyStorage, error) {
	var initErr error
	storageOnce.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			initErr = err
			return
		}
		dataDir := filepath.Join(homeDir, ".apikey-manager", "data")
		storageInstance, initErr = NewKeyStorage(dataDir)
	})
	if initErr != nil {
		return nil, initErr
	}
	return storageInstance, nil
}

// NewKeyStorage creates a new KeyStorage with the specified data directory.
func NewKeyStorage(dataDir string) (*KeyStorage, error) {
	// Create data directory with restricted permissions
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	crypto, err := GetCrypto()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crypto: %w", err)
	}

	s := &KeyStorage{
		dataDir:   dataDir,
		keysFile:  filepath.Join(dataDir, "keys.json"),
		auditFile: filepath.Join(dataDir, "audit.jsonl"),
		crypto:    crypto,
		keysCache: make(map[string]*models.APIKey),
	}

	if err := s.loadKeys(); err != nil {
		// Log warning but don't fail - empty cache is acceptable
		fmt.Fprintf(os.Stderr, "⚠️  加载密钥失败: %v\n", err)
		s.loadFailed = true
	}

	return s, nil
}

// loadKeys loads keys from the encrypted JSON file.
func (s *KeyStorage) loadKeys() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.keysFile)
	if os.IsNotExist(err) {
		return nil // Empty storage is OK
	}
	if err != nil {
		return err
	}

	// Try to parse as unencrypted JSON first (legacy format)
	var keysFile models.KeysFile
	if err := json.Unmarshal(data, &keysFile); err == nil && keysFile.Version != "" {
		fmt.Fprintf(os.Stderr, "⚠️  检测到旧格式文件，将在保存时自动升级为加密格式\n")
		for _, key := range keysFile.Keys {
			s.keysCache[key.Name] = key
		}
		return nil
	}

	// New format: entire file is encrypted
	decryptedJSON, err := s.crypto.Decrypt(string(data))
	if err != nil {
		return fmt.Errorf("failed to decrypt keys file: %w", err)
	}

	if err := json.Unmarshal([]byte(decryptedJSON), &keysFile); err != nil {
		return fmt.Errorf("failed to parse keys JSON: %w", err)
	}

	for _, key := range keysFile.Keys {
		s.keysCache[key.Name] = key
	}

	return nil
}

// saveKeys saves keys to the encrypted JSON file with atomic write.
func (s *KeyStorage) saveKeys() error {
	if s.loadFailed {
		return fmt.Errorf("refusing to save: keys file failed to load, saving may cause data loss")
	}

	// Build keys list
	keys := make([]*models.APIKey, 0, len(s.keysCache))
	for _, key := range s.keysCache {
		keys = append(keys, key)
	}

	keysFile := models.KeysFile{
		Version:   "2.0",
		UpdatedAt: time.Now().Format(time.RFC3339),
		Keys:      keys,
	}

	jsonBytes, err := json.MarshalIndent(keysFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keys: %w", err)
	}

	encrypted, err := s.crypto.Encrypt(string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to encrypt keys: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempFile := filepath.Join(s.dataDir, ".keys_temp.json")
	if err := os.WriteFile(tempFile, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, s.keysFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AddKey adds a new API key.
func (s *KeyStorage) AddKey(name, value, provider string, opts ...KeyOption) (*models.APIKey, error) {
	if !ValidateKeyName(name) {
		return nil, fmt.Errorf("invalid key name '%s': must start with letter or underscore, contain only alphanumerics and underscores, max 256 chars", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt the value
	encrypted, err := s.crypto.Encrypt(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt key value: %w", err)
	}

	key := models.NewAPIKey(name, encrypted, provider)

	// Apply options
	for _, opt := range opts {
		opt(key)
	}

	s.keysCache[name] = key

	if err := s.saveKeys(); err != nil {
		delete(s.keysCache, name) // Rollback on failure
		return nil, err
	}

	// Audit log
	s.logUsage(name, "add", "system")

	return key, nil
}

// KeyOption is a functional option for configuring a key.
type KeyOption func(*models.APIKey)

// WithDescription sets the key description.
func WithDescription(desc string) KeyOption {
	return func(k *models.APIKey) {
		k.Description = &desc
	}
}

// WithSourceProject sets the source project.
func WithSourceProject(project string) KeyOption {
	return func(k *models.APIKey) {
		k.SourceProject = &project
	}
}

// WithTags sets the key tags.
func WithTags(tags []string) KeyOption {
	return func(k *models.APIKey) {
		k.Tags = tags
	}
}

// GetKey returns the key metadata (not decrypted value).
func (s *KeyStorage) GetKey(name string) *models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keysCache[name]
}

// GetKeyValue returns the decrypted key value.
func (s *KeyStorage) GetKeyValue(name, project string) (string, error) {
	s.mu.RLock()
	key := s.keysCache[name]
	s.mu.RUnlock()

	if key == nil {
		return "", fmt.Errorf("key '%s' not found", name)
	}

	value, err := s.crypto.Decrypt(key.ValueEncrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt key '%s': %w", name, err)
	}

	s.logUsage(name, "read", project)
	return value, nil
}

// ListKeys returns all keys, optionally filtered by provider.
func (s *KeyStorage) ListKeys(provider string) []*models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*models.APIKey, 0, len(s.keysCache))
	for _, key := range s.keysCache {
		if provider == "" || key.Provider == provider {
			keys = append(keys, key)
		}
	}
	return keys
}

// SearchKeys searches keys by query string.
func (s *KeyStorage) SearchKeys(query string) []*models.APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*models.APIKey

	for _, key := range s.keysCache {
		if strings.Contains(strings.ToLower(key.Name), queryLower) ||
			strings.Contains(strings.ToLower(key.Provider), queryLower) ||
			(key.Description != nil && strings.Contains(strings.ToLower(*key.Description), queryLower)) ||
			(key.SourceProject != nil && strings.Contains(strings.ToLower(*key.SourceProject), queryLower)) {
			results = append(results, key)
		}
	}
	return results
}

// UpdateKey updates key metadata (not the value).
func (s *KeyStorage) UpdateKey(name string, updates map[string]interface{}) (*models.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.keysCache[name]
	if key == nil {
		return nil, fmt.Errorf("key '%s' not found", name)
	}

	// Apply updates
	if v, ok := updates["provider"].(string); ok {
		key.Provider = v
	}
	if v, ok := updates["description"].(string); ok {
		key.Description = &v
	}
	if v, ok := updates["source_project"].(string); ok {
		key.SourceProject = &v
	}
	if v, ok := updates["tags"].([]string); ok {
		key.Tags = v
	}
	if v, ok := updates["is_active"].(bool); ok {
		key.IsActive = v
	}

	key.UpdatedAt = models.FlexTime{Time: time.Now()}

	if err := s.saveKeys(); err != nil {
		return nil, err
	}

	s.logUsage(name, "update", "system")
	return key, nil
}

// DeleteKey removes a key.
func (s *KeyStorage) DeleteKey(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.keysCache[name]; !exists {
		return fmt.Errorf("key '%s' not found", name)
	}

	delete(s.keysCache, name)

	if err := s.saveKeys(); err != nil {
		return err
	}

	s.logUsage(name, "delete", "system")
	return nil
}

// GetKeysForInjection returns decrypted keys for injection.
func (s *KeyStorage) GetKeysForInjection(project, provider string, keyNames []string) (map[string]string, error) {
	return s.getKeysBatch(project, provider, keyNames, "inject")
}

// GetKeysForExport returns decrypted keys for export.
func (s *KeyStorage) GetKeysForExport(project, provider string, keyNames []string) (map[string]string, error) {
	return s.getKeysBatch(project, provider, keyNames, "export")
}

func (s *KeyStorage) getKeysBatch(project, provider string, keyNames []string, action string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyNamesSet := make(map[string]bool)
	for _, name := range keyNames {
		keyNamesSet[name] = true
	}

	result := make(map[string]string)
	for _, key := range s.keysCache {
		// Filter by provider
		if provider != "" && key.Provider != provider {
			continue
		}
		// Filter by name list
		if len(keyNames) > 0 && !keyNamesSet[key.Name] {
			continue
		}

		value, err := s.crypto.Decrypt(key.ValueEncrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt key '%s': %w", key.Name, err)
		}
		result[key.Name] = value
		s.logUsage(key.Name, action, project)
	}

	return result, nil
}

// AuditErrors tracks audit log write failures.
var AuditErrors int64

// logUsage writes an audit log entry.
func (s *KeyStorage) logUsage(keyName, action, project string) {
	log := models.NewKeyUsageLog(keyName, project, action)

	// Sign the log entry
	logJSON, _ := json.Marshal(struct {
		KeyName   string `json:"key_name"`
		Project   string `json:"project"`
		Action    string `json:"action"`
		Timestamp string `json:"timestamp"`
	}{
		KeyName:   log.KeyName,
		Project:   log.Project,
		Action:    log.Action,
		Timestamp: log.Timestamp.Format(time.RFC3339Nano),
	})

	signature, _ := s.crypto.SignMessage(string(logJSON))
	log.Signature = &signature

	// Append to audit file
	f, err := os.OpenFile(s.auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		AuditErrors++
		fmt.Fprintf(os.Stderr, "⚠️  审计日志写入失败 (累计 %d 次): %v\n", AuditErrors, err)
		return
	}
	defer f.Close()

	logBytes, _ := json.Marshal(log)
	if _, err := f.Write(logBytes); err != nil {
		AuditErrors++
		fmt.Fprintf(os.Stderr, "⚠️  审计日志写入失败 (累计 %d 次): %v\n", AuditErrors, err)
		return
	}
	f.WriteString("\n")
}

// VerifyAuditLogs verifies the integrity of audit logs.
func (s *KeyStorage) VerifyAuditLogs() (total, verified, unsigned, tampered int, err error) {
	data, err := os.ReadFile(s.auditFile)
	if os.IsNotExist(err) {
		return 0, 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		total++

		var log models.KeyUsageLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			tampered++
			continue
		}

		if log.Signature == nil || *log.Signature == "" {
			unsigned++
			continue
		}

		// Verify signature
		logJSON, _ := json.Marshal(struct {
			KeyName   string `json:"key_name"`
			Project   string `json:"project"`
			Action    string `json:"action"`
			Timestamp string `json:"timestamp"`
		}{
			KeyName:   log.KeyName,
			Project:   log.Project,
			Action:    log.Action,
			Timestamp: log.Timestamp.Format(time.RFC3339Nano),
		})

		valid, _ := s.crypto.VerifySignature(string(logJSON), *log.Signature)
		if valid {
			verified++
		} else {
			tampered++
		}
	}

	return total, verified, unsigned, tampered, nil
}

// Backup creates a backup of keys and audit logs.
func (s *KeyStorage) Backup(backupDir string) error {
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}

	// Copy keys file
	if data, err := os.ReadFile(s.keysFile); err == nil {
		if err := os.WriteFile(filepath.Join(backupDir, "keys.json"), data, 0600); err != nil {
			return err
		}
	}

	// Copy audit file
	if data, err := os.ReadFile(s.auditFile); err == nil {
		if err := os.WriteFile(filepath.Join(backupDir, "audit.jsonl"), data, 0600); err != nil {
			return err
		}
	}

	s.logUsage("*", "backup", "system")
	return nil
}
