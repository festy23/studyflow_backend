#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/generate-auth-token.sh [TELEGRAM_ID] [TELEGRAM_SECRET]
# If args are not provided, reads from .env in project root.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TELEGRAM_ID="${1:-}"
TELEGRAM_SECRET="${2:-}"

if [ -z "$TELEGRAM_ID" ] || [ -z "$TELEGRAM_SECRET" ]; then
  if [ -f "$PROJECT_ROOT/.env" ]; then
    # shellcheck disable=SC1090
    source "$PROJECT_ROOT/.env"
  fi
fi

if [ -z "$TELEGRAM_ID" ]; then
  read -rp "Telegram ID: " TELEGRAM_ID
fi
if [ -z "$TELEGRAM_SECRET" ]; then
  read -rsp "TELEGRAM_SECRET: " TELEGRAM_SECRET
  echo
fi

TIMESTAMP=$(date +%s)
MESSAGE="${TELEGRAM_ID}:${TIMESTAMP}"

if command -v python3 &>/dev/null; then
  HMAC=$(python3 -c "
import hmac, hashlib
print(hmac.new('$TELEGRAM_SECRET'.encode(), '$MESSAGE'.encode(), hashlib.sha256).hexdigest())
")
elif command -v openssl &>/dev/null; then
  HMAC=$(echo -n "$MESSAGE" | openssl dgst -sha256 -hmac "$TELEGRAM_SECRET" | awk '{print $NF}')
else
  echo "Error: need python3 or openssl" >&2
  exit 1
fi

HEADER="telegram ${MESSAGE}:${HMAC}"

echo
echo "=== Authorization Header (legacy HMAC) ==="
echo "$HEADER"
echo
echo "=== curl example ==="
echo "curl -H 'Authorization: ${HEADER}' http://localhost:80/health"
echo
echo "=== For Mini App (initData, no secret needed) ==="
echo "Use: Authorization: tma <initData from Telegram.WebApp.initData>"
