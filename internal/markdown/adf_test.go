package markdown

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertADFTitle(t *testing.T) {
	t.Run("frontmatter title wins and the H1 stays in the body", func(t *testing.T) {
		doc, err := ConvertADF([]byte("---\ntitle: From Front\n---\n# A Heading\n\nbody"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "From Front" {
			t.Errorf("title = %q, want From Front", doc.Title)
		}
		if !strings.Contains(string(doc.JSON), `"heading"`) {
			t.Errorf("H1 should remain in the body, got %s", doc.JSON)
		}
	})

	t.Run("leading H1 becomes the title and leaves the body", func(t *testing.T) {
		doc, err := ConvertADF([]byte("# Setup Guide\n\ntext"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "Setup Guide" {
			t.Errorf("title = %q, want Setup Guide", doc.Title)
		}
		if strings.Contains(string(doc.JSON), `"heading"`) {
			t.Errorf("consumed H1 should not stay in the body, got %s", doc.JSON)
		}
	})

	t.Run("no heading leaves the title empty", func(t *testing.T) {
		doc, err := ConvertADF([]byte("just text"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "" {
			t.Errorf("title = %q, want empty", doc.Title)
		}
	})
}

func TestConvertADFStructure(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		contains []string
	}{
		{
			name:     "paragraph",
			source:   "hello world",
			contains: []string{`{"type":"paragraph","content":[{"type":"text","text":"hello world"}]}`},
		},
		{
			name:     "heading level",
			source:   "## Sub",
			contains: []string{`"type":"heading","attrs":{"level":2}`},
		},
		{
			name:     "bold and italic become marks on the text node",
			source:   "**bold** and *soft*",
			contains: []string{`"text":"bold","marks":[{"type":"strong"}]`, `"text":"soft","marks":[{"type":"em"}]`},
		},
		{
			name:     "nested marks accumulate",
			source:   "**[link](https://example.com)**",
			contains: []string{`"marks":[{"type":"strong"},{"type":"link","attrs":{"href":"https://example.com"}}]`},
		},
		{
			name:     "inline code",
			source:   "use `go test`",
			contains: []string{`"text":"go test","marks":[{"type":"code"}]`},
		},
		{
			name:     "fenced code keeps its language and drops the trailing newline",
			source:   "```go\nfmt.Println(1)\n```",
			contains: []string{`"type":"codeBlock","attrs":{"language":"go"}`, `"text":"fmt.Println(1)"`},
		},
		{
			name:     "bullet list wraps items in paragraphs",
			source:   "- one\n- two",
			contains: []string{`"type":"bulletList"`, `"type":"listItem","content":[{"type":"paragraph"`},
		},
		{
			name:     "ordered list",
			source:   "1. one\n2. two",
			contains: []string{`"type":"orderedList"`},
		},
		{
			name:     "task list maps to taskItem state",
			source:   "- [x] done\n- [ ] todo",
			contains: []string{`"type":"taskList"`, `"state":"DONE"`, `"state":"TODO"`},
		},
		{
			name:     "table header cells",
			source:   "| a | b |\n|---|---|\n| 1 | 2 |",
			contains: []string{`"type":"table"`, `"type":"tableHeader"`, `"type":"tableCell"`},
		},
		{
			name:     "blockquote holds block content",
			source:   "> quoted",
			contains: []string{`"type":"blockquote","content":[{"type":"paragraph"`},
		},
		{
			name:     "thematic break",
			source:   "before\n\n---\n\nafter",
			contains: []string{`{"type":"rule"}`},
		},
		{
			name:     "strikethrough",
			source:   "~~gone~~",
			contains: []string{`"marks":[{"type":"strike"}]`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := ConvertADF([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			body := string(doc.JSON)
			for _, want := range tc.contains {
				if !strings.Contains(body, want) {
					t.Errorf("output missing %s\ngot: %s", want, body)
				}
			}
			assertValidADF(t, doc.JSON)
		})
	}
}

// TestConvertADFNoEmptyTextNodes guards the one ADF rule that is easy to break
// and that the API rejects outright: a text node with an empty string.
func TestConvertADFNoEmptyTextNodes(t *testing.T) {
	doc, err := ConvertADF([]byte("# T\n\n\n\n*  *\n\n```\n```\n\n| |\n|-|\n| |\n"))
	if err != nil {
		t.Fatal(err)
	}
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		if node["type"] == "text" {
			if text, _ := node["text"].(string); text == "" {
				t.Errorf("empty text node in %s", doc.JSON)
			}
		}
		content, _ := node["content"].([]any)
		for _, child := range content {
			if next, ok := child.(map[string]any); ok {
				walk(next)
			}
		}
	}
	var root map[string]any
	if err := json.Unmarshal(doc.JSON, &root); err != nil {
		t.Fatal(err)
	}
	walk(root)
}

func TestConvertADFImageBecomesLink(t *testing.T) {
	doc, err := ConvertADF([]byte("![alt text](https://example.com/a.png)"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc.JSON), `"text":"alt text"`) {
		t.Errorf("image alt text should survive as link text, got %s", doc.JSON)
	}
	if !strings.Contains(string(doc.JSON), `"href":"https://example.com/a.png"`) {
		t.Errorf("image destination should become a link href, got %s", doc.JSON)
	}
	if len(doc.Warnings) != 1 {
		t.Errorf("warnings = %v, want one image warning", doc.Warnings)
	}
}

func TestConvertADFEscapes(t *testing.T) {
	t.Run("a backslash escape yields the bare character", func(t *testing.T) {
		doc, err := ConvertADFBody([]byte(`giá 5\*3\*2 đồng`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(doc.JSON), `"text":"giá 5*3*2 đồng"`) {
			t.Errorf("json = %s, want the backslashes consumed", doc.JSON)
		}
	})

	t.Run("a backslash inside a code span is content", func(t *testing.T) {
		doc, err := ConvertADFBody([]byte("code: `a\\*b`"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(doc.JSON), `"text":"a\\*b"`) {
			t.Errorf("json = %s, want the backslash kept inside code", doc.JSON)
		}
	})

	t.Run("a leading heading is kept by the body form", func(t *testing.T) {
		doc, err := ConvertADFBody([]byte("# Heading\n\nbody"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "" {
			t.Errorf("title = %q, want empty", doc.Title)
		}
		if !strings.Contains(string(doc.JSON), `"type":"heading"`) {
			t.Errorf("json = %s, want the heading kept", doc.JSON)
		}
	})

	t.Run("the title form still lifts the heading out", func(t *testing.T) {
		doc, err := ConvertADF([]byte("# Heading\n\nbody"))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "Heading" {
			t.Errorf("title = %q, want Heading", doc.Title)
		}
		if strings.Contains(string(doc.JSON), `"type":"heading"`) {
			t.Errorf("json = %s, want the heading consumed", doc.JSON)
		}
	})
}

func TestTextToADF(t *testing.T) {
	t.Run("blank lines split paragraphs", func(t *testing.T) {
		encoded := string(TextToADF("first\n\nsecond"))
		if strings.Count(encoded, `"type":"paragraph"`) != 2 {
			t.Errorf("want two paragraphs, got %s", encoded)
		}
	})

	t.Run("single newline is a hard break", func(t *testing.T) {
		encoded := string(TextToADF("one\ntwo"))
		if !strings.Contains(encoded, `{"type":"hardBreak"}`) {
			t.Errorf("want a hardBreak, got %s", encoded)
		}
	})

	t.Run("empty input is still a valid document", func(t *testing.T) {
		assertValidADF(t, TextToADF(""))
	})
}

func TestParseADF(t *testing.T) {
	t.Run("passes an ADF document through unchanged", func(t *testing.T) {
		source := `{"type":"doc","version":1,"content":[]}`
		out, ok := ParseADF([]byte(source))
		if !ok {
			t.Fatal("want ok for a doc node")
		}
		if string(out) != source {
			t.Errorf("out = %s, want it unchanged", out)
		}
	})

	t.Run("rejects markdown and non-doc JSON", func(t *testing.T) {
		for _, source := range []string{"# not json", `{"type":"paragraph"}`, `[1,2]`, ""} {
			if _, ok := ParseADF([]byte(source)); ok {
				t.Errorf("ParseADF(%q) = ok, want not ok", source)
			}
		}
	})
}

func TestADFToText(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "paragraphs",
			source: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
			want:   "hello",
		},
		{
			name:   "heading keeps its level",
			source: `{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Sub"}]}]}`,
			want:   "## Sub",
		},
		{
			name:   "bullet list",
			source: `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]}]}]}`,
			want:   "- one",
		},
		{
			name:   "mention renders its display text",
			source: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"text":"@Ann"}}]}]}`,
			want:   "@Ann",
		},
		{
			// A taskItem holds inline content, unlike listItem which holds
			// blocks; treating them the same drops the text.
			name:   "task list keeps its text and state",
			source: `{"type":"doc","content":[{"type":"taskList","attrs":{"localId":"1"},"content":[{"type":"taskItem","attrs":{"localId":"2","state":"DONE"},"content":[{"type":"text","text":"shipped"}]},{"type":"taskItem","attrs":{"localId":"3","state":"TODO"},"content":[{"type":"text","text":"pending"}]}]}]}`,
			want:   "[x] shipped\n[ ] pending",
		},
		{
			name:   "plain string body from the v2 API is passed through",
			source: `"already plain"`,
			want:   "already plain",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ADFToText([]byte(tc.source)); got != tc.want {
				t.Errorf("ADFToText = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestADFRoundTrip checks that the two directions agree on the constructs a
// work item description actually uses. Every block type the converter emits
// belongs here: a construct that survives ConvertADF but vanishes in ADFToText
// looks to the user like the description was never saved.
func TestADFRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "heading", source: "## Plan\n", want: "## Plan"},
		{name: "paragraph drops the emphasis markers", source: "Ship **fast**.\n", want: "Ship fast."},
		{name: "bullet list", source: "- one\n- two\n", want: "- one\n- two"},
		{name: "ordered list", source: "1. one\n2. two\n", want: "1. one\n2. two"},
		{name: "task list", source: "- [x] done\n- [ ] todo\n", want: "[x] done\n[ ] todo"},
		{name: "blockquote", source: "> quoted\n", want: "> quoted"},
		{name: "code block", source: "```go\nx := 1\n```\n", want: "```go\nx := 1\n```"},
		{name: "table", source: "| a | b |\n|---|---|\n| 1 | 2 |\n", want: "a | b\n1 | 2"},
		{name: "rule", source: "before\n\n---\n\nafter\n", want: "before\n\n---\n\nafter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := ConvertADF([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			if got := ADFToText(doc.JSON); got != tc.want {
				t.Errorf("round trip = %q, want %q\nADF: %s", got, tc.want, doc.JSON)
			}
		})
	}
}

// TestADFRoundTripKeepsEveryBlock is the guard the task-list bug slipped past:
// whatever the converter emits, the flattener must render something for it.
func TestADFRoundTripKeepsEveryBlock(t *testing.T) {
	source := "# Title\n\nIntro.\n\n## Heading\n\n- bullet\n\n1. numbered\n\n" +
		"- [x] done\n- [ ] todo\n\n> quoted\n\n```go\ncode()\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	doc, err := ConvertADF([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	text := ADFToText(doc.JSON)

	for _, want := range []string{
		"Intro.", "## Heading", "- bullet", "1. numbered",
		"[x] done", "[ ] todo", "> quoted", "code()", "a | b",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("round trip lost %q\ngot:\n%s\nADF: %s", want, text, doc.JSON)
		}
	}
}

// assertValidADF checks the envelope every Jira request needs: a versioned doc
// node whose content is an array.
func assertValidADF(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var doc struct {
		Type    string `json:"type"`
		Version int    `json:"version"`
		Content []any  `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, raw)
	}
	if doc.Type != "doc" || doc.Version != 1 {
		t.Errorf("envelope = %s, want type doc version 1", raw)
	}
	if doc.Content == nil {
		t.Errorf("content must be an array, got %s", raw)
	}
}
