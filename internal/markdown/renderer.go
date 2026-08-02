package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// renderer walks a goldmark AST and emits Confluence storage-format XHTML for
// the v1 supported element set.
type renderer struct {
	source   []byte
	buf      bytes.Buffer
	warnings []string
	taskID   int
}

func (r *renderer) render(root ast.Node) {
	r.blockChildren(root)
}

func (r *renderer) blockChildren(n ast.Node) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		r.block(child)
	}
}

func (r *renderer) block(n ast.Node) {
	switch node := n.(type) {
	case *ast.Heading:
		fmt.Fprintf(&r.buf, "<h%d>", node.Level)
		r.inlineChildren(node)
		fmt.Fprintf(&r.buf, "</h%d>", node.Level)
	case *ast.Paragraph:
		r.buf.WriteString("<p>")
		r.inlineChildren(node)
		r.buf.WriteString("</p>")
	case *ast.TextBlock:
		// Tight list-item content: inline only, no wrapping <p>.
		r.inlineChildren(node)
	case *ast.Blockquote:
		r.buf.WriteString("<blockquote>")
		r.blockChildren(node)
		r.buf.WriteString("</blockquote>")
	case *ast.List:
		r.list(node)
	case *ast.FencedCodeBlock:
		r.code(node.Lines())
	case *ast.CodeBlock:
		r.code(node.Lines())
	case *ast.ThematicBreak:
		r.buf.WriteString("<hr/>")
	case *east.Table:
		r.table(node)
	case *ast.HTMLBlock:
		// Raw HTML blocks are not emitted into storage format in v1.
	default:
		r.blockChildren(node)
	}
}

func (r *renderer) list(list *ast.List) {
	if isTaskList(list) {
		r.buf.WriteString("<ac:task-list>")
		for item := list.FirstChild(); item != nil; item = item.NextSibling() {
			r.task(item)
		}
		r.buf.WriteString("</ac:task-list>")
		return
	}

	tag := "ul"
	if list.IsOrdered() {
		tag = "ol"
	}
	fmt.Fprintf(&r.buf, "<%s>", tag)
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		r.buf.WriteString("<li>")
		r.blockChildren(item)
		r.buf.WriteString("</li>")
	}
	fmt.Fprintf(&r.buf, "</%s>", tag)
}

func (r *renderer) task(item ast.Node) {
	checkbox := firstTaskCheckBox(item)
	status := "incomplete"
	if checkbox != nil && checkbox.IsChecked {
		status = "complete"
	}
	r.taskID++
	fmt.Fprintf(&r.buf,
		"<ac:task><ac:task-id>%d</ac:task-id><ac:task-status>%s</ac:task-status><ac:task-body>",
		r.taskID, status)

	if first := item.FirstChild(); first != nil {
		for inline := first.FirstChild(); inline != nil; inline = inline.NextSibling() {
			if checkbox != nil && inline == ast.Node(checkbox) {
				continue
			}
			r.inline(inline)
		}
		for block := first.NextSibling(); block != nil; block = block.NextSibling() {
			r.block(block)
		}
	}
	r.buf.WriteString("</ac:task-body></ac:task>")
}

func (r *renderer) code(lines *text.Segments) {
	var raw strings.Builder
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		raw.Write(segment.Value(r.source))
	}
	r.buf.WriteString("<pre>")
	r.buf.WriteString(escapeText(raw.String()))
	r.buf.WriteString("</pre>")
}

func (r *renderer) table(tbl *east.Table) {
	r.buf.WriteString("<table><tbody>")
	for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
		cellTag := "td"
		if _, isHeader := row.(*east.TableHeader); isHeader {
			cellTag = "th"
		}
		r.buf.WriteString("<tr>")
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			fmt.Fprintf(&r.buf, "<%s>", cellTag)
			r.inlineChildren(cell)
			fmt.Fprintf(&r.buf, "</%s>", cellTag)
		}
		r.buf.WriteString("</tr>")
	}
	r.buf.WriteString("</tbody></table>")
}

func (r *renderer) inlineChildren(n ast.Node) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		r.inline(child)
	}
}

func (r *renderer) inline(n ast.Node) {
	switch node := n.(type) {
	case *ast.Text:
		r.buf.WriteString(escapeText(string(node.Segment.Value(r.source))))
		switch {
		case node.HardLineBreak():
			r.buf.WriteString("<br/>")
		case node.SoftLineBreak():
			r.buf.WriteString("\n")
		}
	case *ast.String:
		r.buf.WriteString(escapeText(string(node.Value)))
	case *ast.CodeSpan:
		r.buf.WriteString("<code>")
		r.buf.WriteString(escapeText(rawText(node, r.source)))
		r.buf.WriteString("</code>")
	case *ast.Emphasis:
		tag := "em"
		if node.Level >= 2 {
			tag = "strong"
		}
		fmt.Fprintf(&r.buf, "<%s>", tag)
		r.inlineChildren(node)
		fmt.Fprintf(&r.buf, "</%s>", tag)
	case *east.Strikethrough:
		r.buf.WriteString("<s>")
		r.inlineChildren(node)
		r.buf.WriteString("</s>")
	case *ast.Link:
		r.buf.WriteString(`<a href="`)
		r.buf.WriteString(escapeAttr(string(node.Destination)))
		r.buf.WriteString(`">`)
		r.inlineChildren(node)
		r.buf.WriteString("</a>")
	case *ast.AutoLink:
		target := string(node.URL(r.source))
		r.buf.WriteString(`<a href="`)
		r.buf.WriteString(escapeAttr(target))
		r.buf.WriteString(`">`)
		r.buf.WriteString(escapeText(target))
		r.buf.WriteString("</a>")
	case *ast.Image:
		r.warnings = append(r.warnings, "image skipped (not supported in v1): "+string(node.Destination))
	case *east.TaskCheckBox:
		// Rendered by task(); ignore anywhere else.
	case *ast.RawHTML:
		// Raw inline HTML is not emitted into storage format in v1.
	default:
		r.inlineChildren(node)
	}
}

func isTaskList(list *ast.List) bool {
	return firstTaskCheckBox(list.FirstChild()) != nil
}

func firstTaskCheckBox(item ast.Node) *east.TaskCheckBox {
	if item == nil {
		return nil
	}
	first := item.FirstChild()
	if first == nil {
		return nil
	}
	for child := first.FirstChild(); child != nil; child = child.NextSibling() {
		if checkbox, ok := child.(*east.TaskCheckBox); ok {
			return checkbox
		}
	}
	return nil
}

// rawText concatenates the plain text of a node's inline descendants.
func rawText(n ast.Node, source []byte) string {
	var out strings.Builder
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := node.(type) {
		case *ast.Text:
			out.Write(t.Segment.Value(source))
		case *ast.String:
			out.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return out.String()
}

var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

func escapeText(s string) string { return textEscaper.Replace(s) }
func escapeAttr(s string) string { return attrEscaper.Replace(s) }
