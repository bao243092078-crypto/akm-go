// Package http provides the HTTP API server for akm.
package http

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
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
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API routes
	api := r.Group("/api")
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
	if enableWeb {
		fmt.Printf("🖥️  Web UI:   http://localhost%s/\n", addr)
	}
	fmt.Println()

	return r.Run(addr)
}
