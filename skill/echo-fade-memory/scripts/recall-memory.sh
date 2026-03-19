#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BASE_URL="${EFM_BASE_URL:-$($SCRIPT_DIR/_resolve_base_url.py)}"
QUERY="${1:-}"
K="${2:-5}"

if [ -z "$QUERY" ]; then
  echo "usage: $(basename "$0") <query> [k]" >&2
  exit 1
fi

python3 - "$BASE_URL" "$QUERY" "$K" <<'PY'
import json
import sys
import urllib.parse
import urllib.request

base_url, query, k = sys.argv[1], sys.argv[2], sys.argv[3]

params = urllib.parse.urlencode({
    "query": query,
    "limit": k,
})
url = f"{base_url.rstrip('/')}/v1/memories/recall?{params}"
with urllib.request.urlopen(url, timeout=15) as resp:
    data = json.loads(resp.read().decode("utf-8"))

if isinstance(data, dict) and "results" in data:
    print(json.dumps(data, indent=2, ensure_ascii=False))
else:
    print(json.dumps({"results": data}, indent=2, ensure_ascii=False))
PY
