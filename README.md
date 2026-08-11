# acli-plus

One command-line tool for **Confluence Cloud** and **Jira Cloud**.

Atlassian's official [`acli`](https://developer.atlassian.com/cloud/acli/) covers
Jira only — it has no Confluence create/update/delete. `acli-plus` fills that gap
by talking to the Confluence REST API (`/wiki/api/v2`) directly, and then covers
Jira too (`/rest/api/3` and `/rest/agile/1.0`) with the same command names acli
uses — so you get both halves of an Atlassian site from one binary, and `acli`
itself does not need to be installed.

```bash
acli-plus create docs/onboarding.md https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook
# created "onboarding" -> https://acme.atlassian.net/wiki/pages/viewpage.action?pageId=120033

acli-plus jira workitem create -p TEAM -t Task -s "Fix login redirect" -a @me
# created TEAM-142 "Fix login redirect" -> https://acme.atlassian.net/browse/TEAM-142
```

- Single static binary, no runtime dependencies (Go 1.26, `CGO_ENABLED=0`).
- **One credential for both products.** Confluence and Jira share a site, an
  account, and an API token, so `acli-plus setup` is all either side needs.
- Credentials stored **per host**, so one machine can work with many sites.
- Safe by default: `--dry-run` everywhere, confirmation before destructive
  writes, and Confluence `delete` only moves pages to the trash.

---

## Table of contents

