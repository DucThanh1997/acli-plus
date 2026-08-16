package markdown

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ADFDocument is the result of converting Markdown to Atlassian Document
// Format, the rich-text shape the Jira v3 REST API requires.
type ADFDocument struct {
	// Title is the frontmatter title, or the leading H1 when there is no
	// frontmatter. Unlike Convert (where a page carries its own title), a work
	// item description has no heading of its own, so the H1 is consumed here and
	// used as the summary rather than left in the body.
	Title    string
	JSON     json.RawMessage
	Warnings []string
}

// adfNode is one node of the ADF tree. Empty fields are omitted so the payload
// matches the schema, which rejects empty text nodes and stray null attrs.
type adfNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []adfNode      `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []adfMark      `json:"marks,omitempty"`
}

// adfMark is an inline style applied to a text node.
type adfMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// ConvertADF parses Markdown (optionally with YAML frontmatter) and renders an
// ADF document. Constructs ADF cannot express from Markdown alone — images,
// which need a media id from an upload — are downgraded to links and reported
// as warnings.
func ConvertADF(source []byte) (ADFDocument, error) {
	return convertADF(source, true)
}

// ConvertADFBody renders Markdown the same way but leaves a leading heading in
// the body and never reports a Title. It is for sources that supply the summary
// separately — --description and --comment take their text inline, so consuming
// the first heading there would silently drop it from what the user wrote.
func ConvertADFBody(source []byte) (ADFDocument, error) {
	return convertADF(source, false)
}

func convertADF(source []byte, takeTitle bool) (ADFDocument, error) {
	front, body, err := splitFrontMatter(source)
	if err != nil {
		return ADFDocument{}, err
	}

	parser := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()
	root := parser.Parse(text.NewReader(body))

	rend := &adfRenderer{source: body}
	content := rend.blocks(root)

	var title string
	if takeTitle {
		title = strings.TrimSpace(front.Title)
		if title == "" {
			title, content = takeLeadingHeading(content)
		}
	}

	encoded, err := encodeDoc(content)
	if err != nil {
		return ADFDocument{}, err
	}
	return ADFDocument{Title: title, JSON: encoded, Warnings: rend.warnings}, nil
}

// TextToADF wraps plain text in a minimal ADF document: blank lines separate
// paragraphs and single newlines become hard breaks. It is for text that is
// already the flattened rendering of a document — cloning a work item whose raw
// payload is unavailable — where re-reading the flattening as Markdown would
// invent formatting the source never had. Text the user typed goes through
// ConvertADFBody instead.
func TextToADF(value string) json.RawMessage {
	blocks := make([]adfNode, 0, 4)
	for _, block := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		para := adfNode{Type: "paragraph"}
		for i, line := range strings.Split(block, "\n") {
			if i > 0 {
				para.Content = append(para.Content, adfNode{Type: "hardBreak"})
			}
			if line != "" {
				para.Content = append(para.Content, adfNode{Type: "text", Text: line})
			}
		}
		blocks = append(blocks, para)
	}
	encoded, err := encodeDoc(blocks)
	if err != nil {
		// encodeDoc only fails if the tree cannot be marshaled, which plain text
		// cannot cause; fall back to an empty document rather than panicking.
		return json.RawMessage(`{"type":"doc","version":1,"content":[]}`)
	}
	return encoded
}

// ParseADF returns data unchanged when it already is an ADF document, letting
// callers pass a hand-written ADF file straight through to the API.
func ParseADF(data []byte) (json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var probe struct {
		Type    string          `json:"type"`
		Version int             `json:"version"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		return nil, false
	}
	if probe.Type != "doc" {
		return nil, false
	}
	return json.RawMessage(trimmed), true
}

func encodeDoc(content []adfNode) (json.RawMessage, error) {
	doc := struct {
		Type    string    `json:"type"`
		Version int       `json:"version"`
		Content []adfNode `json:"content"`
	}{Type: "doc", Version: 1, Content: content}
	if doc.Content == nil {
		doc.Content = []adfNode{}
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encoding ADF: %w", err)
	}
	return encoded, nil
}

// takeLeadingHeading pulls a leading level-1 heading off the block list and
// returns its plain text as the title.
func takeLeadingHeading(content []adfNode) (string, []adfNode) {
	if len(content) == 0 || content[0].Type != "heading" {
		return "", content
	}
	if level, _ := content[0].Attrs["level"].(int); level != 1 {
		return "", content
	}
	return nodesText(content[0].Content), content[1:]
}

func nodesText(nodes []adfNode) string {
	var out strings.Builder
	for _, node := range nodes {
		out.WriteString(node.Text)
		out.WriteString(nodesText(node.Content))
	}
	return strings.TrimSpace(out.String())
}

