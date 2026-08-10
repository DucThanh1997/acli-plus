package markdown

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ADFToText flattens an Atlassian Document Format value into readable plain
// text for terminal output. Input that is not ADF JSON (the v2 API returned
// wiki markup as a plain string) is returned as-is.
func ADFToText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if plain, ok := root.(string); ok {
		return strings.TrimSpace(plain)
	}

	var out strings.Builder
	writeADFBlock(&out, root, "")
	return strings.TrimRight(out.String(), "\n")
}

// writeADFBlock renders one block-level node and its descendants.
func writeADFBlock(out *strings.Builder, value any, indent string) {
	node, ok := value.(map[string]any)
	if !ok {
		return
	}
	content := childNodes(node)

	switch nodeType(node) {
	case "doc":
		for _, child := range content {
			writeADFBlock(out, child, indent)
		}
	case "paragraph":
		writeLines(out, indent, adfInline(content))
	case "heading":
		level := 1
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if raw, ok := attrs["level"].(float64); ok {
				level = int(raw)
			}
		}
		writeLines(out, indent, strings.Repeat("#", level)+" "+adfInline(content))
	case "bulletList":
		writeADFList(out, content, indent, false)
	case "orderedList":
		writeADFList(out, content, indent, true)
	case "taskList":
		for _, item := range content {
			writeADFTask(out, item, indent)
		}
	case "blockquote":
		var quoted strings.Builder
		for _, child := range content {
			writeADFBlock(&quoted, child, "")
		}
		for _, line := range splitLines(quoted.String()) {
			out.WriteString(indent + "> " + line + "\n")
		}
		out.WriteString("\n")
	case "codeBlock":
		language := ""
		if attrs, ok := node["attrs"].(map[string]any); ok {
			language, _ = attrs["language"].(string)
		}
		out.WriteString(indent + "```" + language + "\n")
		for _, line := range splitLines(adfInline(content)) {
			out.WriteString(indent + line + "\n")
		}
		out.WriteString(indent + "```\n\n")
	case "rule":
		out.WriteString(indent + "---\n\n")
	case "table":
		for _, row := range content {
			writeADFTableRow(out, row, indent)
		}
		out.WriteString("\n")
	case "mediaSingle", "mediaGroup":
		for _, child := range content {
			writeADFBlock(out, child, indent)
		}
	case "media":
		writeLines(out, indent, "[attachment: "+attrString(node, "id", "alt", "url")+"]")
	default:
		// Unknown block: render its children, falling back to inline text so
		// nothing silently disappears from the output.
		if len(content) > 0 && isBlockContent(content) {
			for _, child := range content {
				writeADFBlock(out, child, indent)
			}
			return
		}
		writeLines(out, indent, adfInline(content))
	}
}

func writeADFList(out *strings.Builder, items []any, indent string, ordered bool) {
	for i, item := range items {
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", i+1)
		}
		writeADFListItem(out, item, indent, marker)
	}
}

// writeADFListItem renders the item's first block on the marker line and any
// further blocks (nested lists, extra paragraphs) indented beneath it.
func writeADFListItem(out *strings.Builder, value any, indent, marker string) {
	node, ok := value.(map[string]any)
	if !ok {
		return
	}
	var body strings.Builder
	for _, child := range childNodes(node) {
		writeADFBlock(&body, child, "")
	}
	lines := splitLines(strings.TrimRight(body.String(), "\n"))
	if len(lines) == 0 {
		return
	}
	out.WriteString(indent + marker + lines[0] + "\n")
	continuation := indent + strings.Repeat(" ", len(marker))
	for _, line := range lines[1:] {
		if line == "" {
			out.WriteString("\n")
			continue
		}
		out.WriteString(continuation + line + "\n")
	}
}

func writeADFTask(out *strings.Builder, value any, indent string) {
	node, ok := value.(map[string]any)
	if !ok {
		return
	}
	box := "[ ] "
	if attrs, ok := node["attrs"].(map[string]any); ok {
		if state, _ := attrs["state"].(string); state == "DONE" {
			box = "[x] "
		}
	}
	writeADFListItem(out, value, indent, box)
}

func writeADFTableRow(out *strings.Builder, value any, indent string) {
	row, ok := value.(map[string]any)
	if !ok {
		return
	}
	cells := make([]string, 0, 4)
	for _, cell := range childNodes(row) {
		var body strings.Builder
		if node, ok := cell.(map[string]any); ok {
			for _, child := range childNodes(node) {
				writeADFBlock(&body, child, "")
			}
		}
		cells = append(cells, strings.Join(splitLines(strings.TrimSpace(body.String())), " "))
	}
	out.WriteString(indent + strings.Join(cells, " | ") + "\n")
}

// adfInline concatenates inline nodes into a single line of text.
func adfInline(content []any) string {
	var out strings.Builder
	for _, value := range content {
		node, ok := value.(map[string]any)
		if !ok {
			continue
		}
		switch nodeType(node) {
		case "text":
			out.WriteString(stringField(node, "text"))
		case "hardBreak":
			out.WriteString("\n")
		case "mention":
			out.WriteString(attrString(node, "text", "displayName", "id"))
		case "emoji":
			out.WriteString(attrString(node, "text", "shortName"))
		case "date":
			out.WriteString(attrString(node, "timestamp"))
		case "status":
			out.WriteString("[" + attrString(node, "text") + "]")
		case "inlineCard", "blockCard":
			out.WriteString(attrString(node, "url"))
		case "media":
			out.WriteString("[attachment: " + attrString(node, "alt", "id", "url") + "]")
		default:
			out.WriteString(adfInline(childNodes(node)))
		}
	}
	return out.String()
}

// writeLines emits a block of text followed by a blank line, skipping empties.
func writeLines(out *strings.Builder, indent, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	for _, line := range splitLines(body) {
		out.WriteString(indent + line + "\n")
	}
	out.WriteString("\n")
}

func splitLines(value string) []string {
	trimmed := strings.TrimRight(value, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func nodeType(node map[string]any) string {
	value, _ := node["type"].(string)
	return value
}

func stringField(node map[string]any, key string) string {
	value, _ := node[key].(string)
	return value
}

func childNodes(node map[string]any) []any {
	content, _ := node["content"].([]any)
	return content
}

// isBlockContent reports whether a node list looks like block nodes, used to
// decide how to render node types this flattener does not know.
func isBlockContent(content []any) bool {
	for _, value := range content {
		node, ok := value.(map[string]any)
		if !ok {
			return false
		}
		switch nodeType(node) {
		case "text", "hardBreak", "mention", "emoji", "date", "status", "inlineCard":
			return false
		}
	}
	return true
}

// attrString returns the first non-empty attribute among keys.
func attrString(node map[string]any, keys ...string) string {
	attrs, ok := node["attrs"].(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if value, ok := attrs[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
