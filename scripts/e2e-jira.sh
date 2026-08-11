#!/usr/bin/env sh
# e2e-jira.sh — end-to-end check of "acli-plus jira" against a real Jira site.
#
# Phases 0-3 only read and dry-run, so they are safe anywhere. Phase 4 onward
# creates real work items in the project you name and deletes them again on the
# way out, so point it at a scratch project, not one anybody depends on.
#
#   ./scripts/e2e-jira.sh --project TEAM --read-only   # safe: read + dry-run
#   ./scripts/e2e-jira.sh --project TEAM               # full run, asks first
#   ./scripts/e2e-jira.sh --project TEAM --keep --verbose
#
# Exit code is 0 only when every check passed.
set -u

BIN="${ACLI_PLUS_BIN:-}"
PROJECT=""
SITE=""
TYPE="Task"
READ_ONLY=0
KEEP=0
VERBOSE=0
ASSUME_YES=0

PASSED=0
FAILED=0
SKIPPED=0
FAILURES=""
CREATED=""
LAST_OUTPUT=""
WORKDIR=""

usage() {
	cat <<'EOF'
e2e-jira.sh — end-to-end check of "acli-plus jira" against a real Jira site.

Usage: ./scripts/e2e-jira.sh --project KEY [options]

  --project KEY    Jira project to write into (required)
  --type NAME      work item type to create (default: Task)
  --site HOST      Atlassian site, when several are registered
  --read-only      stop after the read, dry-run and error phases
  --keep           do not delete the work items this script creates
  --verbose        print each command's output
  --yes            do not ask before the first real write
  -h, --help       show this help

Phases 0-3 are read-only. Phase 4 onward writes to --project and cleans up
after itself unless --keep is given.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--project) PROJECT="$2"; shift 2 ;;
	--type) TYPE="$2"; shift 2 ;;
	--site) SITE="$2"; shift 2 ;;
	--read-only) READ_ONLY=1; shift ;;
	--keep) KEEP=1; shift ;;
	--verbose) VERBOSE=1; shift ;;
	--yes) ASSUME_YES=1; shift ;;
	-h|--help) usage; exit 0 ;;
	*) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
	esac
done

# ---------------------------------------------------------------- reporting --

# Colour only when stdout is a terminal, so redirected output stays clean.
if [ -t 1 ]; then
	C_BOLD=$(printf '\033[1m'); C_GREEN=$(printf '\033[32m')
	C_RED=$(printf '\033[31m'); C_YELLOW=$(printf '\033[33m')
	C_OFF=$(printf '\033[0m')
else
	C_BOLD=""; C_GREEN=""; C_RED=""; C_YELLOW=""; C_OFF=""
fi

phase() { printf '\n%s== %s%s\n' "$C_BOLD" "$1" "$C_OFF"; }
pass() { PASSED=$((PASSED + 1)); printf '  %sPASS%s  %s\n' "$C_GREEN" "$C_OFF" "$1"; }
skip() { SKIPPED=$((SKIPPED + 1)); printf '  %sSKIP%s  %s (%s)\n' "$C_YELLOW" "$C_OFF" "$1" "$2"; }

fail() {
	FAILED=$((FAILED + 1))
	FAILURES="$FAILURES
  - $1"
	printf '  %sFAIL%s  %s\n' "$C_RED" "$C_OFF" "$1"
	printf '%s\n' "$2" | sed 's/^/        /'
}

show() { [ "$VERBOSE" -eq 1 ] && printf '%s\n' "$LAST_OUTPUT" | sed 's/^/        /'; return 0; }

# acli runs the tool under test with --site threaded through when given.
acli() {
	if [ -n "$SITE" ]; then
		"$BIN" --site "$SITE" "$@"
	else
		"$BIN" "$@"
	fi
}

# expect_ok <label> <args...> — must exit 0. Output is left in LAST_OUTPUT for
# the next assertion to read.
expect_ok() {
	label="$1"; shift
	if LAST_OUTPUT=$(acli "$@" 2>&1); then
		pass "$label"; show; return 0
	fi
	fail "$label" "$LAST_OUTPUT"; return 1
}

# expect_out <label> <needle> <args...> — must exit 0 and print needle.
expect_out() {
	label="$1"; needle="$2"; shift 2
	if ! LAST_OUTPUT=$(acli "$@" 2>&1); then
		fail "$label" "$LAST_OUTPUT"; return 1
	fi
	if printf '%s' "$LAST_OUTPUT" | grep -qF -- "$needle"; then
		pass "$label"; show; return 0
	fi
	fail "$label" "expected output containing: $needle
got:
$LAST_OUTPUT"
	return 1
}

