// Package main is the entry point for the akm CLI tool.
package main

import (
	"embed"
	"os"

	"github.com/baobao/akm-go/internal/cli"
	"github.com/baobao/akm-go/internal/http"
)

//go:embed all:web/dist
var webAssets embed.FS

func init() {
	http.WebAssets = webAssets
}

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
