#!/bin/sh
set -eu

BASE_URL="${EFM_BASE_URL:-http://127.0.0.1:8080}"
ID="${1:-}"

if [ -z "$ID" ]; then
  echo "usage: $(basename "$0") <memory-id>" >&2
  exit 1
fi

python3 - "$BASE_URL" "$ID" <<'PY'
import json
import sys
import urllib.request

base_url, memory_id = sys.argv[1], sys.argv[2]
req = urllib.request.Request(
    f"{base_url.rstrip('/')}/v1/memories/{memory_id}/reinforce",
    data=b"",
    method="POST",
)
with urllib.request.urlopen(req, timeout=15) as resp:
    data = json.loads(resp.read().decode("utf-8"))
print(json.dumps(data, indent=2, ensure_ascii=False))
PY