# expect_err <label> <needle> <args...> — must fail, with needle in the message.
# This is what pins the wording a user actually sees when they get it wrong.
expect_err() {
	label="$1"; needle="$2"; shift 2
	if LAST_OUTPUT=$(acli "$@" 2>&1); then
		fail "$label" "expected a failure, but it succeeded:
$LAST_OUTPUT"
		return 1
	fi
	if printf '%s' "$LAST_OUTPUT" | grep -qF -- "$needle"; then
		pass "$label"; return 0
	fi
	fail "$label" "expected an error containing: $needle
got:
$LAST_OUTPUT"
	return 1
}

# contains asserts against the output already in LAST_OUTPUT.
contains() {
	if printf '%s' "$LAST_OUTPUT" | grep -qF -- "$2"; then
		pass "$1"
	else
		fail "$1" "not found in the previous output:
$LAST_OUTPUT"
	fi
}

# keyFrom pulls a work item key out of a result line such as "created TEAM-1 …".
keyFrom() {
	printf '%s' "$1" | grep -oE '[A-Z][A-Z0-9_]*-[0-9]+' | head -1
}

track() { CREATED="$CREATED $1"; }

cleanup() {
	[ -n "$WORKDIR" ] && rm -rf "$WORKDIR"
	[ -z "$CREATED" ] && return 0
	if [ "$KEEP" -eq 1 ]; then
		printf '\n--keep: leaving behind%s\n' "$CREATED"
		return 0
	fi
	printf '\nCleaning up:%s\n' "$CREATED"
	for key in $CREATED; do
		acli jira workitem delete "$key" --yes --with-subtasks >/dev/null 2>&1 ||
			printf '  could not delete %s — remove it by hand\n' "$key"
	done
}
trap cleanup EXIT

summary() {
	phase "Summary"
	printf '%d passed, %d failed, %d skipped\n' "$PASSED" "$FAILED" "$SKIPPED"
	if [ "$FAILED" -ne 0 ]; then
		printf 'Failures:%s\n' "$FAILURES"
		exit 1
	fi
	exit 0
}

# ---------------------------------------------------------------- preflight --

if [ -z "$BIN" ]; then
	if [ -x ./bin/acli-plus ]; then
		BIN=./bin/acli-plus
	elif command -v acli-plus >/dev/null 2>&1; then
		BIN=acli-plus
	else
		echo "acli-plus not found: run 'make build' or set ACLI_PLUS_BIN" >&2
		exit 2
	fi
fi

if [ -z "$PROJECT" ]; then
	echo "--project is required (a scratch Jira project this script may write to)" >&2
	usage >&2
	exit 2
fi

phase "Phase 0 — preflight"
printf '  binary:  %s (%s)\n' "$BIN" "$("$BIN" version 2>&1)"
printf '  project: %s\n' "$PROJECT"
printf '  site:    %s\n' "${SITE:-from credentials / acli-plus.yaml}"

if ! expect_ok "Jira answers with the stored credentials" jira project list --limit 1; then
	echo
	echo "Cannot reach Jira. Run 'acli-plus setup' and check that its last line," >&2
	echo "'Reachable on this site', mentions Jira." >&2
	exit 1
fi
expect_out "the target project exists" "$PROJECT" jira project view "$PROJECT" || exit 1

# ------------------------------------------------------------ phase 1: reads --

phase "Phase 1 — read-only commands"

expect_ok "project list" jira project list --limit 5
expect_ok "project list --json" jira project list --limit 1 --json
expect_ok "project list --csv" jira project list --limit 1 --csv
expect_ok "field list" jira field list
expect_out "field list finds a field by name" "labels" jira field list --query label
expect_ok "field list --custom" jira field list --custom
expect_ok "filter list" jira filter list
expect_ok "filter search" jira filter search --limit 5
expect_ok "dashboard search" jira dashboard search --limit 5
expect_ok "workitem link --list-types" jira workitem link --list-types
expect_ok "workitem search" jira workitem search --jql "project = $PROJECT ORDER BY created DESC" --limit 5
expect_ok "workitem search --csv" jira workitem search --jql "project = $PROJECT" --limit 2 --csv
expect_ok "workitem search --fields" jira workitem search --jql "project = $PROJECT" --limit 2 --fields summary,status
expect_ok "workitem search --paginate" jira workitem search --jql "project = $PROJECT" --paginate

expect_ok "board search" jira board search --project "$PROJECT"
BOARD=$(printf '%s' "$LAST_OUTPUT" | awk 'NR==2 {print $1}' | grep -E '^[0-9]+$' || true)
SPRINT=""
if [ -z "$BOARD" ]; then
	skip "board list-sprints" "no board in $PROJECT"