// adfRenderer walks a goldmark AST and builds ADF nodes.
type adfRenderer struct {
	source   []byte
	warnings []string
	localID  int
}

// blocks renders every child of n as a block node.
func (r *adfRenderer) blocks(n ast.Node) []adfNode {
	var out []adfNode
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		out = append(out, r.block(child)...)
	}
	return out
}

// block renders one node, returning zero or more ADF blocks: zero for content
// ADF has no place for (raw HTML), more than one only via unwrapped containers.
func (r *adfRenderer) block(n ast.Node) []adfNode {
	switch node := n.(type) {
	case *ast.Heading:
		level := node.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return []adfNode{{
			Type:    "heading",
			Attrs:   map[string]any{"level": level},
			Content: r.inlines(node, nil),
		}}
	case *ast.Paragraph, *ast.TextBlock:
		inline := r.inlines(node, nil)
		if len(inline) == 0 {
			return nil
		}
		return []adfNode{{Type: "paragraph", Content: inline}}
	case *ast.Blockquote:
		return []adfNode{{Type: "blockquote", Content: r.blockContent(node)}}
	case *ast.List:
		return []adfNode{r.list(node)}
	case *ast.FencedCodeBlock:
		return []adfNode{r.code(node.Lines(), string(node.Language(r.source)))}
	case *ast.CodeBlock:
		return []adfNode{r.code(node.Lines(), "")}
	case *ast.ThematicBreak:
		return []adfNode{{Type: "rule"}}
	case *east.Table:
		return []adfNode{r.table(node)}
	case *ast.HTMLBlock:
		// ADF has no raw-HTML node; dropping matches the Confluence renderer.
		return nil
	default:
		return r.blocks(n)
	}
}

// blockContent renders children as blocks, guaranteeing at least one block for
// containers (list items, table cells, quotes) whose schema requires one.
func (r *adfRenderer) blockContent(n ast.Node) []adfNode {
	content := r.blocks(n)
	if len(content) == 0 {
		return []adfNode{{Type: "paragraph"}}
	}
	return content
}

func (r *adfRenderer) list(list *ast.List) adfNode {
	if isTaskList(list) {
		return r.taskList(list)
	}

	out := adfNode{Type: "bulletList"}
	if list.IsOrdered() {
		out.Type = "orderedList"
		if list.Start > 1 {
			out.Attrs = map[string]any{"order": list.Start}
		}
	}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		out.Content = append(out.Content, adfNode{
			Type:    "listItem",
			Content: r.blockContent(item),
		})
	}
	return out
}

// taskList maps a GFM checkbox list onto ADF's taskList. Every node needs a
// localId that is unique within the document.
func (r *adfRenderer) taskList(list *ast.List) adfNode {
	out := adfNode{Type: "taskList", Attrs: map[string]any{"localId": r.nextLocalID()}}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		checkbox := firstTaskCheckBox(item)
		state := "TODO"
		if checkbox != nil && checkbox.IsChecked {
			state = "DONE"
		}
		task := adfNode{
			Type:  "taskItem",
			Attrs: map[string]any{"localId": r.nextLocalID(), "state": state},
		}
		if first := item.FirstChild(); first != nil {
			for inline := first.FirstChild(); inline != nil; inline = inline.NextSibling() {
				if checkbox != nil && inline == ast.Node(checkbox) {
					continue
				}
				task.Content = append(task.Content, r.inline(inline, nil)...)
			}
			task.Content = mergeText(task.Content)
		}
		out.Content = append(out.Content, task)
	}
	return out
}

func (r *adfRenderer) nextLocalID() string {
	r.localID++
	return strconv.Itoa(r.localID)
}

func (r *adfRenderer) code(lines *text.Segments, language string) adfNode {
	var raw strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		raw.Write(segment.Value(r.source))
	}
	out := adfNode{Type: "codeBlock"}
	if language != "" {
		out.Attrs = map[string]any{"language": language}
	}
	// A trailing newline from the fence would render as a blank last line.
	if body := strings.TrimRight(raw.String(), "\n"); body != "" {
		out.Content = []adfNode{{Type: "text", Text: body}}
	}
	return out
}

func (r *adfRenderer) table(tbl *east.Table) adfNode {
	out := adfNode{
		Type:  "table",
		Attrs: map[string]any{"isNumberColumnEnabled": false, "layout": "default"},
	}
	for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
		cellType := "tableCell"
		if _, isHeader := row.(*east.TableHeader); isHeader {
			cellType = "tableHeader"
		}
		adfRow := adfNode{Type: "tableRow"}
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			content := r.inlines(cell, nil)
			block := adfNode{Type: "paragraph"}
			if len(content) > 0 {
				block.Content = content
			}
			adfRow.Content = append(adfRow.Content, adfNode{
				Type:    cellType,
				Content: []adfNode{block},
			})
		}
		out.Content = append(out.Content, adfRow)
	}
	return out
}

