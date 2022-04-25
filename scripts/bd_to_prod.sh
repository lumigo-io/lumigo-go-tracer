#!/bin/bash
set -eo pipefail

go get -u github.com/stevenmatthewt/semantics

TAG=$(semantics --output-tag)
if [[ ! -z "$TAG" ]] ; then
  git log --oneline > changelog.md
  gh release create $TAG -F changelog.md
else
  echo "The commit message is not major/minor/patch version"
fi
echo \{\"type\":\"Release\",\"repo\":\"${CIRCLE_PROJECT_REPONAME}\",\"buildUrl\":\"${CIRCLE_BUILD_URL}\"\} | curl -X POST "https://listener.logz.io:8071?token=${LOGZ_TOKEN}" -v --data-binary @-