elif LAST_OUTPUT=$(acli jira board list-sprints --board "$BOARD" 2>&1); then
	pass "board list-sprints"
	show
	SPRINT=$(printf '%s' "$LAST_OUTPUT" | awk 'NR==2 {print $1}' | grep -E '^[0-9]+$' || true)
# A Kanban board genuinely has no sprints, so the right result is the explanation
# rather than a table — what would be wrong is a raw API error.
elif printf '%s' "$LAST_OUTPUT" | grep -qF "does not use sprints"; then
	pass "board list-sprints explains that board $BOARD is Kanban"
else
	fail "board list-sprints" "$LAST_OUTPUT"
fi

if [ -n "$SPRINT" ]; then
	expect_ok "sprint list-workitems" jira sprint list-workitems --sprint "$SPRINT"
else
	skip "sprint list-workitems" "no sprint to look into"
fi

# --------------------------------------------------------- phase 2: dry runs --

phase "Phase 2 — dry run (nothing may be written)"

BEFORE=$(acli jira workitem search --jql "project = $PROJECT" --paginate 2>/dev/null | wc -l | tr -d ' ')

expect_out "create --dry-run" "[dry-run]" \
	jira workitem create -p "$PROJECT" -t "$TYPE" -s "e2e dry run" --dry-run

# edit and delete need something to act on, and an empty project has nothing.
if [ "$BEFORE" -gt 0 ]; then
	expect_out "edit --dry-run" "[dry-run]" \
		jira workitem edit --jql "project = $PROJECT" -s "e2e dry run" --dry-run
	expect_out "delete --dry-run does not prompt" "[dry-run]" \
		jira workitem delete --jql "project = $PROJECT" --dry-run
else
	skip "edit --dry-run" "$PROJECT has no work items yet"
	skip "delete --dry-run does not prompt" "$PROJECT has no work items yet"
	expect_err "a JQL query matching nothing says so" "matched no work items" \
		jira workitem edit --jql "project = $PROJECT" -s "x" --dry-run
fi

AFTER=$(acli jira workitem search --jql "project = $PROJECT" --paginate 2>/dev/null | wc -l | tr -d ' ')
if [ "$BEFORE" = "$AFTER" ]; then
	pass "the project is unchanged after the dry runs ($BEFORE rows before and after)"
else
	fail "the project is unchanged after the dry runs" "row count went from $BEFORE to $AFTER"
fi

# ------------------------------------------------------------ phase 3: errors --

phase "Phase 3 — error messages"

expect_err "a malformed key is caught before any API call" "not a work item key" \
	jira workitem view NOTAKEY
expect_err "an unknown field name is reported, not guessed" "field not found" \
	jira workitem create -p "$PROJECT" -t "$TYPE" -s x --field "Definitely Not A Field=1" --dry-run
expect_err "create without --type" "--type is required" \
	jira workitem create -p "$PROJECT" -s "missing type"
expect_err "search without JQL" "JQL query is required" \
	jira workitem search
expect_err "edit with nothing to change" "nothing to change" \
	jira workitem edit --key "$PROJECT-1"
expect_err "no target selected" "no work items selected" \
	jira workitem edit -s "x"
expect_err "a nonexistent work item" "not found" \
	jira workitem view "$PROJECT-999999"
expect_err "an unknown link type lists the real ones" "issue link type not found" \
	jira workitem link --type NotALinkType --inward "$PROJECT-1" --outward "$PROJECT-2"

[ "$READ_ONLY" -eq 1 ] && summary

# ------------------------------------------------------------ phase 4: create --

phase "Phase 4 — real writes into $PROJECT"

if [ "$ASSUME_YES" -eq 0 ]; then
	printf 'This creates and then deletes work items in %s. Continue? [y/N]: ' "$PROJECT"
	read -r answer
	case "$answer" in
	y|Y|yes|YES) ;;
	*) echo "stopped"; exit 0 ;;
	esac
fi

WORKDIR=$(mktemp -d)

expect_ok "create" jira workitem create -p "$PROJECT" -t "$TYPE" \
	-s "acli-plus e2e main" -d "Created by the e2e script." -a @me --label acli-plus-e2e || summary
MAIN=$(keyFrom "$LAST_OUTPUT")
if [ -z "$MAIN" ]; then
	fail "parse the created key" "could not find a key in: $LAST_OUTPUT"
	summary
fi
track "$MAIN"
printf '  work item: %s\n' "$MAIN"
contains "the result line links to the work item" "/browse/$MAIN"

