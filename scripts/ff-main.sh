#!/usr/bin/env bash
# Fast-forward main to develop and push.
# Usage: scripts/ff-main.sh            (fast-forward + push)
#        scripts/ff-main.sh --no-push  (fast-forward locally only)
#single line terminal 

#git fetch origin && git checkout main && git merge --ff-only origin/main && git merge --ff-only origin/develop && git push origin main


set -euo pipefail

SRC=develop
DST=main
PUSH=1
[[ "${1:-}" == "--no-push" ]] && PUSH=0

if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "error: working tree has uncommitted changes; commit or stash first" >&2
  exit 1
fi

orig=$(git rev-parse --abbrev-ref HEAD)
trap 'git checkout -q "$orig"' EXIT

git fetch origin --prune

# Local branches must not be ahead of / diverged from origin.
for b in "$SRC" "$DST"; do
  if git show-ref --verify --quiet "refs/remotes/origin/$b"; then
    git checkout -q "$b"
    git merge --ff-only -q "origin/$b"
  fi
done

if git merge-base --is-ancestor "$SRC" "$DST"; then
  echo "$DST is already up to date with $SRC"
  exit 0
fi

git checkout -q "$DST"
echo "Commits going to $DST:"
git log --oneline "$DST..$SRC"
git merge --ff-only "$SRC"

if [[ $PUSH -eq 1 ]]; then
  git push origin "$DST"
fi
echo "done: $DST -> $(git rev-parse --short HEAD)"
