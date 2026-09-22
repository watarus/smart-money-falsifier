#!/usr/bin/env bash
# Probe one Nansen endpoint and show status, credit headers, and a body excerpt.
# Usage: scripts/probe.sh <path-after-/api/v1/> '<json body>' [body-chars]
set -euo pipefail
cd "$(dirname "$0")/.."
set -a; . ./.env; set +a

path=$1
body=$2
chars=${3:-400}

# Credits are scarce, so never throw a paid response away: every 200 becomes a
# fixture the Go build and its tests can run against offline.
mkdir -p data/fixtures
out=data/fixtures/$(echo "$path" | tr '/' '_').json
tmp=$(mktemp)

code=$(curl -s -D "$tmp.h" -o "$tmp" -w '%{http_code}' \
  -X POST "https://api.nansen.ai/api/v1/${path}" \
  -H 'Content-Type: application/json' \
  -H "apikey: $NANSEN_API_KEY" \
  -d "$body")

# Only a 200 becomes a fixture: an error body would otherwise clobber a good
# sample that the Go tests decode against.
if [ "$code" = 200 ]; then
  mv "$tmp" "$out"; mv "$tmp.h" "$out.headers"
  echo "--- $path -> $code  (saved: $out)"
else
  out=$tmp
  echo "--- $path -> $code  (not saved)"
fi

grep -i '^x-nansen-credits' "${out}.headers" "$tmp.h" 2>/dev/null | tr -d '\r' | sed 's/^[^:]*json.headers://' || true
head -c "$chars" "$out"; echo
