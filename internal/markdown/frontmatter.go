// Package markdown converts a Markdown file (optionally with YAML frontmatter)
// into Confluence storage-format XHTML. It is pure logic with no I/O.
package markdown

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontMatter holds the recognized keys from a document's YAML frontmatter.
type FrontMatter struct {
	Title  string `yaml:"title"`
	Space  string `yaml:"space"`
	Parent string `yaml:"parent"`
}

const frontMatterFence = "---"

// splitFrontMatter separates an optional leading YAML block (delimited by ---)
// from the document body. A document without a leading fence is treated as
// body-only. A fence that is opened but never closed is a parse error.
func splitFrontMatter(source []byte) (FrontMatter, []byte, error) {
	lines := strings.Split(normalizeNewlines(source), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontMatterFence {
		return FrontMatter{}, source, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == frontMatterFence {
			end = i
			break
		}
	}
	if end == -1 {
		return FrontMatter{}, nil, fmt.Errorf("markdown: unterminated frontmatter block")
	}

	block := strings.Join(lines[1:end], "\n")
	var front FrontMatter
	if err := yaml.Unmarshal([]byte(block), &front); err != nil {
		return FrontMatter{}, nil, fmt.Errorf("markdown: parsing frontmatter: %w", err)
	}

	body := strings.Join(lines[end+1:], "\n")
	return front, []byte(body), nil
}

func normalizeNewlines(source []byte) string {
	return strings.ReplaceAll(string(source), "\r\n", "\n")
}
