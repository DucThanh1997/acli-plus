#!/usr/bin/env sh
# e2e-confluence.sh — end-to-end check of "acli-plus confluence page" against a
# real Confluence site.
#
# Phases 0-3 only parse, read and dry-run, so they are safe anywhere. Phase 4
# onward creates real pages under the parent you name and trashes them on the
# way out, so point it at a scratch space, not one anybody depends on.
#
#   ./scripts/e2e-confluence.sh --parent <spaceUrl> --read-only   # safe
#   ./scripts/e2e-confluence.sh --parent <spaceUrl>               # full, asks first
#   ./scripts/e2e-confluence.sh --parent <spaceUrl> --keep --verbose
#
# Exit code is 0 only when every check passed.
set -u

BIN="${ACLI_PLUS_BIN:-}"
PARENT=""
SITE=""
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
HOST=""
SPACE=""

usage() {
	cat <<'EOF'
e2e-confluence.sh — end-to-end check of "acli-plus confluence page".

Usage: ./scripts/e2e-confluence.sh --parent <url> [options]

  --parent URL     space or page URL to publish under (required)
  --site HOST      Atlassian site, when several are registered
  --read-only      stop after the parse, read and dry-run phases
  --keep           do not trash the pages this script creates
  --verbose        print each command's output
  --yes            do not ask before the first real write
  -h, --help       show this help

--parent is where every test page is created, e.g.
  https://acme.atlassian.net/wiki/spaces/DEV
  https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Handbook

Phases 0-3 write nothing. Phase 4 onward creates pages under --parent and
trashes them again unless --keep is given.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--parent) PARENT="$2"; shift 2 ;;
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

# acli runs the tool under test with --site threaded through when known. stdin
# is closed so a command that asks for confirmation gets EOF and aborts instead
# of hanging the whole run — an unattended script must never wait on a prompt.
acli() {
	if [ -n "$SITE" ]; then
		"$BIN" --site "$SITE" "$@" < /dev/null
	else
		"$BIN" "$@" < /dev/null
	fi
}

expect_ok() {
	label="$1"; shift
	if LAST_OUTPUT=$(acli "$@" 2>&1); then
		pass "$label"; show; return 0
	fi
	fail "$label" "$LAST_OUTPUT"; return 1
}

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

# expect_err pins the wording a user actually sees when they get it wrong.
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

contains() {
	if printf '%s' "$LAST_OUTPUT" | grep -qF -- "$2"; then
		pass "$1"
	else
		fail "$1" "not found in the previous output:
$LAST_OUTPUT"
	fi
}

lacks() {
	if printf '%s' "$LAST_OUTPUT" | grep -qF -- "$2"; then
		fail "$1" "should not appear, but the previous output has it:
$LAST_OUTPUT"
	else
		pass "$1"
	fi
}

# idFrom pulls the page id out of a result line. The link acli-plus prints is a
# viewpage.action URL, which its own ParseRef does not accept back, so every
# later command addresses the page by this bare id instead.
idFrom() {
	printf '%s' "$1" | sed -n 's/.*pageId=\([0-9][0-9]*\).*/\1/p' | head -1
}

track() { CREATED="$CREATED $1"; }

