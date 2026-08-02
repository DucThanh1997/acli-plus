package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Document is the result of converting a Markdown file to Confluence storage format.
type Document struct {
	Title    string
	Body     string   // Confluence storage-format XHTML
	Warnings []string // e.g. skipped images
}

// Convert parses Markdown source (optionally with YAML frontmatter) and renders
// Confluence storage-format XHTML. fallbackTitle is used when the document has
// neither a frontmatter title nor a leading H1 (typically the file name).
func Convert(source []byte, fallbackTitle string) (Document, error) {
	front, body, err := splitFrontMatter(source)
	if err != nil {
		return Document{}, err
	}

	parser := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()
	root := parser.Parse(text.NewReader(body))

	rend := &renderer{source: body}
	rend.render(root)

	return Document{
		Title:    resolveTitle(front.Title, fallbackTitle),
		Body:     rend.buf.String(),
		Warnings: rend.warnings,
	}, nil
}

// resolveTitle uses the frontmatter title when present, otherwise the file name.
// A leading H1 is intentionally NOT used as the title and stays in the body.
func resolveTitle(frontTitle, fallback string) string {
	if title := strings.TrimSpace(frontTitle); title != "" {
		return title
	}
	return fallback
}
