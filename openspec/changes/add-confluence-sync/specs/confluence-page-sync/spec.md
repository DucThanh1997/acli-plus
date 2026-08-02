## ADDED Requirements

### Requirement: Create page as upsert under a parent
The `acli-plus create <file.md> <url>` command SHALL treat `<url>` as the parent location (a page or a space) and publish the Markdown file as a child page under it. If a page with the resolved title already exists directly under that parent, the command SHALL update that page instead of creating a duplicate.

**Acceptance Criteria**:
- no child with the resolved title exists under the parent → a new child page is created
- a child with the same title already exists under the parent → that page is updated, not duplicated
- `<url>` is a space with no page → the page is created at the space root
- the resolved host has no credentials → the command fails pointing to `acli-plus setup`

#### Scenario: Create a new child page
- **WHEN** the user runs `create notes.md <parentUrl>` and no child with the resolved title exists under the parent
- **THEN** a new page is created as a child of the parent

#### Scenario: Create collapses to update when the title exists
- **WHEN** a child page with the resolved title already exists under the parent
- **THEN** that existing page is updated rather than a duplicate being created

#### Scenario: Create at space root
- **WHEN** `<url>` points at a space rather than a page
- **THEN** the page is created at the root of that space

### Requirement: Update page in place by page id
The `acli-plus update <file.md> <url>` command SHALL target the specific page identified by `<url>` and overwrite its body with the rendered Markdown. If the file's resolved title differs from the current page title, the page SHALL be renamed. If the page id no longer resolves, the command SHALL insert the content into the space parsed from `<url>` and warn about the fallback.

**Acceptance Criteria**:
- the URL's page exists → its body is overwritten in place
- the file's title differs from the page title → the page is renamed to the file's title, keeping the same page id
- the URL's page id does not resolve → the content is inserted into the URL's space and a warning notes the fallback

#### Scenario: Update overwrites in place
- **WHEN** the user runs `update page.md <pageUrl>` for an existing page
- **THEN** the page's body is replaced with the rendered Markdown

#### Scenario: Update renames the page
- **WHEN** the file's resolved title differs from the target page's current title
- **THEN** the page is renamed to the file's title while keeping the same page id

#### Scenario: Update inserts when the page is gone
- **WHEN** the page id in `<url>` does not resolve
- **THEN** the content is inserted into the space parsed from `<url>` and the user is warned about the fallback

### Requirement: Title resolution
acli-plus SHALL resolve a page's title using the frontmatter `title` when present, otherwise the file name without its extension. A leading `# H1` SHALL NOT be used as the title and SHALL remain in the rendered body as normal content.

**Acceptance Criteria**:
- frontmatter `title` present → it is used as the title
- no frontmatter title → the file name without its extension is the title
- a leading `# H1` present → it stays in the rendered body (never used as the title, never stripped)

#### Scenario: Frontmatter title wins
- **WHEN** the file has frontmatter `title: Onboarding`
- **THEN** the page title is `Onboarding`

#### Scenario: Filename is the default title
- **WHEN** the file has no frontmatter `title`
- **THEN** the page title is the file name without its extension

#### Scenario: Leading H1 stays in the body
- **WHEN** the file has no frontmatter title but begins with `# Setup Guide`
- **THEN** the page title is the file name and the `# Setup Guide` heading still appears in the page body

### Requirement: Version handling and stateless conflict detection
On every write, acli-plus SHALL send the correct next version number and stamp a recognizable version message. Before overwriting an existing page on `update`, if the page's latest version was not stamped by acli-plus (indicating it was edited outside the tool), the command SHALL stop, warn, and require confirmation before proceeding. A `--force` or `--yes` flag SHALL skip the prompt.

**Acceptance Criteria**:
- the page's latest version was written by acli-plus → the update proceeds without prompting
- the page's latest version was edited outside acli-plus → the user is warned and prompted before overwrite
- `--force` or `--yes` is supplied → no prompt is shown and the overwrite proceeds
- a successful write → the stored version number increments by exactly one

#### Scenario: Unmodified page updates silently
- **WHEN** the target page's latest version carries acli-plus's stamp
- **THEN** the update proceeds without a conflict prompt

#### Scenario: Externally modified page prompts
- **WHEN** the target page's latest version was edited in the Confluence UI and lacks acli-plus's stamp
- **THEN** the command warns and asks for confirmation before overwriting

#### Scenario: Force skips the prompt
- **WHEN** the user passes `--force` (or `--yes`) on an externally modified page
- **THEN** the page is overwritten without prompting

### Requirement: Delete page to trash
The `acli-plus delete <url>` command SHALL move the page identified by `<url>` to the Confluence trash (a reversible operation) after a `[y/N]` confirmation. A `--yes` flag SHALL skip the confirmation. Permanent purge SHALL NOT be performed.

**Acceptance Criteria**:
- the user confirms deletion → the page is moved to trash
- the user declines deletion → nothing is changed
- `--yes` is supplied → the page is trashed without prompting
- the URL does not resolve to a page → the command fails with a clear error

#### Scenario: Confirmed delete trashes the page
- **WHEN** the user runs `delete <url>` and confirms the prompt
- **THEN** the page is moved to the Confluence trash

#### Scenario: Declined delete is a no-op
- **WHEN** the user runs `delete <url>` and answers `N`
- **THEN** no change is made to the page

#### Scenario: Nonexistent page errors
- **WHEN** `<url>` does not resolve to an existing page
- **THEN** the command fails with a clear error and deletes nothing

### Requirement: Confluence URL parsing
acli-plus SHALL parse Confluence URLs to extract host, space key, and page id when present. It SHALL support standard page URLs, space URLs without a page id, personal space keys (prefixed with `~`), and a bare numeric page id. Short `/wiki/x/...` links SHALL be rejected with a message asking for the full URL.

**Acceptance Criteria**:
- a standard page URL → host, space key, and page id are extracted
- a space URL → host and space key are extracted with no page id
- a bare numeric argument → treated as a page id against the resolved host
- a short `/wiki/x/...` link → rejected with guidance to paste the full URL

#### Scenario: Parse a full page URL
- **WHEN** a URL like `https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Title` is supplied
- **THEN** host `acme.atlassian.net`, space `DEV`, and page id `98765` are extracted

#### Scenario: Parse a space URL
- **WHEN** a URL points at a space overview with no page id
- **THEN** the host and space key are extracted and no page id is set

#### Scenario: Reject a short link
- **WHEN** a `/wiki/x/...` short link is supplied
- **THEN** the command is rejected with a message asking for the full page URL

### Requirement: Dry-run preview
When `--dry-run` is supplied to `create`, `update`, or `delete`, acli-plus SHALL report the action it would take (create vs update, and the target page or space) without performing any write.

**Acceptance Criteria**:
- `--dry-run` on create or update → prints whether it would create or update and the resolved target, and writes nothing
- `--dry-run` on delete → prints the page that would be trashed, and deletes nothing

#### Scenario: Dry-run reports without writing
- **WHEN** the user runs `create notes.md <url> --dry-run`
- **THEN** the tool prints whether it would create or update and the resolved target, and no page is written

#### Scenario: Dry-run delete
- **WHEN** the user runs `delete <url> --dry-run`
- **THEN** the tool prints the page it would trash and performs no deletion
