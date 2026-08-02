#!/usr/bin/env bash
# Publish a GitLab release from locally-built binaries — no CI/runner needed.
#
# Versioning: with no argument, reads the latest release and bumps by 0.1
#   (0.1 -> 0.2 -> ... -> 0.9 -> 1.0 -> 1.1 ...). Pass a version to override:
#     ./scripts/publish-release.sh 1.2
#
# Token: put it in a gitignored .publish.env at the repo root (recommended):
#     GITLAB_TOKEN=glpat-xxxx          # scope: api (role Maintainer)
#   or export GITLAB_TOKEN before running. NEVER commit the token.
set -euo pipefail
cd "$(dirname "$0")/.."

# --- token (from .publish.env or the environment) ---
[ -f .publish.env ] && . ./.publish.env
: "${GITLAB_TOKEN:?set GITLAB_TOKEN in .publish.env (or the environment) — a token with the api scope}"

HOST="${GITLAB_HOST:-https://gitlab.techvify.dev}"
PROJECT="${GITLAB_PROJECT:-d14/ai-kit-group/acli-plus}"
REF="${RELEASE_REF:-main}"
PROJ_ENC=$(printf '%s' "$PROJECT" | sed 's:/:%2F:g')
API="$HOST/api/v4/projects/$PROJ_ENC"

auth=(--header "PRIVATE-TOKEN: $GITLAB_TOKEN")

# --- resolve the version to publish ---
if [ "${1:-}" != "" ]; then
  VER="${1#v}"
else
  latest=$(curl -fsSL "${auth[@]}" "$API/releases?per_page=1" \
    | grep -o '"tag_name":"[^"]*"' | head -1 | sed 's/.*:"v\{0,1\}//; s/"$//' || true)
  if [ -z "$latest" ]; then
    VER="0.1"
  else
    major=$(printf '%s' "$latest" | cut -d. -f1)
    minor=$(printf '%s' "$latest" | cut -d. -f2)
    tenths=$(( major * 10 + minor + 1 )) # +0.1, carrying at .10 -> next major
    VER="$(( tenths / 10 )).$(( tenths % 10 ))"
  fi
fi
TAG="v${VER}"
echo "Publishing ${TAG} (ref: ${REF})"

# --- build all platforms with the version baked in ---
echo "Building..."
VERSION="$VER" bash scripts/build.sh >/dev/null
ls dist/acli-plus_* >/dev/null 2>&1 || { echo "build produced no binaries" >&2; exit 1; }

# --- upload binaries to the generic package registry ---
echo "Uploading binaries..."
links=""
for file in dist/acli-plus_*; do
  name=$(basename "$file")
  url="$API/packages/generic/acli-plus/$VER/$name"
  curl -fsS "${auth[@]}" --upload-file "$file" "$url" >/dev/null
  echo "  $name"
  links="${links}{\"name\":\"$name\",\"url\":\"$url\",\"filepath\":\"/$name\",\"link_type\":\"package\"},"
done
links="[${links%,}]"

# --- create the release (creates the tag at $REF) ---
echo "Creating release ${TAG}..."
curl -fsS "${auth[@]}" --header "Content-Type: application/json" \
  --data "{\"tag_name\":\"$TAG\",\"ref\":\"$REF\",\"name\":\"acli-plus $TAG\",\"description\":\"acli-plus $TAG\",\"assets\":{\"links\":$links}}" \
  "$API/releases" >/dev/null

echo "Done: ${HOST}/${PROJECT}/-/releases/${TAG}"
echo "Latest permalink: ${HOST}/${PROJECT}/-/releases/permalink/latest"
