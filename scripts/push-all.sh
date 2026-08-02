#!/usr/bin/env bash
# Push the current branch (and tags) to BOTH remotes in one go:
#   github = https://github.com/DucThanh1997/acli-plus        (public, distribution)
#   gitlab = https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus (internal mirror)
#
# Usage:  ./scripts/push-all.sh
set -euo pipefail
cd "$(dirname "$0")/.."

branch="$(git rev-parse --abbrev-ref HEAD)"

push_to() {
  local remote="$1"
  if ! git remote get-url "$remote" >/dev/null 2>&1; then
    echo "skip: remote '$remote' not configured (add it: git remote add $remote <url>)" >&2
    return 0
  fi
  echo "→ ${remote}: pushing branch '${branch}' + tags"
  git push "$remote" "$branch"
  git push "$remote" --tags
}

push_to github
push_to gitlab
echo "Done — pushed '${branch}' + tags to github and gitlab."
