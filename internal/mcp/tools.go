package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/baobao/akm-go/internal/core"
)

// listKeys returns a JSON list of all keys.
func listKeys(provider string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	keys := storage.ListKeys(provider)

	type keyInfo struct {
		Name          string   `json:"name"`
		Provider      string   `json:"provider"`
		Description   *string  `json:"description,omitempty"`
		SourceProject *string  `json:"source_project,omitempty"`
		Tags          []string `json:"tags,omitempty"`
		IsActive      bool     `json:"is_active"`
	}

	result := make([]keyInfo, 0, len(keys))
	for _, key := range keys {
		result = append(result, keyInfo{
			Name:          key.Name,
			Provider:      key.Provider,
			Description:   key.Description,
			SourceProject: key.SourceProject,
			Tags:          key.Tags,
			IsActive:      key.IsActive,
		})
	}

	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"keys":  result,
		"count": len(result),
	}, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// searchKeys searches keys and returns JSON results.
func searchKeys(query string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	keys := storage.SearchKeys(query)

	type keyInfo struct {
		Name        string  `json:"name"`
		Provider    string  `json:"provider"`
		Description *string `json:"description,omitempty"`
	}

	result := make([]keyInfo, 0, len(keys))
	for _, key := range keys {
		result = append(result, keyInfo{
			Name:        key.Name,
			Provider:    key.Provider,
			Description: key.Description,
		})
	}

	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"query":   query,
		"results": result,
		"count":   len(result),
	}, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// getKey returns key metadata (not the value).
func getKey(name string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	key := storage.GetKey(name)
	if key == nil {
		return "", fmt.Errorf("key '%s' not found", name)
	}

	result := map[string]interface{}{
		"name":       key.Name,
		"provider":   key.Provider,
		"is_active":  key.IsActive,
		"created_at": key.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": key.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if key.Description != nil {
		result["description"] = *key.Description
	}
	if key.SourceProject != nil {
		result["source_project"] = *key.SourceProject
	}
	if len(key.Tags) > 0 {
		result["tags"] = key.Tags
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// verifyKeys verifies key validity.
func verifyKeys(name string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	var keys []*struct {
		Name     string
		Provider string
	}

	if name != "" {
		key := storage.GetKey(name)
		if key == nil {
			return "", fmt.Errorf("key '%s' not found", name)
		}
		keys = append(keys, &struct {
			Name     string
			Provider string
		}{key.Name, key.Provider})
	} else {
		for _, key := range storage.ListKeys("") {
			keys = append(keys, &struct {
				Name     string
				Provider string
			}{key.Name, key.Provider})
		}
	}

	// TODO: Implement actual verification via provider APIs
	results := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		results = append(results, map[string]interface{}{
			"name":     key.Name,
			"provider": key.Provider,
			"status":   "pending",
			"message":  "验证功能开发中",
		})
	}

	jsonBytes, err := json.MarshalIndent(map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// exportKeys exports keys in the specified format.
func exportKeys(format, provider string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	keys, err := storage.GetKeysForExport("mcp-export", provider, nil)
	if err != nil {
		return "", err
	}

	switch format {
	case "json":
		jsonBytes, err := json.MarshalIndent(keys, "", "  ")
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil

	case "shell":
		var lines []string
		for name, value := range keys {
			escaped := strings.ReplaceAll(value, "'", "'\"'\"'")
			lines = append(lines, fmt.Sprintf("export %s='%s'", name, escaped))
		}
		return strings.Join(lines, "\n"), nil

	default: // env
		var lines []string
		for name, value := range keys {
			escaped := core.EscapeDotenvValue(value)
			lines = append(lines, fmt.Sprintf("%s=\"%s\"", name, escaped))
		}
		return strings.Join(lines, "\n"), nil
	}
}

// injectKeys writes a .env file to the specified path.
func injectKeys(path, provider string) (string, error) {
	storage, err := core.GetStorage()
	if err != nil {
		return "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[1:])
	}

	// Check if path is a directory
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("path '%s' does not exist: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path '%s' is not a directory", path)
	}

	project := filepath.Base(path)
	keys, err := storage.GetKeysForInjection(project, provider, nil)
	if err != nil {
		return "", err
	}

	if len(keys) == 0 {
		return "No keys to inject", nil
	}

	// Generate .env content
	var lines []string
	lines = append(lines, "# Generated by akm MCP")
	lines = append(lines, fmt.Sprintf("# Project: %s", project))
	lines = append(lines, "")

	for name, value := range keys {
		escaped := core.EscapeDotenvValue(value)
		lines = append(lines, fmt.Sprintf("%s=\"%s\"", name, escaped))
	}

	content := strings.Join(lines, "\n") + "\n"

	// Write file
	envPath := filepath.Join(path, ".env")
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write .env: %w", err)
	}

	return fmt.Sprintf("Wrote %d keys to %s", len(keys), envPath), nil
}

// healthCheck returns system health status.
func healthCheck() (string, error) {
	result := map[string]interface{}{
		"status": "healthy",
	}

	// Check crypto
	crypto, err := core.GetCrypto()
	if err != nil {
		result["crypto"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	} else {
		testMsg := "test"
		encrypted, err := crypto.Encrypt(testMsg)
		if err != nil {
			result["crypto"] = map[string]interface{}{
				"status": "error",
				"error":  "encryption failed",
			}
		} else {
			decrypted, err := crypto.Decrypt(encrypted)
			if err != nil || decrypted != testMsg {
				result["crypto"] = map[string]interface{}{
					"status": "error",
					"error":  "decryption failed",
				}
			} else {
				result["crypto"] = map[string]interface{}{
					"status": "ok",
				}
			}
		}
	}

	// Check storage
	storage, err := core.GetStorage()
	if err != nil {
		result["storage"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	} else {
		keys := storage.ListKeys("")
		result["storage"] = map[string]interface{}{
			"status":     "ok",
			"keys_count": len(keys),
		}
	}

	// Check audit logs
	if storage != nil {
		total, verified, unsigned, tampered, err := storage.VerifyAuditLogs()
		if err != nil {
			result["audit"] = map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			}
		} else {
			status := "ok"
			if tampered > 0 {
				status = "warning"
			}
			result["audit"] = map[string]interface{}{
				"status":   status,
				"total":    total,
				"verified": verified,
				"unsigned": unsigned,
				"tampered": tampered,
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
