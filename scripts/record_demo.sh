#!/usr/bin/env bash
# Record the 30-60s buildathon demo from the real report, with burned-in
# captions (the rules require it to be followable without audio).
#
# Renders out/report.html from cache first, so this spends zero credits.
# Output: out/demo.webm (raw capture) and out/demo.mp4 (H.264, for X).
#
# Usage: scripts/record_demo.sh
set -euo pipefail
cd "$(dirname "$0")/.."

SESSION=nansen-demo
REPORT="file://$PWD/out/report.html"
WEBM="$PWD/out/demo.webm"
MP4="$PWD/out/demo.mp4"
RUNTIME_MS=53000

NO_COLOR=1 ./bin/scorecard --offline >/dev/null 2>&1

cleanup() { agent-browser --session "$SESSION" close >/dev/null 2>&1 || true; }
trap cleanup EXIT

rm -f "$WEBM" "$MP4"
agent-browser --session "$SESSION" set viewport 1280 720 >/dev/null
agent-browser --session "$SESSION" record start "$WEBM" "$REPORT" >/dev/null
agent-browser --session "$SESSION" wait 800 >/dev/null
agent-browser --session "$SESSION" eval "$(cat scripts/demo/timeline.js)" >/dev/null
agent-browser --session "$SESSION" wait "$RUNTIME_MS" >/dev/null
agent-browser --session "$SESSION" record stop >/dev/null

# Drop the page-load frames before the first caption; re-encode for X.
ffmpeg -y -loglevel error -ss 1.0 -i "$WEBM" \
  -c:v libx264 -pix_fmt yuv420p -preset slow -crf 20 -movflags +faststart \
  -vf "scale=1280:-2" -an "$MP4"

ffprobe -v error -show_entries format=duration -of csv=p=0 "$MP4" | awk '{printf "duration %.1fs\n", $1}'
echo "wrote $MP4"
