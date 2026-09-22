#!/usr/bin/env bash
# Break each guard the falsifier depends on, one at a time, and require the
# test suite to fail on an assertion. A guard whose removal still passes has
# no test protecting it. Every file is restored before the next mutation.
#
# Usage: scripts/mutate_check.sh
set -uo pipefail
cd "$(dirname "$0")/.."

run() { # name file old new
  local name=$1 file=$2 old=$3 new=$4 backup
  backup=$(mktemp)
  cp "$file" "$backup"
  if ! python3 - "$file" "$old" "$new" <<'PY'
import sys
path, old, new = sys.argv[1:4]
s = open(path).read()
if s.count(old) != 1:
    sys.exit(f"mutation target not unique in {path}: {old!r}")
open(path, "w").write(s.replace(old, new, 1))
PY
  then
    cp "$backup" "$file"; rm -f "$backup"
    echo "SETUP-ERROR $name"; return 1
  fi

  local out status
  out=$(NO_COLOR=1 go test ./... 2>&1); status=$?
  cp "$backup" "$file"; rm -f "$backup"

  # A build failure is not a caught mutation: it proves nothing about tests.
  if grep -q -- '--- FAIL' <<<"$out"; then
    echo "CAUGHT   $name"
  elif [ $status -ne 0 ]; then
    echo "BROKEN   $name (failed without a test assertion)"; return 1
  else
    echo "MISSED   $name"; return 1
  fi
}

fail=0
run "exit check gates CONFIRMED" internal/score/verdict.go \
  "		if !hasFlow {
			return VerdictIndependent
		}" "" || fail=1
run "signal floor withholds verdict" internal/pipeline/pipeline.go \
  "	if boughtUSD < minSignalUSD {" "	if false && boughtUSD < minSignalUSD {" || fail=1
run "null exchange avg stays distinguishable" internal/nansen/types.go \
  'ExchangeAvgFlowUSD      *float64 `json:"exchange_avg_flow_usd"`' \
  'ExchangeAvgFlowUSD      *float64 `json:"-"`' || fail=1
run "coverage gates CONFIRMED" internal/score/verdict.go \
  "		if covered < n {
			return VerdictUnverified
		}" "" || fail=1

NO_COLOR=1 go test ./... >/dev/null 2>&1 || { echo "suite not green after restore"; fail=1; }
exit $fail
