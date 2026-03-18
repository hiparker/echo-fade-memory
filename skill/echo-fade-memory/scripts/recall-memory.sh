#!/bin/sh
set -eu

BASE_URL="${EFM_BASE_URL:-http://127.0.0.1:8080}"
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
url = f"{base_url.rstrip('/')}/v1/memories?q={urllib.parse.quote(query)}&k={urllib.parse.quote(k)}"
with urllib.request.urlopen(url, timeout=15) as resp:
    data = json.loads(resp.read().decode("utf-8"))
print(json.dumps(data, indent=2, ensure_ascii=False))
PY
