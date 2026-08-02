## Why

Atlassian's official `acli` covers only Jira, admin, and Rovo — it has **no Confluence commands at all**, so there is nothing to "wrap" for Confluence page management. Teams that keep docs as Markdown in git have no first-class way to publish them to Confluence. `acli-plus` fills that gap: an acli-style Go CLI that pushes Markdown files to Confluence Cloud (create/update/delete) over the REST API, so docs-as-code workflows work against Confluence the way they already do against Jira with `acli`.

## What Changes

- **New `acli-plus setup` command** — interactive, per-site credential registration: paste a Confluence site URL, enter Atlassian email + API token (token read hidden). Credentials are stored on the machine **keyed by host**, so multiple sites coexist and multiple projects can reuse one site's token.
- **New `acli-plus create <file.md> <url>`** — upsert a page whose parent is `<url>`. Title comes from frontmatter `title`, otherwise the file name (a leading `# H1` stays in the body). If a child with that title already exists under the parent, it updates it; otherwise it creates. (Renaming via `create` yields a new page — accepted behavior.)
- **New `acli-plus update <file.md> <url>`** — upsert targeting the exact page in `<url>` (in-place; can rename the page). If the page id no longer resolves, it inserts into the URL's space.
- **New `acli-plus delete <url>`** — move the page at `<url>` to trash (reversible), with a `[y/N]` confirmation (`--yes` to skip). No permanent purge in v1.
- **New Markdown → Confluence storage-format renderer** — v1 covers headings, paragraphs, ordered/unordered lists, links, bold/italic, inline code, **tables, fenced code blocks, blockquotes, and task lists**. Images and advanced macros are deferred (warn & skip).
- **Per-project, non-secret config** — an optional `acli-plus.yaml` in the repo (git-committable) declaring `site` and default `space`/`parent`. Tokens never live in the project file. Because pasted URLs carry the host, project config is optional for create/update/delete.
- **Stateless conflict detection** — every write stamps a version message (`via acli-plus`). On update, if the page's latest version was **not** written by acli-plus (edited manually in the UI), the command stops, warns, and asks `[y/N]` before overwriting. `--force`/`--yes` skips the prompt; `--dry-run` previews the create-vs-update decision without writing.

Explicitly **out of scope for v1**: pushing whole directories, image/attachment upload, code-macro syntax highlighting, label sync, named profiles, short `/wiki/x/` links, Jira delegation, and any Atlassian product other than Confluence.

## Capabilities

### New Capabilities
- `auth-and-config`: The `setup` command, the per-host credential store, the optional per-project `acli-plus.yaml`, and the resolution/precedence rules that pick a site and its token.
- `confluence-page-sync`: The `create`, `update`, and `delete` commands — URL parsing, title resolution, upsert semantics, version-number handling, stateless conflict detection, and `--dry-run`/`--yes`/`--force`.
- `markdown-conversion`: Converting a Markdown file (with optional YAML frontmatter) into Confluence storage-format XHTML for the supported element set, and the deferral rules for unsupported elements.

### Modified Capabilities
<!-- Greenfield project — no existing specs to modify. -->
- None.

## Impact

- **New project scaffolding**: `go.mod` (currently only `go.sum` exists), a Cobra command tree, and an internal Confluence REST client (net/http; no external SDK).
- **Dependencies** (already staged in `go.sum`): `spf13/cobra` (CLI), `yuin/goldmark` (Markdown parsing/AST → custom storage-format renderer), `gopkg.in/yaml.v3` (frontmatter + config), `golang.org/x/term` (hidden token prompt).
- **External API**: Confluence Cloud REST API v2 (`/wiki/api/v2/pages`), Basic auth with `email:api_token`. Requires per-site API tokens from `id.atlassian.com`.
- **On-disk footprint**: a global credential store at `~/.config/acli-plus/` (chmod 0600) and an optional `acli-plus.yaml` per repo.
- **Security**: secrets stored plaintext-in-file at 0600 for v1 (OS keychain noted as a follow-up); tokens must never be written into the git-tracked project config.
