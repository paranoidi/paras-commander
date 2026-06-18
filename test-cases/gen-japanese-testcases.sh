#!/usr/bin/env bash
# Temporary helper: creates japanese/ with encoding test cases for rename-dialog F4.
# Safe to delete japanese/ and this script when done.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec go run "${ROOT}/gen-japanese-testcases.go" "$@"
