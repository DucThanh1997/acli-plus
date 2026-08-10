package jira

import (
	"encoding/json"
	"testing"
)

// TestDocumentMarshalsAsRawJSON guards a silent, total failure mode: Document
// is a []byte, and without its MarshalJSON every description and comment would
// be sent to Jira as a base64 string instead of an ADF object.
func TestDocumentMarshalsAsRawJSON(t *testing.T) {
	body := `{"type":"doc","version":1,"content":[{"type":"paragraph"}]}`

	t.Run("inside a fields payload", func(t *testing.T) {
		fields := FieldValues{"description": Document(body)}
		encoded, err := json.Marshal(map[string]any{"fields": fields})
		if err != nil {
			t.Fatal(err)
		}
		want := `{"fields":{"description":` + body + `}}`
		if string(encoded) != want {
			t.Errorf("encoded = %s\nwant     %s", encoded, want)
		}
	})

	t.Run("a zero document is still a valid empty doc", func(t *testing.T) {
		encoded, err := json.Marshal(Document(nil))
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != `{"type":"doc","version":1,"content":[]}` {
			t.Errorf("encoded = %s, want an empty ADF document", encoded)
		}
	})

	t.Run("Empty reports whether there is content", func(t *testing.T) {
		if !Document(nil).Empty() {
			t.Error("a nil document should be empty")
		}
		if Document(body).Empty() {
			t.Error("a document with content should not be empty")
		}
	})
}
