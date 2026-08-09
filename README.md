# acli-plus

Publish Markdown files to **Confluence Cloud** from the command line.

Atlassian's official [`acli`](https://developer.atlassian.com/cloud/acli/) covers
Jira only — it has no Confluence create/update/delete. `acli-plus` fills that
gap by talking to the Confluence REST API (`/wiki/api/v2`) directly: point it at
a Markdown file and a Confluence URL, and it renders the file to Confluence
storage format and writes the page.

```bash
acli-plus create docs/onboarding.md https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook
# created "onboarding" -> https://acme.atlassian.net/wiki/pages/viewpage.action?pageId=120033
```

- Single static binary, no runtime dependencies (Go 1.26, `CGO_ENABLED=0`).
- Credentials stored **per Confluence host**, so one machine can publish to many sites.
- Safe by default: `--dry-run` everywhere, confirmation before overwriting pages
  edited outside the tool, and `delete` only moves pages to the trash.

---

## Table of contents

- [Install](#install)
- [Quick start](#quick-start)
- [Command reference](#command-reference)
- [Global flags](#global-flags)
- [Page references (what you can paste as `<url>`)](#page-references-what-you-can-paste-as-url)
- [Titles and frontmatter](#titles-and-frontmatter)
- [Markdown support](#markdown-support)
- [Configuration](#configuration)
- [Conflict handling](#conflict-handling)
- [Output and exit codes](#output-and-exit-codes)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Releasing (maintainers)](#releasing-maintainers)

---

## Install

### One-liner (macOS / Linux, recommended)

Downloads the right prebuilt binary from the latest GitHub release. No Go, no
clone, no token — the GitHub repo is public.

```bash
curl -fsSL https://raw.githubusercontent.com/DucThanh1997/acli-plus/main/install.sh | sh
```

The installer picks `/usr/local/bin` (using `sudo` if needed) and falls back to
`~/.local/bin`. On macOS it also strips the Gatekeeper quarantine flag so the
unsigned binary runs.

Environment overrides:

| Variable | Default | Effect |
|---|---|---|
| `ACLI_PLUS_VERSION` | `latest` | Pin a version, e.g. `0.3` (resolves to tag `v0.3`). |
| `INSTALL_DIR` | `/usr/local/bin` | Target directory, e.g. `~/bin`. |
| `ACLI_PLUS_BASE_URL` | GitHub repo URL | Point at a different repo/mirror (GitHub or GitLab). |
| `ACLI_PLUS_TOKEN` | — | Read token, only needed for a **private GitLab** mirror. |

```bash
# pin a version into a user directory
curl -fsSL https://raw.githubusercontent.com/DucThanh1997/acli-plus/main/install.sh \
  | ACLI_PLUS_VERSION=0.3 INSTALL_DIR="$HOME/.local/bin" sh
```

### Inside an ai-kit / bootstrap script

Idempotent — installs only when missing, always the latest release:

```sh
if ! command -v acli-plus >/dev/null 2>&1; then
  echo "Installing acli-plus..."
  curl -fsSL https://raw.githubusercontent.com/DucThanh1997/acli-plus/main/install.sh | sh
fi
acli-plus version
```

### From a checkout

```bash
git clone https://github.com/DucThanh1997/acli-plus.git
cd acli-plus
./install.sh          # uses dist/ if prebuilt, else downloads, else builds with Go
```

Resolution order inside `install.sh`: local `dist/<asset>` → release download →
build from source (only if Go is installed and you are in the repo).

### From the internal GitLab mirror (private, needs a token)

Only if you cannot reach GitHub. Use a Project/Group **read** token
(`read_api` + `read_repository`):

```bash
TOK=glpat-READ_TOKEN
curl -fsSL --header "PRIVATE-TOKEN: $TOK" \
  https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus/-/raw/main/install.sh \
  | ACLI_PLUS_BASE_URL=https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus \
    ACLI_PLUS_TOKEN=$TOK sh
```

### Manual download

Grab a binary from [Releases](https://github.com/DucThanh1997/acli-plus/releases)
— assets are named `acli-plus_<os>_<arch>` (`darwin`/`linux` × `amd64`/`arm64`,
plus `acli-plus_windows_amd64.exe`).

```bash
curl -fsSL -o acli-plus \
  https://github.com/DucThanh1997/acli-plus/releases/latest/download/acli-plus_darwin_arm64
chmod +x acli-plus
xattr -d com.apple.quarantine acli-plus 2>/dev/null   # macOS only
sudo mv acli-plus /usr/local/bin/
```

**Windows:** the shell installer is POSIX `sh` and does not handle the `.exe`
suffix — download `acli-plus_windows_amd64.exe` from Releases, rename it to
`acli-plus.exe`, and put it on your `PATH`.

### Build from source

Requires Go 1.26+.

```bash
make build            # -> ./bin/acli-plus
make install          # runs ./install.sh
```

### Upgrade / uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/DucThanh1997/acli-plus/main/install.sh | sh   # upgrade
sudo rm /usr/local/bin/acli-plus                                                          # uninstall
rm -rf ~/.config/acli-plus                                                                # + stored credentials
```

---

## Quick start

```bash
# 1. Register a Confluence site (token is typed hidden, stored per host)
acli-plus setup

# 2. Create a page as a child of a parent page (paste the PARENT's URL)
acli-plus create onboarding.md \
  https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook

# 3. Update that page later (paste the PAGE's own URL)
acli-plus update onboarding.md \
  https://acme.atlassian.net/wiki/spaces/DEV/pages/120033/Onboarding

# 4. Preview before touching anything
acli-plus update onboarding.md <page-url> --dry-run

# 5. Delete (moves to trash — reversible)
acli-plus delete https://acme.atlassian.net/wiki/spaces/DEV/pages/120033/Onboarding
```

---

## Command reference

| Command | Arguments | What it does |
|---|---|---|
| [`setup`](#acli-plus-setup) | — | Stores site URL, email, API token for one Confluence host. |
| [`create`](#acli-plus-create-filemd-url) | `<file.md> <url>` | `<url>` is the **parent** (page or space). Creates a child page. |
| [`update`](#acli-plus-update-filemd-url) | `<file.md> <url>` | `<url>` is the **target page**. Overwrites it in place. |
| [`delete`](#acli-plus-delete-url) | `<url>` | Moves the page at `<url>` to the trash. |
| [`version`](#acli-plus-version) | — | Prints the version. |

`acli-plus --help` and `acli-plus <command> --help` print the same information.

### `acli-plus setup`

Interactive. Asks for three things and stores them keyed by host:

```
$ acli-plus setup
Confluence site URL (e.g. https://your-team.atlassian.net): https://acme.atlassian.net
Atlassian account email: you@acme.com
API token (create at https://id.atlassian.com/manage-profile/security/api-tokens): ********
Saved credentials for acme.atlassian.net to /Users/you/.config/acli-plus/credentials.yaml
```

- The token is read **without echo** when stdin is a terminal.
- Credentials are verified against the site before being saved.
- Run it once per Confluence site; entries are keyed by host and never duplicated.
- Create the API token at <https://id.atlassian.com/manage-profile/security/api-tokens>
  (Basic auth = account email + API token). Tokens are stored locally only and
  never written into project files.

### `acli-plus create <file.md> <url>`

`<url>` is the **parent**:

- a **page URL** → the new page becomes a child of that page;
- a **space URL** → the new page is created at the space root.

```bash
# child of a specific page
acli-plus create guide.md https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook

# at the root of a space
acli-plus create guide.md https://acme.atlassian.net/wiki/spaces/DEV

# preview only
acli-plus create guide.md https://acme.atlassian.net/wiki/spaces/DEV --dry-run
```

**Idempotent by title.** Confluence titles are unique per space, so `create`
first looks for a page with the same title *anywhere in that space*. If one
exists, it is **updated** instead of duplicated — re-running `create` in a script
is safe.

### `acli-plus update <file.md> <url>`

`<url>` must identify a **specific page** (it needs a page id). The page is
overwritten in place and can be renamed — the new title comes from the file
(see [Titles and frontmatter](#titles-and-frontmatter)).

```bash
acli-plus update guide.md https://acme.atlassian.net/wiki/spaces/DEV/pages/120033/Guide
acli-plus update guide.md 120033 --site acme.atlassian.net      # bare page id
acli-plus update guide.md <page-url> --yes                      # no prompts (CI)
```

**Fallback:** if the page id no longer resolves (deleted, purged), the content is
created at the root of the URL's space instead, and a warning is printed:

```
warning: target page id not found; inserted into space DEV
created "Guide" -> https://acme.atlassian.net/wiki/pages/viewpage.action?pageId=120099
```

### `acli-plus delete <url>`

Moves the page to the Confluence trash — reversible from the UI, not a purge.
Asks for confirmation unless `--yes`/`--force`.

```bash
acli-plus delete https://acme.atlassian.net/wiki/spaces/DEV/pages/120033/Guide
```

```
Delete (move to trash) page "Guide" (id 120033)? [y/N]: y
deleted (moved to trash) "Guide"
```

Answering anything but `y`/`yes` aborts with `aborted; no changes made`.

### `acli-plus version`

```bash
$ acli-plus version
acli-plus 0.1.0-dev
```

The version is baked in at build time (`-ldflags -X …/internal/cmd.version`).
Released binaries report their release number; a locally built one reports
`0.1.0-dev` unless you pass `VERSION=` to `make build`.

---

## Global flags

Available on every command.

| Flag | Effect |
|---|---|
| `--dry-run` | Print what *would* happen (`[dry-run] created …`) and write nothing. Still contacts Confluence to resolve the target. |
| `--yes` | Skip all confirmation prompts. Use in CI/scripts. |
| `--force` | Overwrite even if the page was last edited outside acli-plus. (Also skips prompts.) |
| `--site` | Confluence site (host or full URL) to use when the argument carries no host — e.g. with a bare page id. |

```bash
acli-plus update notes.md 120033 --site https://acme.atlassian.net --yes
acli-plus delete <page-url> --dry-run
```

---

## Page references (what you can paste as `<url>`)

| Form | Example | Yields |
|---|---|---|
| Page URL | `https://acme.atlassian.net/wiki/spaces/DEV/pages/120033/Guide` | host + space + page id |
| Space URL | `https://acme.atlassian.net/wiki/spaces/DEV` | host + space (no page) |
| Bare page id | `120033` | page id only — host must come from `--site`, `ACLI_PLUS_SITE`, or `acli-plus.yaml` |
| Short link `/wiki/x/…` | `https://acme.atlassian.net/wiki/x/AbCd` | ❌ rejected — paste the full page URL |

`create` accepts a page or space ref. `update` and `delete` require a ref with a
page id. Personal-space keys (`~712020abc`) and URL-encoded segments work.

---

## Titles and frontmatter

The page title is:

1. the frontmatter `title:` if present, otherwise
2. the **file name** without its extension.

A leading `# H1` is **not** used as the title — it stays in the body as normal
content, so what you see in the file is what lands on the page.

```markdown
---
title: Onboarding Guide      # optional; without it the title is "onboarding"
---

# This heading stays in the body

Body content starts here…
```

So `acli-plus create onboarding.md <url>` with no frontmatter creates a page
titled `onboarding`. Since `update` also applies the title, renaming a page is
just changing `title:` and re-running `update`.

> The `space:` and `parent:` frontmatter keys are parsed but not yet acted on —
> the target always comes from the URL argument.

---

## Markdown support

Rendered to Confluence storage format via [goldmark](https://github.com/yuin/goldmark)
with GitHub Flavored Markdown enabled.

| Supported | Notes |
|---|---|
| Headings `#`–`######` | → `<h1>`–`<h6>` |
| Paragraphs, line breaks | hard breaks → `<br/>` |
| Bold, italic, `~~strikethrough~~` | |
| Inline code, fenced & indented code blocks | plain code macro, no syntax highlighting yet |
| Links and autolinks | |
| Bullet and ordered lists (nested) | |
| Task lists `- [ ]` / `- [x]` | → native Confluence `<ac:task-list>` checkboxes |
| Tables (GFM) | |
| Blockquotes | |
| Horizontal rules `---` | |

| Not supported (v1) | Behaviour |
|---|---|
| Images / attachments | skipped with `warning: image skipped …` |
| Raw HTML (block or inline) | silently dropped |
| Code-block syntax highlighting | rendered as plain code |
| Labels, whole-directory publishing, named profiles | not implemented |
| Jira and other Atlassian products | out of scope — Confluence only |

---

## Configuration

### Credentials

Stored at `$XDG_CONFIG_HOME/acli-plus/credentials.yaml`, or
`~/.config/acli-plus/credentials.yaml` when `XDG_CONFIG_HOME` is unset.
Directory `0700`, file `0600` (re-enforced on every save).

```yaml
hosts:
  acme.atlassian.net:
    email: you@acme.com
    token: <api-token>
  other-team.atlassian.net:
    email: you@acme.com
    token: <api-token>
```

Run `acli-plus setup` once per site. Because a pasted URL already carries its
host, the matching token is selected automatically — no profile switching.

### Host resolution order

1. host in the `<url>` argument
2. `--site`
3. `ACLI_PLUS_SITE` environment variable
4. `site:` in `acli-plus.yaml` in the current directory

If none resolve, the command stops with
`no Confluence site specified: pass a URL, --site, ACLI_PLUS_SITE, or set 'site' in acli-plus.yaml`.

### Per-project file (`acli-plus.yaml`)

Optional, **non-secret**, safe to commit — never put a token here:

```yaml
site: https://acme.atlassian.net
```

Only `site` is honoured today; `space` and `parent` are reserved for a future
version.

---

## Conflict handling

Every write stamps the Confluence version message with `via acli-plus`. On an
overwrite, if the page's latest version was **not** stamped that way — someone
edited it in the Confluence UI — the command stops and asks:

```
Page "Guide" (id 120033, version 7) was modified outside acli-plus. Overwrite? [y/N]:
```

Decline and nothing is written (`aborted; no changes made`). Pass `--yes` or
`--force` to skip the check, or `--dry-run` to inspect first. Confluence keeps
full version history either way, so an accidental overwrite is recoverable from
the page's version list.

---

## Output and exit codes

- Results go to **stdout**: `created "Title" -> <link>`, `updated "Title" -> <link>`,
  `deleted (moved to trash) "Title"`, or `aborted; no changes made`.
- With `--dry-run`, every line is prefixed `[dry-run] `.
- Warnings and prompts go to **stderr** (`warning: image skipped …`).
- On failure: `error: <message>` on stderr, exit code **1**. Success is **0**.

---

## Troubleshooting

| Message | Cause / fix |
|---|---|
| `no credentials for host; run 'acli-plus setup' (host acme.atlassian.net)` | That host was never registered. Run `acli-plus setup` for it. |
| `no Confluence site specified: …` | Bare page id with no host source. Add `--site`, `ACLI_PLUS_SITE`, or `acli-plus.yaml`. |
| `confluence: authentication failed` | Bad/revoked API token, wrong email, or no permission on the space. Re-run `acli-plus setup`; check the token at <https://id.atlassian.com/manage-profile/security/api-tokens>. |
| `confluence: short /wiki/x/ links are not supported; paste the full page URL` | Open the short link in a browser and copy the resulting `/wiki/spaces/…/pages/…` URL. |
| `confluence: not a recognizable Confluence URL or page id` | The argument isn't a Confluence URL or a numeric id. |
| `confluence: page not found` on `delete` | The page is already trashed, or the id belongs to another site. |
| `update needs a page URL (with a page id)` | You passed a space URL to `update`/`delete`. Use `create` for a space target. |
| `confluence: space not found: DEV` | Wrong space key, or your account can't see that space. |
| `confluence api 4xx: …` | Raw API error passed through — usually a permissions or payload issue. |
| macOS: *"cannot be opened because the developer cannot be verified"* | The binary is unsigned. `xattr -d com.apple.quarantine $(which acli-plus)` — `install.sh` does this for you. |
| `NOTE: … is not on your PATH` | Add the printed line to your shell profile, e.g. `export PATH="$HOME/.local/bin:$PATH"`. |

---

## Development

```bash
make build     # build ./bin/acli-plus for this machine
make test      # go test ./...
make fmt       # gofmt -w .
make vet       # go vet ./...
make release   # cross-compile every platform into ./dist
make install   # run ./install.sh
make clean     # remove bin/ and dist/
```

Layout — four layers, dependencies point inward:

```
main.go
internal/
  cmd/                      handler layer — Cobra commands, flags, output
  app/                      use cases — create/update/delete, setup, rendering
  domain/confluence/        entities, ports (Gateway), URL parsing, errors
  gateway/confluencerest/   REST adapter for /wiki/api/v2
  markdown/                 Markdown → Confluence storage format (pure, no I/O)
  config/                   credential store, host resolution, project file
scripts/                    build.sh, publish-release.sh, push-all.sh
```

`domain` and `markdown` have no I/O, so they are unit-tested directly; `app` is
tested against a fake `Gateway`.

---

## Releasing (maintainers)

### GitHub (primary distribution)

Tag and push — [`.github/workflows/release.yml`](.github/workflows/release.yml)
runs the tests, cross-compiles every platform, and publishes the release that
`install.sh` reads:

```bash
git tag v0.4
./scripts/push-all.sh          # pushes the branch + tags to github AND gitlab
```

### GitLab (internal mirror, no runner needed)

Build locally and publish straight to GitLab with a token scoped `api`
(role Maintainer). Keep it in a gitignored `.publish.env`:

```bash
cp .publish.env.example .publish.env    # paste GITLAB_TOKEN=glpat-…
./scripts/publish-release.sh            # auto-bump: 0.1 → 0.2 → … → 0.9 → 1.0
./scripts/publish-release.sh 1.2        # or force a version
```

It uploads every platform binary to the generic package registry and creates the
release, giving permanent asset links plus a `permalink/latest` that
`install.sh`'s GitLab mode uses. [`.gitlab-ci.yml`](.gitlab-ci.yml) is optional —
only needed if you would rather have CI publish on tag.

### Offline bundle (machines with neither Go nor network access to releases)

```bash
./scripts/build.sh
tar czf acli-plus-dist.tgz dist install.sh
# recipient: tar xzf acli-plus-dist.tgz && ./install.sh
```

### Pushing to both remotes

```bash
./scripts/push-all.sh
```

`github` = <https://github.com/DucThanh1997/acli-plus> (public, distribution),
`gitlab` = <https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus> (internal mirror).
