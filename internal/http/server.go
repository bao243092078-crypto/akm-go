// Package http provides the HTTP API server for akm.
package http

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded web UI files (injected from main package).
var WebAssets embed.FS

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// StartServer starts the HTTP API server.
func StartServer(port int, enableWeb bool) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS configuration
	allowOrigins := loadCorsOrigins()
	allowCredentials := true
	for _, origin := range allowOrigins {
		if origin == "*" {
			allowCredentials = false
			break
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: allowCredentials,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	api := r.Group("/api")
	api.Use(apiKeyMiddleware())
	{
		// Keys
		api.GET("/keys", listKeysHandler)
		api.POST("/keys", addKeyHandler)
		api.GET("/keys/:name", getKeyHandler)
		api.DELETE("/keys/:name", deleteKeyHandler)

		// Export
		api.POST("/export/env", exportEnvHandler)

		// Health
		api.GET("/health", healthHandler)
	}

	// Proxy routes (OpenAI-compatible)
	v1 := r.Group("/v1")
	v1.Use(apiKeyMiddleware())
	{
		v1.Any("/chat/completions", proxyHandler)
		v1.Any("/completions", proxyHandler)
		v1.Any("/embeddings", proxyHandler)
		v1.Any("/models", proxyHandler)
		v1.Any("/models/*path", proxyHandler)
	}

	// Web UI (if enabled)
	if enableWeb {
		// Try to serve embedded web assets
		subFS, err := fs.Sub(WebAssets, "web/dist")
		if err == nil {
			httpFS := http.FS(subFS)

			// Read index.html once
			indexHTML, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				fmt.Printf("Warning: Failed to read index.html: %v\n", err)
			}

			// Serve assets directory
			assetsFS, _ := fs.Sub(subFS, "assets")
			r.StaticFS("/assets", http.FS(assetsFS))

			// Serve other static files
			r.GET("/vite.svg", func(c *gin.Context) {
				c.FileFromFS("vite.svg", httpFS)
			})

			// Serve index.html for root and SPA routes
			serveIndex := func(c *gin.Context) {
				if indexHTML != nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
				} else {
					c.String(http.StatusInternalServerError, "index.html not found")
				}
			}

			r.GET("/", serveIndex)

			// SPA fallback
			r.NoRoute(func(c *gin.Context) {
				if !strings.HasPrefix(c.Request.URL.Path, "/api") {
					serveIndex(c)
				}
			})
		} else {
			fmt.Printf("Warning: Failed to load web assets: %v\n", err)
			r.GET("/", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Web UI not available. Build with: cd web && npm run build",
					"api":     "/api",
				})
			})
		}
	}

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("🌐 HTTP API: http://localhost%s/api\n", addr)
	fmt.Printf("🔀 Proxy:    http://localhost%s/v1/chat/completions\n", addr)
	if enableWeb {
		fmt.Printf("🖥️  Web UI:   http://localhost%s/\n", addr)
	}
	fmt.Println()

	return r.Run(addr)
}

func loadCorsOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("AKM_CORS_ORIGINS"))
	if raw == "" {
		return []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}
	}
	items := []string{}
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	if len(items) == 0 {
		return []string{"*"}
	}
	return items
}

func apiKeyMiddleware() gin.HandlerFunc {
	require := parseBoolEnv("AKM_REQUIRE_API_KEY", false) || os.Getenv("AKM_API_KEY") != ""
	return func(c *gin.Context) {
		if !require || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if c.Request.URL.Path == "/api/health" {
			c.Next()
			return
		}
		apiKey := os.Getenv("AKM_API_KEY")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "AKM_API_KEY not configured"})
			return
		}
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			token = c.GetHeader("X-API-Key")
		}
		if token == "" {
			token = c.GetHeader("Api-Key")
		}
		if token != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return ""
}

func parseBoolEnv(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