expect_out "view shows the summary" "acli-plus e2e main" jira workitem view "$MAIN"
expect_out "view shows the assignee" "Assignee:" jira workitem view "$MAIN"
expect_out "view --json carries the raw payload" '"fields"' jira workitem view "$MAIN" --json
expect_out "view --fields narrows the request" "Summary:" jira workitem view "$MAIN" --fields summary
expect_out "the new work item is findable by JQL" "$MAIN" jira workitem search --jql "key = $MAIN"

HOST=$(acli jira workitem view "$MAIN" 2>/dev/null | sed -n 's|^URL: *https://\([^/]*\)/.*|\1|p' | head -1)
if [ -n "$HOST" ]; then
	expect_ok "a browse URL works in place of a key" jira workitem view "https://$HOST/browse/$MAIN"
else
	skip "a browse URL works in place of a key" "could not read the host from the view output"
fi

# ------------------------------------------------------- phase 5: Markdown/ADF --

phase "Phase 5 — Markdown to ADF and back"

cat >"$WORKDIR/story.md" <<'MD'
---
title: acli-plus e2e from file
---

Intro with **bold**, *italic* and `inline code`.

## A heading

- first bullet
- second bullet

1. numbered one
2. numbered two

- [x] a done task
- [ ] a pending task

> a quoted line

```go
fmt.Println("hello")
```

| col a | col b |
|---|---|
| 1 | 2 |

