package markdown

import (
	"strings"
	"testing"
)

func TestConvertTitleResolution(t *testing.T) {
	t.Run("frontmatter title wins and H1 is kept", func(t *testing.T) {
		doc, err := Convert([]byte("---\ntitle: From Front\n---\n# A Heading\n\nbody"), "fallback")
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "From Front" {
			t.Errorf("title = %q, want From Front", doc.Title)
		}
		if !strings.Contains(doc.Body, "<h1>A Heading</h1>") {
			t.Errorf("H1 should remain in body, got %q", doc.Body)
		}
	})

	t.Run("filename is the title and a leading H1 stays in the body", func(t *testing.T) {
		doc, err := Convert([]byte("# Setup Guide\n\ntext"), "myfile")
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "myfile" {
			t.Errorf("title = %q, want myfile (file name)", doc.Title)
		}
		if !strings.Contains(doc.Body, "<h1>Setup Guide</h1>") {
			t.Errorf("leading H1 should remain in body, got %q", doc.Body)
		}
		if !strings.Contains(doc.Body, "<p>text</p>") {
			t.Errorf("body missing paragraph, got %q", doc.Body)
		}
	})

	t.Run("filename used when there is no frontmatter title", func(t *testing.T) {
		doc, err := Convert([]byte("just text"), "my-file")
		if err != nil {
			t.Fatal(err)
		}
		if doc.Title != "my-file" {
			t.Errorf("title = %q, want my-file", doc.Title)
		}
	})
}

func TestConvertFrontMatter(t *testing.T) {
	t.Run("no frontmatter is body-only", func(t *testing.T) {
		doc, err := Convert([]byte("# H\n\nbody"), "f")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(doc.Body, "body") {
			t.Errorf("body missing, got %q", doc.Body)
		}
	})

	t.Run("unterminated frontmatter errors", func(t *testing.T) {
		_, err := Convert([]byte("---\ntitle: x\nbody with no closing fence"), "f")
		if err == nil {
			t.Fatal("expected error for unterminated frontmatter")
		}
	})
}

func TestConvertSupportedElements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "GFM table",
			input: "| a | b |\n|---|---|\n| 1 | 2 |\n",
			want:  []string{"<table><tbody>", "<th>a</th>", "<td>1</td>"},
		},
		{
			name:  "fenced code block escaped",
			input: "```\ncode<>&\n```\n",
			want:  []string{"<pre>", "code&lt;&gt;&amp;"},
		},
		{
			name:  "blockquote",
			input: "> quoted\n",
			want:  []string{"<blockquote>", "quoted"},
		},
		{
			name:  "task list preserves checked state",
			input: "- [x] done\n- [ ] todo\n",
			want: []string{
				"<ac:task-list>",
				"<ac:task-status>complete</ac:task-status>", "done",
				"<ac:task-status>incomplete</ac:task-status>", "todo",
			},
		},
		{
			name:  "inline emphasis, link, code",
			input: "**bold** *em* `code` [x](https://e.com)\n",
			want:  []string{"<strong>bold</strong>", "<em>em</em>", "<code>code</code>", `<a href="https://e.com">x</a>`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Convert([]byte(tc.input), "f")
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(doc.Body, want) {
					t.Errorf("body missing %q\ngot: %s", want, doc.Body)
				}
			}
		})
	}
}

func TestConvertSpecialCharsEscaped(t *testing.T) {
	doc, err := Convert([]byte("a < b & c > d"), "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"&lt;", "&amp;", "&gt;"} {
		if !strings.Contains(doc.Body, want) {
			t.Errorf("expected %q in %q", want, doc.Body)
		}
	}
}

func TestConvertImageDeferred(t *testing.T) {
	doc, err := Convert([]byte("text ![alt](pic.png) more"), "f")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc.Body, "<img") {
		t.Errorf("image should be skipped, got %q", doc.Body)
	}
	if len(doc.Warnings) == 0 {
		t.Error("expected a warning for the skipped image")
	}
}

func TestConvertEmptyDocument(t *testing.T) {
	doc, err := Convert([]byte(""), "f")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Body != "" {
		t.Errorf("empty document should produce empty body, got %q", doc.Body)
	}
}