- [Install](#install)
- [Quick start](#quick-start)
- [Command reference](#command-reference)
- [Jira commands](#jira-commands)
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
# 1. Register an Atlassian site once — this covers Confluence *and* Jira
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

# 6. Same site, Jira side — no extra login
acli-plus jira workitem create -p TEAM -t Bug -s "Login loops on Safari" -a @me
acli-plus jira workitem search --jql "assignee = currentUser() AND statusCategory != Done"
acli-plus jira workitem transition TEAM-142 --status "In Progress"
```

---

## Command reference

| Command | Arguments | What it does |
|---|---|---|
| [`setup`](#acli-plus-setup) | — | Stores site URL, email, API token for one Atlassian host. |
| [`create`](#acli-plus-create-filemd-url) | `<file.md> <url>` | `<url>` is the **parent** (page or space). Creates a child page. |
| [`update`](#acli-plus-update-filemd-url) | `<file.md> <url>` | `<url>` is the **target page**. Overwrites it in place. |
| [`delete`](#acli-plus-delete-url) | `<url>` | Moves the page at `<url>` to the trash. |
| [`jira`](#jira-commands) | *see below* | Work items, projects, boards, sprints, filters, dashboards. |
| [`version`](#acli-plus-version) | — | Prints the version. |

`acli-plus --help` and `acli-plus <command> --help` print the same information.

### `acli-plus setup`

Interactive. Asks for three things and stores them keyed by host:

```
$ acli-plus setup
Atlassian site URL (e.g. https://your-team.atlassian.net): https://acme.atlassian.net
Atlassian account email: you@acme.com
API token (create at https://id.atlassian.com/manage-profile/security/api-tokens): ********
Saved credentials for acme.atlassian.net to /Users/you/.config/acli-plus/credentials.yaml
Reachable on this site: Confluence, Jira
```

- The token is read **without echo** when stdin is a terminal.
- Credentials are checked against **both** Confluence and Jira before being
  saved, and the last line reports which ones answered. A site licensed for only
  one product still works — the credentials are accepted as long as one product
  does, and only the commands for the other one will fail.
- Run it once per site; entries are keyed by host and never duplicated.
- **No separate Jira login.** One site, one account, one API token covers both.
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

## Jira commands

Everything lives under `acli-plus jira`. Command and flag names mirror
Atlassian's `acli`, so anything written for `acli jira …` works by swapping the
binary — and `acli` does not have to be installed.

Authentication is the credential from `acli-plus setup`. Jira and Confluence are
the same site, the same account, and the same API token.

### Work items

`acli-plus jira workitem <command>` (aliases: `issue`, `wi`).

| Command | What it does |
|---|---|
| `create` | Create one work item. |
| `create-bulk` | Create many from a JSON file (`--generate-json` prints a template). |
| `view <key>` | Show one work item; `--fields` narrows what is fetched. |
| `search` | Find work items with `--jql`. |
| `edit` | Change fields on one or many. |
| `delete` | Permanently delete (confirms first). |
| `clone` | Copy into new work items. |
| `archive` / `unarchive` | Archive or restore — **Jira Premium and Enterprise only**. |
| `assign` | Set or clear the assignee. |
| `transition` | Move to another status; `--list` shows what is reachable. |
| `link` | Link two work items; `--list-types` shows the configured types. |
| `comment-create` / `comment-list` / `comment-update` / `comment-delete` | Manage comments. |
| `comment-visibility` | Restrict a comment to a group or role, or `--public` to lift it. |
| `attachment-list` / `attachment-delete` | List or remove attachments. |
| `watcher-list` / `watcher-remove` | See and remove watchers. |

`watcher-list` and `field list` are the two additions to acli's surface — you
need them to find the ids the matching `remove`/`delete` commands take.

#### Choosing which work items to act on

Every bulk command (`edit`, `delete`, `clone`, `assign`, `transition`,
`archive`, `unarchive`) accepts all three forms, and combines them:

```bash
acli-plus jira workitem edit TEAM-1 TEAM-2 --label triaged     # positional
acli-plus jira workitem edit --key TEAM-1,TEAM-2 --label triaged
acli-plus jira workitem edit --jql "project = TEAM AND labels = old" --label triaged
```

Keys are case-insensitive, and a pasted URL works anywhere a key does —
`https://acme.atlassian.net/browse/TEAM-1`, or a board URL with
`?selectedIssue=TEAM-1`. A URL also **selects the site**, exactly like a
Confluence page URL does.

#### Setting fields

Named flags cover the common fields:

```bash
acli-plus jira workitem create \
  -p TEAM -t Story -s "Checkout rewrite" \
  -a ann@acme.com --label backend,q3 --priority High \
  --due 2026-09-30 --parent TEAM-100 --component api
```

Anything else — including every custom field — goes through `--field`, which
takes the field's **display name or id** and shapes the value to match the
field's type:

```bash
acli-plus jira workitem create -p TEAM -t Story -s "…" \
  --field "Story Points=5" \
  --field "Severity=High" \
  --field customfield_10050=ios,android
```

| Field type | What you type | What is sent |
|---|---|---|
| string, date | `Summary=hello` | `"hello"` |
| number | `Story Points=5` | `5` |
| user | `Team Lead=@me` | `{"accountId": "…"}` |
| option | `Severity=High` | `{"value": "High"}` |
| array of strings | `Labels=a,b` | `["a","b"]` |
| array of options/versions | `Platforms=ios,android` | `[{"value":"ios"},…]` |
| anything else | `Field={"value":"X"}` | passed through as JSON |

An empty value (`--field Severity=`) clears the field. Ambiguous names are an
error listing the candidate ids, never a guess.

People can be given as `@me`, an email, a display name, or an account id. A name
that matches several accounts is reported rather than picked.

#### Descriptions and comments

Descriptions and comments are stored as Atlassian Document Format. You write
Markdown; acli-plus converts it:

```bash
acli-plus jira workitem create -p TEAM -t Task -s "Ship it" \
  -d "Blocked on **infra**. See [the RFC](https://example.com)."

# summary and description from one Markdown file:
# the frontmatter title (or leading H1) becomes the summary
acli-plus jira workitem create -p TEAM -t Story --from-file docs/story.md

# open $EDITOR — first line is the summary, the rest the description
acli-plus jira workitem create -p TEAM -t Task -e
```

Headings, bold/italic/strikethrough, inline and fenced code (with language),
links, nested lists, task lists, tables, blockquotes and rules all convert.
Images become links, with a warning, because ADF images need an uploaded media
id that a Markdown file cannot supply. A file that already holds an ADF document
is passed through untouched.

#### Searching

```bash
acli-plus jira workitem search --jql "project = TEAM AND status = 'In Progress'"
acli-plus jira workitem search --jql "assignee = currentUser()" --paginate --csv
acli-plus jira workitem search --jql "project = TEAM" --fields summary,status --json
```

Jira's current search API is cursor-paginated and **reports no total**, so by
default you get one page. `--paginate` walks every page; `--limit` stops after a
count.

### Projects, fields, boards, sprints, filters, dashboards

```bash
acli-plus jira project list --type software
acli-plus jira project view TEAM
acli-plus jira project create --key TEAM --name "Team Space" --type software
acli-plus jira project update TEAM --name "New name"
acli-plus jira project archive TEAM        # Premium/Enterprise
acli-plus jira project restore TEAM
acli-plus jira project delete TEAM         # trashed by default; --no-undo is immediate

acli-plus jira field list --custom --query points   # find customfield ids
acli-plus jira field create --name Team --type com.atlassian.jira.plugin.system.customfieldtypes:textfield
acli-plus jira field delete "Team"
acli-plus jira field cancel-delete "Team"

acli-plus jira board search --project TEAM
acli-plus jira board list-sprints --board 42 --state active
acli-plus jira sprint list-workitems --sprint 128
acli-plus jira sprint list-workitems --sprint "Sprint 7" --board 42

acli-plus jira filter list --favourites
acli-plus jira filter search --name sprint
acli-plus jira filter add-favourite 10001
acli-plus jira filter change-owner 10001 --owner ann@acme.com

acli-plus jira dashboard search --name "Team health"
```

Boards, sprints and filters accept a **name or an id**; a name that matches more
than one is reported with the ids to choose from.

### Output

Read commands print an aligned table. `--json` prints the untouched API payload
(so custom fields are all there), and listing commands also take `--csv`.

```bash
acli-plus jira workitem view TEAM-1 --json | jq '.fields.customfield_10016'
acli-plus jira workitem search --jql "project = TEAM" --paginate --csv > backlog.csv
```

### What is not covered

| Not supported | Why |
|---|---|
| Uploading attachments | Not in acli's surface either; `attachment-list`/`-delete` are. |
| Creating or closing sprints | acli exposes only `sprint list-workitems`. |
| `acli jira auth` | `acli-plus setup` is the equivalent, shared with Confluence. |

Archiving work items and projects needs Jira Premium or Enterprise; on other
plans acli-plus reports that the operation is not available on your plan rather
than a confusing 404.

---

## Global flags

Available on every command.

| Flag | Effect |
|---|---|
| `--dry-run` | Print what *would* happen (`[dry-run] created …`) and write nothing. Still contacts the site to resolve the target. |
| `--yes` | Skip all confirmation prompts. Use in CI/scripts. |
| `--force` | Overwrite even if the page was last edited outside acli-plus. (Also skips prompts.) |
| `--site` | Atlassian site (host or full URL) to use when the argument carries no host — e.g. a bare page id or a bare work item key. |

```bash
acli-plus update notes.md 120033 --site https://acme.atlassian.net --yes
acli-plus delete <page-url> --dry-run
acli-plus jira workitem delete TEAM-1 --dry-run
acli-plus jira workitem delete TEAM-1 --yes    # no prompt, for scripts
```

On the Jira side the prompts guard the irreversible commands — `workitem delete`,
`comment-delete`, `attachment-delete`, `project delete` and `field delete`.
Unlike a Confluence page, a deleted work item does **not** go to a trash.

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

Parsed with [goldmark](https://github.com/yuin/goldmark) and GitHub Flavored
Markdown enabled, then rendered to Confluence storage format for pages, or to
Atlassian Document Format for Jira descriptions and comments. The table below
describes the Confluence renderer; see
[Descriptions and comments](#descriptions-and-comments) for the Jira side, which
covers the same constructs.

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

Run `acli-plus setup` once per site — the entry serves Confluence and Jira
alike. Because a pasted URL already carries its host, the matching token is
selected automatically, whether it is a wiki page URL or a `/browse/TEAM-1` link.

### Host resolution order

1. host in the `<url>` argument (or in a pasted work item URL)
2. `--site`
3. `ACLI_PLUS_SITE` environment variable
4. `site:` in `acli-plus.yaml` in the current directory
5. the only registered host, when exactly one is registered

Step 5 exists because most Jira commands carry no URL to take a host from —
`jira project list` has nothing to parse — and a machine set up against a single
site should not have to repeat `--site` on every call.

With **no** site registered the command stops with `no Atlassian site specified
and none registered: run 'acli-plus setup', …`. With **several** registered and
nothing naming one, it stops with `several sites are registered; pass --site to
choose one (registered: …)` rather than guessing.

### Per-project file (`acli-plus.yaml`)

Optional, **non-secret**, safe to commit — never put a token here:

```yaml
site: https://acme.atlassian.net
jira_project: TEAM       # default for 'jira workitem create' and 'jira board search'
jira_board: 42           # default for the board and sprint commands
```

With `jira_project` set, `-p/--project` becomes optional:

```bash
acli-plus jira workitem create -t Task -s "Fix login redirect"
```

`space` and `parent` are reserved for a future version and are not honoured yet.

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
  `deleted (moved to trash) "Title"`, `created TEAM-142 "Summary" -> <link>`, or
  `aborted; no changes made`. Jira tables, JSON and CSV go to stdout too, so they
  pipe cleanly.
- With `--dry-run`, every line is prefixed `[dry-run] `.
- Warnings and prompts go to **stderr** (`warning: image skipped …`), as does
  `no results` for an empty table.
- On failure: `error: <message>` on stderr, exit code **1**. Success is **0**.

---

## Troubleshooting

| Message | Cause / fix |
|---|---|
| `no credentials for host; run 'acli-plus setup' (host acme.atlassian.net)` | That host was never registered. Run `acli-plus setup` for it. |
| `no Atlassian site specified and none registered: …` | Nothing named a site and none is registered. Run `acli-plus setup`. |
| `several sites are registered; pass --site to choose one` | More than one host in the credential store and nothing said which. Add `--site`, `ACLI_PLUS_SITE`, or `site:` in `acli-plus.yaml`. |
| `this board does not use sprints (Kanban boards have none)` | Sprints are a Scrum concept; the board you named is Kanban. |
| `the JQL query matched no work items: …` | The query ran but selected nothing, so there was nothing to edit or delete. |
| `confluence: authentication failed` | Bad/revoked API token, wrong email, or no permission on the space. Re-run `acli-plus setup`; check the token at <https://id.atlassian.com/manage-profile/security/api-tokens>. |
| `confluence: short /wiki/x/ links are not supported; paste the full page URL` | Open the short link in a browser and copy the resulting `/wiki/spaces/…/pages/…` URL. |
| `confluence: not a recognizable Confluence URL or page id` | The argument isn't a Confluence URL or a numeric id. |
| `confluence: page not found` on `delete` | The page is already trashed, or the id belongs to another site. |
| `update needs a page URL (with a page id)` | You passed a space URL to `update`/`delete`. Use `create` for a space target. |
| `confluence: space not found: DEV` | Wrong space key, or your account can't see that space. |
| `confluence api 4xx: …` | Raw API error passed through — usually a permissions or payload issue. |
| `authentication failed; check your email and API token` on `jira …` | The token is fine for Confluence but your account has no Jira access, or the token was revoked. `acli-plus setup` prints which products answered. |
| `not a work item key (expected e.g. TEAM-123) or a Jira work item URL` | The argument isn't a key or a Jira URL carrying one. |
| `no transition to that status is available from the current status (available: …)` | Jira only offers transitions valid from the current status. The message lists them; `workitem transition <key> --list` shows the same. |
| `field not found: "Story points"` | Run `acli-plus jira field list --query points` to see the exact name and id. |
| `"Ann" matches 3 accounts; pass an account id instead: …` | Ambiguous person. Use the email or the account id from the message. |
| `this operation is not available on your Jira plan` | Archiving work items and projects needs Premium or Enterprise. |
| `jira api 400: Field 'customfield_x' cannot be set` | The field isn't on the create/edit screen for that project and issue type. |
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

Layout — four layers, dependencies point inward. Confluence and Jira are two
products behind the same layering, sharing the credential store and the Markdown
parser:

```
main.go
internal/
  cmd/                      handler layer — Cobra commands, flags, output
    jira/                   the 'jira' command tree (injected via Deps)
  app/                      use cases — pages, setup, work items, catalogs
  domain/confluence/        entities, ports (Gateway), URL parsing, errors
  domain/jira/              entities, ports, work item key/URL parsing, errors
  gateway/confluencerest/   REST adapter for /wiki/api/v2
  gateway/jirarest/         REST adapter for /rest/api/3 and /rest/agile/1.0
  markdown/                 Markdown → Confluence storage format and → ADF (pure)
  config/                   credential store, host resolution, project file
scripts/                    build.sh, publish-release.sh, push-all.sh
```

`domain` and `markdown` have no I/O, so they are unit-tested directly; `app` is
tested against fake gateways, `gateway/jirarest` against an `httptest` server,
and `cmd/jira` asserts the command tree still matches acli's.

`make test` never touches the network. To check the Jira commands against a real
site, [`scripts/e2e-jira.sh`](scripts/e2e-jira.sh) drives every one of them and
deletes what it created on the way out;
[`scripts/e2e-jira.md`](scripts/e2e-jira.md) is the same run written out case by
case for checking things by hand.

```bash
./scripts/e2e-jira.sh --project TEAM --read-only   # reads and dry-runs only
./scripts/e2e-jira.sh --project TEAM               # full run, asks before writing
```

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
