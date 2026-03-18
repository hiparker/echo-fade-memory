#!/bin/sh
set -eu

BASE_URL="${EFM_BASE_URL:-http://127.0.0.1:8080}"
CONTENT="${1:-}"
shift || true

if [ -z "$CONTENT" ]; then
  echo "usage: $(basename "$0") <content> [--type TYPE] [--summary TEXT] [--importance N] [--ref REF] [--kind KIND] [--conflict-group GROUP]" >&2
  exit 1
fi

TYPE="project"
SUMMARY=""
IMPORTANCE="0.8"
REF="session:manual"
KIND="chat"
CONFLICT_GROUP=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --type)
      TYPE="${2:-}"
      shift 2
      ;;
    --summary)
      SUMMARY="${2:-}"
      shift 2
      ;;
    --importance)
      IMPORTANCE="${2:-}"
      shift 2
      ;;
    --ref)
      REF="${2:-}"
      shift 2
      ;;
    --kind)
      KIND="${2:-}"
      shift 2
      ;;
    --conflict-group)
      CONFLICT_GROUP="${2:-}"
      shift 2
      ;;
    *)
      echo "unknown option: $1" >&2
      exit 1
      ;;
  esac
done

python3 - "$BASE_URL" "$CONTENT" "$TYPE" "$SUMMARY" "$IMPORTANCE" "$KIND" "$REF" "$CONFLICT_GROUP" <<'PY'
import json
import sys
import urllib.request

base_url, content, memory_type, summary, importance, kind, ref, conflict_group = sys.argv[1:]

payload = {
    "content": content,
    "memory_type": memory_type,
    "importance": float(importance),
    "source_refs": [{"kind": kind, "ref": ref}],
}
if summary:
    payload["summary"] = summary
if conflict_group:
    payload["conflict_group"] = conflict_group

req = urllib.request.Request(
    f"{base_url.rstrip('/')}/v1/memories",
    data=json.dumps(payload).encode("utf-8"),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=15) as resp:
    data = json.loads(resp.read().decode("utf-8"))
print(json.dumps(data, indent=2, ensure_ascii=False))
PY