A [link](https://example.com).

---
MD

if expect_ok "create --from-file" jira workitem create -p "$PROJECT" -t "$TYPE" --from-file "$WORKDIR/story.md"; then
	FROMFILE=$(keyFrom "$LAST_OUTPUT")
	track "$FROMFILE"
	expect_out "the frontmatter title became the summary" "acli-plus e2e from file" \
		jira workitem view "$FROMFILE"

	# Round trip: Markdown -> ADF -> Jira -> ADF -> plain text. Each construct
	# below has to survive both renderers.
	expect_ok "read the description back" jira workitem view "$FROMFILE"
	contains "  heading survived" "## A heading"
	contains "  bullet list survived" "- first bullet"
	contains "  ordered list survived" "1. numbered one"
	contains "  done task survived" "[x] a done task"
	contains "  pending task survived" "[ ] a pending task"
	contains "  blockquote survived" "> a quoted line"
	contains "  code block survived" 'fmt.Println("hello")'
	contains "  table survived" "col a | col b"
else
	skip "Markdown to ADF checks" "the --from-file create failed"
fi

expect_ok "create -d with Markdown" jira workitem create -p "$PROJECT" -t "$TYPE" \
	-s "acli-plus e2e markdown" -d "Line with **bold** and a [link](https://example.com)."
MD_KEY=$(keyFrom "$LAST_OUTPUT")
[ -n "$MD_KEY" ] && track "$MD_KEY"

# ------------------------------------------------------- phase 6: edit/workflow --

phase "Phase 6 — editing, fields and workflow"

expect_ok "edit the summary" jira workitem edit "$MAIN" -s "acli-plus e2e edited"
expect_out "the edit took effect" "acli-plus e2e edited" jira workitem view "$MAIN"
expect_ok "--field resolves a field by display name" jira workitem edit "$MAIN" --field "Labels=acli-plus-e2e,second"
expect_out "the field value was applied" "second" jira workitem view "$MAIN" --fields labels
expect_ok "edit --no-notify" jira workitem edit "$MAIN" --priority Medium --no-notify || true

expect_ok "transition --list" jira workitem transition "$MAIN" --list
TARGET=$(printf '%s' "$LAST_OUTPUT" | awk -F'  +' 'NR==2 {print $3}')
if [ -n "$TARGET" ]; then
	expect_ok "transition to \"$TARGET\"" jira workitem transition "$MAIN" --status "$TARGET"
	expect_out "the status changed" "$TARGET" jira workitem view "$MAIN"
else
	skip "transition" "no transition available from the current status"
fi
expect_err "an unreachable status lists the reachable ones" "no transition to that status" \
	jira workitem transition "$MAIN" --status "Definitely Not A Status"

expect_ok "assign @me" jira workitem assign "$MAIN" --assignee @me
expect_ok "unassign" jira workitem assign "$MAIN" --assignee ""
expect_ok "assign back" jira workitem assign "$MAIN" --assignee @me

# ------------------------------------------ phase 7: comments, links, watchers --

phase "Phase 7 — comments, links, watchers, attachments"

expect_ok "comment-create" jira workitem comment-create "$MAIN" -b "First comment with **markdown**."
expect_out "comment-list shows it" "First comment" jira workitem comment-list "$MAIN"
COMMENT=$(printf '%s' "$LAST_OUTPUT" | awk 'NR==1 {print $1}' | grep -E '^[0-9]+$' || true)
if [ -n "$COMMENT" ]; then
	expect_ok "comment-update" jira workitem comment-update "$MAIN" --id "$COMMENT" -b "Edited comment."
	expect_out "the comment text changed" "Edited comment." jira workitem comment-list "$MAIN"
	# comment-visibility reads the comment and writes it back, so the body must
	# survive a visibility-only change.
	expect_ok "comment-visibility --public" jira workitem comment-visibility "$MAIN" --id "$COMMENT" --public
	expect_out "the body survived the visibility change" "Edited comment." jira workitem comment-list "$MAIN"
	expect_ok "comment-delete" jira workitem comment-delete "$MAIN" --id "$COMMENT" --yes
else
	skip "comment update/visibility/delete" "could not read the comment id"
fi

expect_ok "create a link target" jira workitem create -p "$PROJECT" -t "$TYPE" -s "acli-plus e2e link target"
OTHER=$(keyFrom "$LAST_OUTPUT")
if [ -n "$OTHER" ]; then
	track "$OTHER"
	expect_ok "link two work items" jira workitem link --type Relates --inward "$MAIN" --outward "$OTHER"
else
	skip "link two work items" "the link target was not created"
fi

expect_ok "attachment-list (empty is a valid result)" jira workitem attachment-list "$MAIN"
expect_ok "watcher-list" jira workitem watcher-list "$MAIN"
if printf '%s' "$LAST_OUTPUT" | grep -qv '^no watchers'; then
	expect_ok "watcher-remove @me" jira workitem watcher-remove "$MAIN" --watcher @me || true
else
	skip "watcher-remove" "nobody is watching $MAIN"
fi

# --------------------------------------------------- phase 8: clone/bulk/archive --

phase "Phase 8 — clone, bulk create, archive"

expect_ok "clone" jira workitem clone "$MAIN" --prefix "COPY -"
CLONE=$(printf '%s' "$LAST_OUTPUT" | sed -n 's/.*cloned into //p' | grep -oE '[A-Z][A-Z0-9_]*-[0-9]+' | head -1)
if [ -n "$CLONE" ]; then
	track "$CLONE"
	expect_out "the clone carries the prefixed summary" "COPY -" jira workitem view "$CLONE"
else
	skip "clone verification" "could not parse the clone key"
fi

expect_out "create-bulk --generate-json prints a template" '"project"' \
	jira workitem create-bulk --generate-json

cat >"$WORKDIR/bulk.json" <<EOF
[
  {"project": "$PROJECT", "type": "$TYPE", "summary": "acli-plus e2e bulk 1", "labels": ["acli-plus-e2e"]},
  {"project": "$PROJECT", "type": "$TYPE", "summary": "acli-plus e2e bulk 2", "description": "With **markdown**."}
]
EOF

expect_out "create-bulk --dry-run" "[dry-run]" jira workitem create-bulk --from-json "$WORKDIR/bulk.json" --dry-run
if expect_ok "create-bulk" jira workitem create-bulk --from-json "$WORKDIR/bulk.json"; then
	for key in $(printf '%s' "$LAST_OUTPUT" | grep -oE "$PROJECT-[0-9]+"); do
		track "$key"
	done
fi

# Archiving needs Jira Premium or Enterprise. Either outcome is correct; what
# must not happen is a bare 404 that reads like "your work item is missing".
if LAST_OUTPUT=$(acli jira workitem archive "$MAIN" 2>&1); then
	pass "archive (this site has Premium or Enterprise)"
	expect_ok "unarchive" jira workitem unarchive "$MAIN"
elif printf '%s' "$LAST_OUTPUT" | grep -qF "not available on your Jira plan"; then
	pass "archive reports the plan limit instead of a confusing 404"
else
	fail "archive" "$LAST_OUTPUT"
fi

# The delete confirmation is exercised by the cleanup trap below, which passes
# --yes; here we check that declining leaves the work item alone.
if [ -n "$OTHER" ]; then
	if printf 'n\n' | acli jira workitem delete "$OTHER" >/dev/null 2>&1 &&
		acli jira workitem view "$OTHER" >/dev/null 2>&1; then
		pass "declining the delete prompt leaves the work item alone"
	else
		fail "declining the delete prompt leaves the work item alone" \
			"$OTHER is gone or unreadable after answering 'n'"
	fi
fi

summary
