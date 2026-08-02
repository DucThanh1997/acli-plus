## 1. Project scaffolding

- [x] 1.1 `go mod init` for the module and reconcile `go.mod` with the staged `go.sum` (cobra, goldmark, yaml.v3, x/term)
- [x] 1.2 Create the Cobra root command `acli-plus` with `--version` and global `--dry-run`, `--yes`/`--force`, `--site` flags
- [x] 1.3 Lay out package structure: `cmd/` (commands), `internal/config`, `internal/confluence` (REST client), `internal/mdconv` (renderer), `internal/urlparse`

## 2. Config & credentials (auth-and-config)

- [x] 2.1 Implement the per-host credential store at `~/.config/acli-plus/credentials.yaml`: load/save keyed by host, create file with 0600 permissions
- [x] 2.2 Implement the per-project `acli-plus.yaml` loader (site, default space/parent); tolerate absence; never read/write a token here
- [x] 2.3 Implement host resolution: URL host authoritative → `--site` flag → env var → project config; look up token by resolved host; error to "run acli-plus setup" when missing
- [x] 2.4 Implement `acli-plus setup`: prompt for site URL + email, read API token hidden via x/term, verify credentials against the site, then upsert the host entry (no duplicates)

## 3. Confluence REST client (API v2)

- [x] 3.1 HTTP client with Basic auth (`email:api_token`), base URL per host, JSON encode/decode, and typed API errors
- [x] 3.2 Resolve and cache numeric space-id from a space key
- [x] 3.3 Read helpers: get page by id (with version + version message), find child page by title under a parent/space
- [x] 3.4 Write helpers: create page, update page (send next version number + acli-plus version-message stamp), delete page to trash
- [x] 3.5 Version-message sentinel helpers: stamp on write, detect whether a page's latest version was written by acli-plus; handle 409 by re-fetching

## 4. Markdown conversion (markdown-conversion)

- [x] 4.1 Frontmatter parser: split leading `---` YAML block from body; expose title/space/parent; clear error on malformed YAML
- [x] 4.2 Custom goldmark renderer emitting Confluence storage-format XHTML for the supported set (headings, paragraphs, ordered/unordered lists, links, bold/italic, inline code, blockquotes)
- [x] 4.3 Render GFM tables, fenced code blocks (`<pre>`), and task lists (checked/unchecked) to storage format
- [x] 4.4 Title resolution (frontmatter `title` → first `# H1` → filename) and strip the leading H1 from the body when it becomes the title
- [x] 4.5 Deferred elements: warn & skip images/attachments; ensure the rest still renders as valid storage format
- [x] 4.6 Ensure output validity: escape `<`, `>`, `&`; well-formed/closed tags; valid empty body for empty input

## 5. URL parsing (confluence-page-sync)

- [x] 5.1 Parse host, space key, and page id from standard page URLs and space URLs; support personal space keys (`~`) and a bare numeric page id
- [x] 5.2 Reject short `/wiki/x/...` links with a message asking for the full URL

## 6. Commands (confluence-page-sync)

- [x] 6.1 `create <file.md> <url>`: treat `<url>` as parent; resolve title; upsert (update existing child by title, else create); support space-root parent
- [x] 6.2 `update <file.md> <url>`: target the URL's page id; overwrite in place; rename when title differs; insert into the URL's space with a warning when the id does not resolve
- [x] 6.3 Conflict detection on update: when the latest version lacks acli-plus's stamp, warn and prompt `[y/N]`; `--force`/`--yes` skips
- [x] 6.4 `delete <url>`: confirm `[y/N]` (skip with `--yes`), move page to trash, clear error when the URL does not resolve
- [x] 6.5 Wire `--dry-run` through create/update/delete: report the would-be action and target, perform no write

## 7. Tests & docs

- [x] 7.1 Unit tests for URL parsing, frontmatter/title resolution, and the storage-format renderer (one test per spec scenario)
- [x] 7.2 Unit tests for config resolution precedence and the version-message conflict detection
- [x] 7.3 README: `setup`, `create`, `update`, `delete`, `--dry-run`, multi-site config, and the v1 scope/limitations
- [x] 7.4 Manual smoke test against a real Confluence site: setup → create → update (with and without external edit) → delete
