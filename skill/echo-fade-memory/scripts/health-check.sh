#!/bin/sh
set -eu

BASE_URL="${EFM_BASE_URL:-http://127.0.0.1:8080}"

curl -fsS "${BASE_URL%/}/v1/healthz"
