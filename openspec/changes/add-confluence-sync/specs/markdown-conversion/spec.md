## ADDED Requirements

### Requirement: Frontmatter parsing
acli-plus SHALL parse optional YAML frontmatter at the top of a Markdown file, exposing recognized keys (such as `title`, `space`, `parent`) and separating the frontmatter from the document body. A file without frontmatter SHALL be treated as body-only.

**Acceptance Criteria**:
- a file begins with a `---` delimited YAML block → recognized keys are available and the block is excluded from the body
- a file has no frontmatter → the entire file is treated as body
- the frontmatter is malformed YAML → the command fails with a clear parse error and publishes nothing

#### Scenario: File with frontmatter
- **WHEN** a file starts with a `---` delimited YAML block declaring `title`
- **THEN** the title is available to title resolution and the block is not rendered into the body

#### Scenario: File without frontmatter
- **WHEN** a file has no leading `---` block
- **THEN** the whole file is rendered as the body

#### Scenario: Malformed frontmatter errors
- **WHEN** the leading YAML block is invalid
- **THEN** the command fails with a clear parse error and does not publish

### Requirement: Supported Markdown elements
acli-plus SHALL convert the following Markdown elements to valid Confluence storage format: headings, paragraphs, ordered and unordered lists, links, bold and italic text, inline code, GFM tables, fenced code blocks, blockquotes, and task lists (checkboxes).

**Acceptance Criteria**:
- a GFM table → an equivalent Confluence table
- a fenced code block → a preformatted code block in storage format
- a task list (`- [ ]` / `- [x]`) → a Confluence task list with matching checked state
- a blockquote → a Confluence blockquote
- headings, paragraphs, lists, links, bold, italic, and inline code → their storage-format equivalents

#### Scenario: Table is converted
- **WHEN** the body contains a GFM table
- **THEN** the output contains an equivalent Confluence table

#### Scenario: Fenced code block is converted
- **WHEN** the body contains a fenced code block
- **THEN** the output contains a preformatted code block in storage format

#### Scenario: Task list preserves checked state
- **WHEN** the body contains `- [x] done` and `- [ ] todo`
- **THEN** the output is a Confluence task list with the first item checked and the second unchecked

#### Scenario: Blockquote is converted
- **WHEN** the body contains a `>` blockquote
- **THEN** the output contains a Confluence blockquote

### Requirement: Deferred elements are handled gracefully
For elements not supported in v1 — notably images and attachments — acli-plus SHALL emit a warning and skip the unsupported element while still producing valid storage-format output for the rest of the document. Unsupported content SHALL NOT abort the publish.

**Acceptance Criteria**:
- the body contains an image → a warning is emitted, the image is omitted, and the rest of the page publishes
- a document whose only special content is unsupported → an otherwise-valid page is published and a warning is emitted

#### Scenario: Image triggers a warning and is skipped
- **WHEN** the body contains a Markdown image
- **THEN** a warning is emitted, the image is omitted from the output, and the remaining content is published

### Requirement: Storage-format validity
The generated output SHALL be well-formed Confluence storage-format XHTML: tags properly closed and special characters (`<`, `>`, `&`) escaped, so the Confluence API accepts the body.

**Acceptance Criteria**:
- body text contains `<`, `>`, or `&` → these characters are escaped in the output
- an empty document → a valid, empty body is produced with no error
- generated output → all emitted tags are well-formed and closed

#### Scenario: Special characters are escaped
- **WHEN** the body contains literal `<`, `>`, or `&` characters in text
- **THEN** they are escaped in the storage-format output

#### Scenario: Empty document is valid
- **WHEN** the body is empty
- **THEN** a valid empty storage-format body is produced without error
