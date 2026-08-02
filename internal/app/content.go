// Package app holds the use-case layer: it orchestrates the Confluence gateway
// and the Markdown renderer to implement create/update/delete and setup.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"acli-plus/internal/markdown"
)

// Content is a rendered Markdown document ready to publish.
type Content struct {
	Title    string
	Body     string // Confluence storage-format XHTML
	Warnings []string
}

// RenderFile reads a Markdown file and converts it to Confluence storage format.
// The fallback title (used when the file has no frontmatter title or leading H1)
// is the file name without its extension.
func RenderFile(path string) (Content, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Content{}, fmt.Errorf("reading %s: %w", path, err)
	}
	fallback := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	doc, err := markdown.Convert(source, fallback)
	if err != nil {
		return Content{}, err
	}
	return Content{Title: doc.Title, Body: doc.Body, Warnings: doc.Warnings}, nil
}