cleanup() {
	[ -n "$WORKDIR" ] && rm -rf "$WORKDIR"
	[ -z "$CREATED" ] && return 0
	if [ "$KEEP" -eq 1 ]; then
		printf '\n--keep: leaving behind page ids%s\n' "$CREATED"
		return 0
	fi
	printf '\nCleaning up page ids:%s\n' "$CREATED"
	for id in $CREATED; do
		acli confluence page delete "$id" --yes >/dev/null 2>&1 ||
			printf '  could not trash %s — remove it by hand\n' "$id"
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

if [ -z "$PARENT" ]; then
	echo "--parent is required (a scratch space or page this script may write under)" >&2
	usage >&2
	exit 2
fi

# Pull host and space out of --parent. The host becomes --site when the caller
# did not name one, so cleanup by bare page id resolves even on a machine with
# several sites registered.
HOST=$(printf '%s' "$PARENT" | sed -n 's|^https\{0,1\}://\([^/]*\).*|\1|p')
SPACE=$(printf '%s' "$PARENT" | sed -n 's|.*/spaces/\([^/?#]*\).*|\1|p')
[ -z "$SITE" ] && [ -n "$HOST" ] && SITE="$HOST"

WORKDIR=$(mktemp -d)
STAMP=$(date +%s 2>/dev/null || echo 0)
PREFIX="acli-plus-e2e-$STAMP"

phase "Phase 0 — preflight"
printf '  binary: %s (%s)\n' "$BIN" "$("$BIN" version 2>&1)"
printf '  parent: %s\n' "$PARENT"
printf '  site:   %s\n' "${SITE:-from credentials / acli-plus.yaml}"
printf '  space:  %s\n' "${SPACE:-(none — parent is a bare page)}"
printf '  titles: %s-*\n' "$PREFIX"

# ------------------------------------------------- phase 1: reference parsing --

# Parsing runs before the connectivity check on purpose: these cases never reach
# the network, so an unreachable site is no reason to leave them unrun.
phase "Phase 1 — URL parsing (no API call)"

expect_err "a short /wiki/x/ link is rejected with guidance" "paste the full page URL" \
	confluence page delete "https://$HOST/wiki/x/AbCd" --yes
expect_err "a URL with neither space nor page is rejected" "not a recognizable Confluence URL" \
	confluence page delete "https://$HOST/wiki/" --yes
expect_err "an empty reference is rejected" "not a recognizable Confluence URL" \
	confluence page delete "" --yes

# The link acli-plus prints for a written page is a viewpage.action URL, and its
# own parser reads "viewpage.action" as the page id. Pinning it here so that a
# fix to either side shows up as a failing expectation rather than silence.
expect_err "the printed viewpage.action link is NOT accepted back as input" "confluence api 400" \
	confluence page delete "https://$HOST/wiki/pages/viewpage.action?pageId=1" --yes

# --------------------------------------------------------- phase 2: dry runs --

phase "Phase 2 — dry run (nothing may be written)"

printf 'preflight\n' > "$WORKDIR/$PREFIX-preflight.md"
if ! expect_ok "Confluence answers and the parent resolves" \
	confluence page create "$WORKDIR/$PREFIX-preflight.md" "$PARENT" --dry-run; then
	echo
	echo "Cannot reach the parent. Check that 'acli-plus setup' ran for this host" >&2
	echo "and that --parent is a full space or page URL (not a /wiki/x/ short link)." >&2
	summary
fi

printf 'dry run body\n' > "$WORKDIR/$PREFIX-dry.md"
expect_out "create --dry-run says what it would do" "[dry-run]" \
	confluence page create "$WORKDIR/$PREFIX-dry.md" "$PARENT" --dry-run
expect_out "  and names the page it would write" "$PREFIX-dry" \
	confluence page create "$WORKDIR/$PREFIX-dry.md" "$PARENT" --dry-run

# ---------------------------------------------------------- phase 3: errors --

phase "Phase 3 — error messages"

expect_err "delete of a nonexistent page id" "not found" \
	confluence page delete 999999999 --yes
expect_err "create with a missing file" "no such file or directory" \
	confluence page create "$WORKDIR/does-not-exist.md" "$PARENT" --dry-run

if [ "$READ_ONLY" -eq 1 ]; then
	phase "Stopping after the read-only phases (--read-only)"
	summary
fi

# ------------------------------------------------------- phase 4: real writes --

if [ "$ASSUME_YES" -eq 0 ]; then
	printf '\nThe next phases create real pages under:\n  %s\n' "$PARENT"
	printf 'They are trashed again on exit unless --keep. Continue? [y/N]: '
	read -r answer
	case "$answer" in y|Y|yes|YES) ;; *) echo "aborted"; summary ;; esac
fi

phase "Phase 4 — create, and create-as-upsert"

