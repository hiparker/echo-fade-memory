#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BASE_URL="${EFM_BASE_URL:-$($SCRIPT_DIR/_resolve_base_url.py)}"
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
    f"{base_url.rstrip('/')}/v1/memories/{memory_id}",
    method="DELETE",
)
with urllib.request.urlopen(req, timeout=15) as resp:
    data = json.loads(resp.read().decode("utf-8"))
print(json.dumps(data, indent=2, ensure_ascii=False))
PY
