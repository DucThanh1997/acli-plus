# acli-plus

Publish Markdown files to **Confluence Cloud** from the command line — the
Confluence create/update/delete that Atlassian's official [`acli`](https://developer.atlassian.com/cloud/acli/)
does not provide. `acli` covers Jira only; `acli-plus` fills the Confluence gap
by talking to the Confluence REST API directly.

## Install

### With the install script (recommended)

From the project folder:

```bash
./install.sh
```

It puts `acli-plus` on your PATH (default `/usr/local/bin`, falling back to
`~/.local/bin`). It uses a prebuilt binary from `dist/` if one matches your
machine (**no Go needed**); otherwise it builds from source when Go is present.
Override the target dir with `INSTALL_DIR=~/bin ./install.sh`.

### Distributing to machines without Go

The maintainer builds binaries for all platforms once (needs Go 1.26+):

```bash
./scripts/build.sh                       # -> dist/acli-plus_<os>_<arch>
tar czf acli-plus-dist.tgz dist install.sh
```

Send `acli-plus-dist.tgz`. The recipient (no Go required):

```bash
tar xzf acli-plus-dist.tgz
./install.sh
```

### End users: one-liner (no clone, no Go)

Once a release is published (see below), a user installs the binary directly —
no repo clone, no Go:

```bash
curl -fsSL https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus/-/raw/v0.1.0/install.sh \
  | ACLI_PLUS_VERSION=0.1.0 sh
```

`install.sh` detects the OS/arch and downloads the matching binary from the
GitLab release. For a **private** project, pass a token:
`... | ACLI_PLUS_VERSION=0.1.0 ACLI_PLUS_TOKEN=<token> sh`.

### Inside an ai-kit setup script

Add an idempotent install step to your kit's setup (pin the version so setups are
reproducible):

```sh
if ! command -v acli-plus >/dev/null 2>&1; then
  echo "Installing acli-plus..."
  curl -fsSL https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus/-/raw/v0.1.0/install.sh \
    | ACLI_PLUS_VERSION=0.1.0 sh
fi
acli-plus version
```

### Cutting a release (maintainer)

CI (`.gitlab-ci.yml`) builds every platform and publishes a GitLab Release on any
`v*` tag:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

The release exposes permanent asset links at
`…/-/releases/v0.1.0/downloads/acli-plus_<os>_<arch>`, which is exactly what
`install.sh` downloads. Set `BASE_URL` in `install.sh` (or `ACLI_PLUS_BASE_URL`)
to your GitLab project URL.

### Make targets

```bash
make build     # build ./bin/acli-plus for this machine
make install   # run install.sh
make release   # cross-compile every platform into ./dist
make test      # run the test suite
```

## Quick start

```bash
# 1. Register a Confluence site (stored per-host, token entered hidden)
acli-plus setup

# 2. Create a page as a child of a parent page (paste the parent's URL)
acli-plus create onboarding.md https://your-team.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook

# 3. Update that exact page later (paste the page's own URL)
acli-plus update onboarding.md https://your-team.atlassian.net/wiki/spaces/DEV/pages/120033/Onboarding

# 4. Delete a page (moves it to the trash; asks first)
acli-plus delete https://your-team.atlassian.net/wiki/spaces/DEV/pages/120033/Onboarding
```

## Commands

| Command | What it does |
|---|---|
| `setup` | Prompts for site URL, email, and API token; stores them keyed by host. |
| `create <file.md> <url>` | `<url>` is the **parent** (page or space). Creates a child; if a page with the same title already exists in that space, it is **updated** instead of duplicated. |
| `update <file.md> <url>` | `<url>` is the **target page**. Overwrites it in place and can rename it. If the page id no longer resolves, the content is inserted into the URL's space. |
| `delete <url>` | Moves the page at `<url>` to the trash (reversible). |
| `version` | Prints the version. |

### Global flags

| Flag | Effect |
|---|---|
| `--dry-run` | Show whether it would create/update/delete and the target — writes nothing. |
| `--yes` | Skip confirmation prompts. |
| `--force` | Overwrite even if the page was modified outside acli-plus. |
| `--site` | Confluence site (host or URL) to use when no URL is supplied. |

## How the title is chosen

The page title is the frontmatter `title` when present, otherwise the **file
name** (without extension). A leading `# H1` is kept in the body as normal
content — it is not used as the title.

```markdown
---
title: Onboarding Guide   # optional; without it, the file name is the title
---

# This heading stays in the body

Body content starts here…
```

## Multiple sites / per-project config

Credentials are stored **per host** (`~/.config/acli-plus/credentials.yaml`,
mode `0600`), so different projects can target different Confluence sites — run
`acli-plus setup` once per site. Because a pasted URL already carries its host,
the right token is selected automatically.

Optionally commit a **non-secret** `acli-plus.yaml` in a project to set defaults
(never put a token here):

```yaml
site: https://your-team.atlassian.net
space: DEV
```

Host resolution precedence: **URL host** → `--site` → `ACLI_PLUS_SITE` env →
`acli-plus.yaml`.

## Conflict handling

Every write stamps a version message. On `update`, if a page's latest version
was **not** written by acli-plus (i.e. someone edited it in the Confluence UI),
the command stops and asks before overwriting. Use `--force`/`--yes` to skip.

## Supported Markdown (v1)

Headings, paragraphs, lists, links, bold/italic, inline code, **tables**,
**fenced code blocks**, **blockquotes**, and **task lists** (checkboxes).

**Not yet supported (v1):** images/attachments (skipped with a warning),
code-macro syntax highlighting, label sync, pushing whole directories, named
profiles, and short `/wiki/x/` links. Confluence is the only Atlassian product
covered.

## Authentication

Uses Basic auth with your Atlassian account email + an API token created at
<https://id.atlassian.com/manage-profile/security/api-tokens>. Tokens are stored
locally only and never written to project files.
