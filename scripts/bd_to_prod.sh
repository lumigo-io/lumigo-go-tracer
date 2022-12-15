#!/bin/bash
set -eo pipefail

source ../utils/common_bash/functions.sh

go get -u github.com/stevenmatthewt/semantics

TAG="$(semantics --output-tag)"
if [[ -n "$TAG" ]] ; then
  git log --oneline > changelog.md
  gh release create "$TAG" -F changelog.md
else
  echo "The commit message is not major/minor/patch version"
fi

send_metric_to_logz_io \
  type="\"Release\"" \
  git_hash="\"$(git show -s --format=%h)\""
