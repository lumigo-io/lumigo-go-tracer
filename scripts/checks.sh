#!/bin/bash
set -eo pipefail

export GOPATH=$(go env GOPATH)
make checks