cat > "$WORKDIR/$PREFIX-main.md" <<MD
---
title: $PREFIX main
---

First body with **bold**, *italic* and \`inline code\`.

## A heading

- first bullet
- second bullet

1. numbered one
2. numbered two

> a quoted line

\`\`\`go
fmt.Println("hello")
\`\`\`

A [link](https://example.com).
MD

MAIN=""
if expect_out "create publishes the file" "created" \
	confluence page create "$WORKDIR/$PREFIX-main.md" "$PARENT"; then
	contains "  the frontmatter title became the page title" "$PREFIX main"
	MAIN=$(idFrom "$LAST_OUTPUT")
	[ -n "$MAIN" ] && track "$MAIN"
fi

# Reading the page back is what turns "the command exited 0" into "the right
# markup reached Confluence". Storage format is XHTML, so each Markdown
# construct has a tag to look for.
if [ -n "$MAIN" ] && expect_ok "view reads the page back" confluence page view "$MAIN"; then
	contains "  bold became <strong>" "<strong>bold</strong>"
	contains "  the heading became <h2>" "<h2>A heading</h2>"
	contains "  the bullet list became <ul>" "<ul>"
	contains "  the numbered list became <ol>" "<ol>"
	contains "  the link carries its href" 'href="https://example.com"'
	lacks "  no literal Markdown syntax survived" "**bold**"
fi

if [ -n "$MAIN" ]; then
	# The upsert rule: same title under the same parent must land on the same
	# page, not a second one. Comparing ids is the only way to see that.
	if expect_out "create again with the same title updates instead of duplicating" "updated" \
		confluence page create "$WORKDIR/$PREFIX-main.md" "$PARENT" --yes; then
		AGAIN=$(idFrom "$LAST_OUTPUT")
		if [ "$AGAIN" = "$MAIN" ]; then
			pass "  it is the same page id ($MAIN), so no duplicate was made"
		else
			fail "  it is the same page id" "first create gave $MAIN, second gave $AGAIN"
		fi
	fi
else
	skip "create-as-upsert checks" "the first create failed"
fi

# ------------------------------------------------------- phase 5: page titles --

phase "Phase 5 — title resolution"

printf 'no frontmatter here\n' > "$WORKDIR/$PREFIX-byfilename.md"
if expect_out "a file with no frontmatter is titled after the file" "$PREFIX-byfilename" \
	confluence page create "$WORKDIR/$PREFIX-byfilename.md" "$PARENT"; then
	ID=$(idFrom "$LAST_OUTPUT")
	[ -n "$ID" ] && track "$ID"
fi

printf '# A leading heading\n\nbody\n' > "$WORKDIR/$PREFIX-h1.md"
H1ID=""
if expect_out "a leading H1 does not become the title" "$PREFIX-h1" \
	confluence page create "$WORKDIR/$PREFIX-h1.md" "$PARENT"; then
	lacks "  the H1 text was not used as the title" "A leading heading"
	H1ID=$(idFrom "$LAST_OUTPUT")
	[ -n "$H1ID" ] && track "$H1ID"
fi

# The H1 must be absent from the title and present in the body. Both halves
# matter: dropping it would be just as wrong as promoting it.
if [ -n "$H1ID" ] && expect_ok "read the H1 page back" confluence page view "$H1ID"; then
	contains "  the H1 is still in the body" "<h1>A leading heading</h1>"
	contains "  and the title is the file name" "$PREFIX-h1"
fi

# ------------------------------------------------------- phase 6: update/rename --

phase "Phase 6 — update in place and rename"

if [ -n "$MAIN" ]; then
	printf 'second body, replacing the first\n' > "$WORKDIR/$PREFIX-main.md"
	if expect_out "update overwrites the page body" "updated" \
		confluence page update "$WORKDIR/$PREFIX-main.md" "$MAIN" --yes; then
		SAME=$(idFrom "$LAST_OUTPUT")
		if [ "$SAME" = "$MAIN" ]; then
			pass "  the page id did not change ($MAIN)"
		else
			fail "  the page id did not change" "was $MAIN, now $SAME"
		fi
	fi

	# Overwrite means replace, not append — the old markup has to be gone.
	if expect_ok "read the updated page back" confluence page view "$MAIN"; then
		contains "  the new body is there" "second body, replacing the first"
		lacks "  the first body is gone" "<strong>bold</strong>"
	fi

	cat > "$WORKDIR/$PREFIX-renamed.md" <<MD
---
title: $PREFIX renamed
---

body after the rename
MD
	if expect_out "update renames when the file's title differs" "$PREFIX renamed" \
		confluence page update "$WORKDIR/$PREFIX-renamed.md" "$MAIN" --yes; then
		SAME=$(idFrom "$LAST_OUTPUT")
		if [ "$SAME" = "$MAIN" ]; then
			pass "  the rename kept the same page id ($MAIN)"
		else
			fail "  the rename kept the same page id" "was $MAIN, now $SAME"
		fi
	fi
else
	skip "update checks" "there is no page to update"
fi

# Confluence's create API accepts no version message, so a page acli-plus just
# published carries version 1 with an empty message — indistinguishable from a
# page written in the UI. Every update in this phase therefore passes --yes.
# Pinning that here: it is a real rough edge, not a passing test in disguise.
printf 'probing the prompt\n' > "$WORKDIR/$PREFIX-prompt.md"
if expect_out "create a page to probe the prompt" "created" \
	confluence page create "$WORKDIR/$PREFIX-prompt.md" "$PARENT"; then
	PROMPTED=$(idFrom "$LAST_OUTPUT")
	[ -n "$PROMPTED" ] && track "$PROMPTED"
	expect_out "  updating it without --yes still prompts" "modified outside acli-plus" \
		confluence page update "$WORKDIR/$PREFIX-prompt.md" "$PROMPTED"
	contains "  and with no answer on stdin it aborts safely" "aborted; no changes made"
fi

skip "an externally edited page prompts before overwrite" "needs an edit made in the Confluence UI"

# --------------------------------------------------------- phase 7: deletion --

phase "Phase 7 — delete"

printf 'to be trashed\n' > "$WORKDIR/$PREFIX-doomed.md"
DOOMED=""
if expect_out "create a page to delete" "created" \
	confluence page create "$WORKDIR/$PREFIX-doomed.md" "$PARENT"; then
	DOOMED=$(idFrom "$LAST_OUTPUT")
fi

if [ -n "$DOOMED" ]; then
	expect_out "delete --dry-run reports without trashing" "[dry-run]" \
		confluence page delete "$DOOMED" --dry-run --yes
	# view proves the page survived the dry run without writing to it, which an
	# update-as-probe could not claim.
	expect_ok "  the page is still there afterwards" confluence page view "$DOOMED"

	expect_out "delete --yes trashes the page" "deleted" \
		confluence page delete "$DOOMED" --yes
	# Trash is reversible, so the id keeps resolving and view still answers —
	# only a second delete reports it gone. Asserting on view here would be
	# asserting Confluence purges, which it does not.
	expect_err "  a second delete reports it already gone" "not found" \
		confluence page delete "$DOOMED" --yes
	expect_ok "  but a trashed page is still readable by id" \
		confluence page view "$DOOMED"

	# A trashed page id no longer resolves, so update must fall back to the
	# space rather than fail — and must say so.
	if [ -n "$SPACE" ]; then
		if expect_ok "update falls back to the space when the page id is gone" \
			confluence page update "$WORKDIR/$PREFIX-doomed.md" \
			"https://$HOST/wiki/spaces/$SPACE/pages/$DOOMED/gone"; then
			ID=$(idFrom "$LAST_OUTPUT")
			[ -n "$ID" ] && track "$ID"
		fi
	else
		skip "update falls back to the space" "--parent carries no space key"
	fi
else
	skip "delete checks" "could not create a page to delete"
fi

summary
