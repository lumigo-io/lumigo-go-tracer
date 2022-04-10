#!/usr/bin/env bash
set -Eeo pipefail

pushd "$(dirname "$0")" &> /dev/null
# Go back one spot because we are on scripts dir. The other scripts assume you are in the root folder
cd ..
../utils/common_bash/defaults/deploy.sh "NAME OF YOUR SERVICE" $*
popd &> /dev/null
