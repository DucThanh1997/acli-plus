#!/usr/bin/env bash
# Publish a GitLab release from locally-built ./dist — no CI/runner needed.
#
# Prereqs:
#   - ./dist built:  ./scripts/build.sh
#   - a GitLab token with the `api` scope (Personal or Project Access Token)
#
# Usage:
#   GITLAB_TOKEN=glpat-xxxx ./scripts/publish-release.sh v0.1.0
#
# It uploads each binary to the project's generic package registry and creates
# a release whose asset links use `filepath`, so install.sh can fetch them at
#   <project>/-/releases/<tag>/downloads/acli-plus_<os>_<arch>
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${1:?usage: GITLAB_TOKEN=xxx $0 <tag, e.g. v0.1.0>}"
VER="${TAG#v}"
: "${GITLAB_TOKEN:?set GITLAB_TOKEN to a token with the api scope}"
HOST="${GITLAB_HOST:-https://gitlab.techvify.dev}"
PROJECT="${GITLAB_PROJECT:-d14/ai-kit-group/acli-plus}"

PROJ_ENC=$(printf '%s' "$PROJECT" | sed 's:/:%2F:g')
API="$HOST/api/v4/projects/$PROJ_ENC"

[ -d dist ] && ls dist/acli-plus_* >/dev/null 2>&1 || {
  echo "dist/ is empty — run ./scripts/build.sh first" >&2
  exit 1
}

echo "Uploading binaries to the generic package registry..."
links=""
for file in dist/acli-plus_*; do
  name=$(basename "$file")
  url="$API/packages/generic/acli-plus/$VER/$name"
  curl --fail --silent --show-error \
    --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
    --upload-file "$file" "$url" >/dev/null
  echo "  uploaded $name"
  links="${links}{\"name\":\"$name\",\"url\":\"$url\",\"filepath\":\"/$name\",\"link_type\":\"package\"},"
done
links="[${links%,}]"

echo "Creating release $TAG..."
curl --fail --silent --show-error \
  --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
  --header "Content-Type: application/json" \
  --data "{\"tag_name\":\"$TAG\",\"name\":\"acli-plus $TAG\",\"description\":\"acli-plus $TAG\",\"assets\":{\"links\":$links}}" \
  "$API/releases" >/dev/null

echo "Done. Release: $HOST/$PROJECT/-/releases/$TAG"
