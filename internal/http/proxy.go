package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/baobao/akm-go/internal/core"
	"github.com/gin-gonic/gin"
)

// ProviderRoute defines how to reach a provider's API.
type ProviderRoute struct {
	BaseURL      string
	AuthHeader   string            // e.g. "Authorization", "x-api-key"
	AuthPrefix   string            // e.g. "Bearer "
	ExtraHeaders map[string]string // e.g. anthropic-version
}

var providerRoutes = map[string]ProviderRoute{
	"openai": {
		BaseURL:    "https://api.openai.com",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	},
	"anthropic": {
		BaseURL:    "https://api.anthropic.com",
		AuthHeader: "x-api-key",
		ExtraHeaders: map[string]string{
			"anthropic-version": "2023-06-01",
		},
	},
	"deepseek": {
		BaseURL:    "https://api.deepseek.com",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	},
	"gemini": {
		BaseURL:    "https://generativelanguage.googleapis.com",
		AuthHeader: "x-goog-api-key",
	},
	"zhipu": {
		BaseURL:    "https://open.bigmodel.cn/api/paas",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	},
}

// model prefix → provider mapping for auto-detection
var modelPrefixMap = map[string]string{
	"gpt-":      "openai",
	"o1-":       "openai",
	"o3-":       "openai",
	"o4-":       "openai",
	"claude-":   "anthropic",
	"deepseek-": "deepseek",
	"gemini-":   "gemini",
	"glm-":      "zhipu",
}

// resolveProvider determines the provider from header or model name.
func resolveProvider(header string, body []byte) (string, error) {
	// 1. Explicit header takes priority
	if header != "" {
		header = strings.ToLower(strings.TrimSpace(header))
		if _, ok := providerRoutes[header]; ok {
			return header, nil
		}
		return "", fmt.Errorf("unknown provider: %s", header)
	}

	// 2. Infer from model name in request body
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err == nil && req.Model != "" {
		model := strings.ToLower(req.Model)
		for prefix, provider := range modelPrefixMap {
			if strings.HasPrefix(model, prefix) {
				return provider, nil
			}
		}
	}

	return "", fmt.Errorf("cannot determine provider: set X-AKM-Provider header or use a recognizable model name")
}

// selectKey picks the API key to use for the given provider.
func selectKey(storage *core.KeyStorage, provider, keyName string) (string, error) {
	// Explicit key name requested
	if keyName != "" {
		value, err := storage.GetKeyValue(keyName, "proxy")
		if err != nil {
			return "", fmt.Errorf("key '%s' not found or decrypt failed: %w", keyName, err)
		}
		return value, nil
	}

	// Find first active key for provider
	keys := storage.ListKeys(provider)
	for _, k := range keys {
		if k.IsActive {
			value, err := storage.GetKeyValue(k.Name, "proxy")
			if err != nil {
				continue
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("no active key found for provider '%s'", provider)
}

// proxyHandler handles /v1/* requests by proxying to the upstream provider.
func proxyHandler(c *gin.Context) {
	// Read request body (needed for provider detection)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	// Resolve provider
	providerHeader := c.GetHeader("X-AKM-Provider")
	provider, err := resolveProvider(providerHeader, bodyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]string{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	route, ok := providerRoutes[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]string{
				"message": fmt.Sprintf("unsupported provider: %s", provider),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// Budget check
	budget, err := core.GetBudgetTracker()
	if err == nil {
		if err := budget.Check(provider); err != nil {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": map[string]string{
					"message": err.Error(),
					"type":    "budget_exceeded",
				},
			})
			return
		}
	}

	// Get API key from storage
	storage, err := core.GetStorage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]string{
				"message": "failed to access key storage",
				"type":    "server_error",
			},
		})
		return
	}

	keyName := c.GetHeader("X-AKM-Key")
	apiKey, err := selectKey(storage, provider, keyName)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": map[string]string{
				"message": err.Error(),
				"type":    "key_error",
			},
		})
		return
	}

	// Build reverse proxy
	target, err := url.Parse(route.BaseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]string{
				"message": "invalid provider URL",
				"type":    "server_error",
			},
		})
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Inject provider auth
			req.Header.Set(route.AuthHeader, route.AuthPrefix+apiKey)

			// Set extra headers
			for k, v := range route.ExtraHeaders {
				req.Header.Set(k, v)
			}

			// Remove AKM-specific headers
			req.Header.Del("X-AKM-Provider")
			req.Header.Del("X-AKM-Key")

			// Remove original Authorization (replaced by provider key)
			if route.AuthHeader != "Authorization" {
				req.Header.Del("Authorization")
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			// Record usage after successful proxy
			if budget != nil {
				budget.Record(provider)
			}
			return nil
		},
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
