#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

COUNT="${COUNT:-5}"
BENCH="${BENCH:-.}"

printf 'ZeroTUI benchmarks\n'
go version
printf '\nRunning %s benchmark repetitions...\n\n' "$COUNT"
go test ./benchmarks -run '^$' -bench "$BENCH" -benchmem -count="$COUNT"
