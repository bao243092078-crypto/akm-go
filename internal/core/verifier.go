package core

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/baobao/akm-go/internal/models"
)

// VerifyResult holds the result of a key verification.
type VerifyResult struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Status   string   `json:"status"`  // "valid", "invalid", "error", "unsupported"
	Message  string   `json:"message"`
	Models   []string `json:"models,omitempty"`
}

// providerVerifier defines how to verify a specific provider's API key.
type providerVerifier struct {
	buildRequest func(apiKey string) (*http.Request, error)
}

var providerVerifiers = map[string]providerVerifier{
	"openai": {
		buildRequest: func(apiKey string) (*http.Request, error) {
			req, err := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return req, nil
		},
	},
	"anthropic": {
		buildRequest: func(apiKey string) (*http.Request, error) {
			req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
			return req, nil
		},
	},
	"gemini": {
		buildRequest: func(apiKey string) (*http.Request, error) {
			req, err := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("x-goog-api-key", apiKey)
			return req, nil
		},
	},
	"deepseek": {
		buildRequest: func(apiKey string) (*http.Request, error) {
			req, err := http.NewRequest("GET", "https://api.deepseek.com/models", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return req, nil
		},
	},
	"zhipu": {
		buildRequest: func(apiKey string) (*http.Request, error) {
			req, err := http.NewRequest("GET", "https://open.bigmodel.cn/api/paas/v4/models", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return req, nil
		},
	},
}

// providerAliases maps alternative provider names to canonical names.
var providerAliases = map[string]string{
	"google": "gemini",
}

// normalizeProvider converts a provider name to its canonical form.
func normalizeProvider(provider string) string {
	p := strings.ToLower(provider)
	if alias, ok := providerAliases[p]; ok {
		return alias
	}
	return p
}

// VerifyKey verifies a single API key by calling the provider's API.
func VerifyKey(name, provider, value string) *VerifyResult {
	normalized := normalizeProvider(provider)
	verifier, ok := providerVerifiers[normalized]
	if !ok {
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "unsupported",
			Message:  fmt.Sprintf("provider '%s' 不支持自动验证", provider),
		}
	}

	req, err := verifier.buildRequest(value)
	if err != nil {
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "error",
			Message:  fmt.Sprintf("构建请求失败: %v", err),
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "error",
			Message:  fmt.Sprintf("请求失败: %v", err),
		}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "valid",
			Message:  "密钥有效",
		}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "invalid",
			Message:  fmt.Sprintf("密钥无效 (HTTP %d)", resp.StatusCode),
		}
	default:
		return &VerifyResult{
			Name:     name,
			Provider: provider,
			Status:   "error",
			Message:  fmt.Sprintf("unexpected HTTP %d", resp.StatusCode),
		}
	}
}

// VerifyAll verifies all keys concurrently with a concurrency limit.
func VerifyAll(storage *KeyStorage, provider, name string) []*VerifyResult {
	keys := storage.ListKeys(provider)

	// Filter by name if specified (preserve provider filter)
	if name != "" {
		var filtered []*models.APIKey
		for _, k := range keys {
			if k.Name == name {
				filtered = append(filtered, k)
				break
			}
		}
		keys = filtered
	}

	if len(keys) == 0 {
		return nil
	}

	results := make([]*VerifyResult, len(keys))
	sem := make(chan struct{}, 5) // max 5 concurrent
	var wg sync.WaitGroup

	for i, key := range keys {
		wg.Add(1)
		go func(idx int, keyName, keyProvider string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Decrypt the key value
			value, err := storage.GetKeyValue(keyName, "verify")
			if err != nil {
				results[idx] = &VerifyResult{
					Name:     keyName,
					Provider: keyProvider,
					Status:   "error",
					Message:  fmt.Sprintf("解密失败: %v", err),
				}
				return
			}

			results[idx] = VerifyKey(keyName, keyProvider, value)
		}(i, key.Name, key.Provider)
	}

	wg.Wait()
	return results
}