func (r *adfRenderer) inlines(n ast.Node, marks []adfMark) []adfNode {
	var out []adfNode
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		out = append(out, r.inline(child, marks)...)
	}
	return mergeText(out)
}

// mergeText joins adjacent text nodes that carry identical marks. goldmark
// splits a run of plain text across several Text nodes, and soft breaks add
// another; without this the ADF is valid but fragmented into a node per word.
func mergeText(nodes []adfNode) []adfNode {
	out := make([]adfNode, 0, len(nodes))
	for _, node := range nodes {
		last := len(out) - 1
		if node.Type == "text" && last >= 0 && out[last].Type == "text" && marksEqual(out[last].Marks, node.Marks) {
			out[last].Text += node.Text
			continue
		}
		out = append(out, node)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// marksEqual compares two mark sets by their encoded form, which covers both the
// mark types and their attributes (a link's href).
func marksEqual(a, b []adfMark) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	encodedA, errA := json.Marshal(a)
	encodedB, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(encodedA) == string(encodedB)
}

// inline renders one inline node. Marks accumulate as the walk descends, since
// ADF carries styling on the text node itself rather than in a wrapper element.
func (r *adfRenderer) inline(n ast.Node, marks []adfMark) []adfNode {
	switch node := n.(type) {
	case *ast.Text:
		// goldmark leaves backslash escapes in the segment and unescapes them at
		// render time, so a renderer reading the raw source has to do it here —
		// otherwise `\*` reaches Jira as a literal backslash-asterisk and there is
		// no way to write a plain `*`. Code spans and code blocks are excluded:
		// a backslash inside them is content, not an escape.
		out := textNodes(string(util.UnescapePunctuations(node.Segment.Value(r.source))), marks)
		switch {
		case node.HardLineBreak():
			out = append(out, adfNode{Type: "hardBreak"})
		case node.SoftLineBreak():
			// A soft break is a space in the rendered output, not a new block.
			out = append(out, textNodes(" ", marks)...)
		}
		return out
	case *ast.String:
		return textNodes(string(node.Value), marks)
	case *ast.CodeSpan:
		return textNodes(rawText(node, r.source), withMark(marks, adfMark{Type: "code"}))
	case *ast.Emphasis:
		mark := adfMark{Type: "em"}
		if node.Level >= 2 {
			mark = adfMark{Type: "strong"}
		}
		return r.inlines(node, withMark(marks, mark))
	case *east.Strikethrough:
		return r.inlines(node, withMark(marks, adfMark{Type: "strike"}))
	case *ast.Link:
		link := adfMark{Type: "link", Attrs: map[string]any{"href": string(node.Destination)}}
		return r.inlines(node, withMark(marks, link))
	case *ast.AutoLink:
		target := string(node.URL(r.source))
		link := adfMark{Type: "link", Attrs: map[string]any{"href": target}}
		return textNodes(target, withMark(marks, link))
	case *ast.Image:
		// ADF images are media nodes keyed by an upload id, which a Markdown
		// source cannot supply; keep the reference as a link instead of dropping it.
		target := string(node.Destination)
		r.warnings = append(r.warnings, "image rendered as a link (ADF needs an uploaded media id): "+target)
		label := rawText(node, r.source)
		if label == "" {
			label = target
		}
		link := adfMark{Type: "link", Attrs: map[string]any{"href": target}}
		return textNodes(label, withMark(marks, link))
	case *east.TaskCheckBox:
		// Rendered by taskList(); ignore anywhere else.
		return nil
	case *ast.RawHTML:
		return nil
	default:
		return r.inlines(n, marks)
	}
}

// textNodes builds a text node, dropping empty strings because ADF rejects a
// text node whose text is "".
func textNodes(value string, marks []adfMark) []adfNode {
	if value == "" {
		return nil
	}
	node := adfNode{Type: "text", Text: value}
	if len(marks) > 0 {
		node.Marks = append([]adfMark(nil), marks...)
	}
	return []adfNode{node}
}

// withMark returns marks plus one more, copying so sibling branches of the walk
// never see each other's styling.
func withMark(marks []adfMark, mark adfMark) []adfMark {
	out := make([]adfMark, 0, len(marks)+1)
	out = append(out, marks...)
	return append(out, mark)
}
