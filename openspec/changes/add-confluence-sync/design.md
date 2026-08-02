## Context

`acli-plus` is a greenfield Go CLI (today the project holds only `go.sum` and `.idea`). It exists because Atlassian's official `acli` covers Jira, admin, and Rovo but has **no Confluence commands** — so Confluence cannot be delivered by wrapping `acli`; acli-plus must talk to the Confluence Cloud REST API directly. The dependencies already staged in `go.sum` fix the stack: `spf13/cobra` (commands), `yuin/goldmark` (Markdown AST), `gopkg.in/yaml.v3` (frontmatter + config), `golang.org/x/term` (hidden token prompt).

Primary users keep Markdown docs in git and want to publish them to Confluence, sometimes across **multiple Confluence sites** (e.g. one per client). v1 is deliberately text-first.

## Goals / Non-Goals

**Goals:**
- `setup` + `create`/`update`/`delete` for Confluence pages, driven by a pasted URL.
- Predictable upsert semantics where the verb decides the URL's role.
- Multiple Confluence sites usable side by side, with secrets kept out of git.
- Markdown → Confluence storage format for the supported element set.
- A safe delete (trash + confirm) and a stateless "someone edited this outside the tool" warning.

**Non-Goals (v1):**
- Image/attachment upload, code-macro syntax highlighting, label sync.
- Whole-directory push / tree mirroring, named profiles, short `/wiki/x/` links.
- OS keychain storage, Jira delegation, any Atlassian product other than Confluence.

## Decisions

1. **Direct REST client, not an acli wrapper.** acli has zero Confluence surface, so wrapping it cannot meet the priority. Use `net/http` against Confluence Cloud REST **API v2** (`/wiki/api/v2/pages`) with Basic auth (`email:api_token`). Space key → numeric space-id is resolved once and cached per run. *Alternative:* REST v1 `/wiki/rest/api/content` (easier title search) — kept as a documented fallback if v2 title lookup proves awkward.

2. **Flat top-level verbs now, nested groups later.** `acli-plus setup|create|update|delete` matches the issue's ergonomics. A `confluence`/`jira` command group is reserved for when other products arrive (verbs can become aliases). *Alternative:* nested-only (`acli-plus confluence create …`) — more scalable but more typing; rejected for v1.

3. **Address pages by pasted URL; the verb decides the URL's role.** `create <file> <url>` → `<url>` is the **parent**; `update <file> <url>` → `<url>` is the **target page**. Because a Confluence URL carries host + space key + page id, multi-site "just works" (the host in the URL selects credentials). *Alternatives:* human path resolver that tree-walks titles (fragile, breaks on duplicate/slashed titles) and explicit `--space/--title` flags (kept as a possible scripting fallback).

4. **Upsert with no persisted identity.** `create` matches an existing child by **title under the parent** (update if found, else create); `update` targets the **page id from the URL** (in-place, can rename). Renaming through `create` therefore produces a new page — explicitly accepted. *Alternatives:* frontmatter `id` write-back or a `.acli-plus.lock` manifest — both rejected to keep `.md` files clean and avoid extra state.

5. **Stateless conflict detection via a version-message sentinel.** Every write stamps a recognizable version message (e.g. `via acli-plus`). On `update`, if the page's latest version was **not** stamped by acli-plus, the page was edited in the UI → stop, warn, confirm `[y/N]`; `--force`/`--yes` skips. This gives the requested "stop & warn on external edits" without any local state. *Alternatives:* a lockfile storing the last-pushed version (rejected — stateful) or silent last-write-wins (rejected — no warning).

6. **Two-layer config.** Global per-host credential store at `~/.config/acli-plus/credentials.yaml` (chmod 0600, one entry per host) holds secrets; an optional per-project `acli-plus.yaml` (non-secret, committable) holds `site` + default `space`/`parent`. **Host resolution:** a URL's host is authoritative for that command; otherwise `--site` flag → env var → project config. The token is always looked up by the resolved host. *Alternative:* a single global site — rejected because each project may target a different site.

7. **Markdown → storage via a custom goldmark renderer.** goldmark parses to an AST; a custom `NodeRenderer` emits Confluence storage-format XHTML. Plain MD→HTML is insufficient because storage format diverges for code fences (`<pre>`), tables, and task lists, and invalid bodies are rejected by the API. Output must be well-formed and entity-escaped.

8. **Title resolution:** frontmatter `title` → file name (without extension). A leading `# H1` is left in the body as normal content — it is not used as the title and not stripped. *Rationale:* the page title should be predictable from the file being published; inferring it from an H1 surprised users and coupled the title to body content.

9. **Delete → trash + confirm.** Cloud delete moves to trash (reversible), gated by `[y/N]` (`--yes` skips). Permanent purge is out of scope.

## Risks / Trade-offs

- **Rename via `create` creates a duplicate** → documented; `update <file> <url>` renames in place; behavior accepted by the user.
- **Push overwrites manual UI edits** (file is source of truth) → mitigated by the version-message conflict prompt; `--force` to override intentionally.
- **goldmark→storage fidelity gaps** → v1 element set is bounded; unsupported elements warn & skip; output is validated as well-formed XHTML.
- **Plaintext token in a 0600 file** → acceptable for v1; OS keychain is a noted follow-up; tokens must never be written to project config.
- **v2 uses numeric space-id, URLs give space key** → resolve+cache space-id from key; clear error when the space is not found.
- **Title uniqueness collisions** → Confluence enforces unique titles per space; surface API 400s with a clear message.
- **Optimistic-lock race (409 on PUT)** → fetch the fresh version immediately before writing; on 409, re-fetch and re-apply the conflict prompt.

## Open Questions

- Secret storage: OS keychain vs the 0600 file (v1 uses the file).
- Whether to ship `--space/--title` scripting flags in v1 or defer.
- Whether any v2 title-search limitation forces the documented REST v1 fallback.
