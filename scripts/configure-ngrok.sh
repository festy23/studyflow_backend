#!/usr/bin/env bash
set -euo pipefail
# Usage: ./scripts/configure-ngrok.sh
# Auto-detects the active ngrok public URL and updates .env with GATEWAY_PUBLIC_URL.
# Run this after starting ngrok and before docker compose up.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

NGROK_URL=$(curl -s http://127.0.0.1:4040/api/tunnels 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['tunnels'][0]['public_url'])" 2>/dev/null || echo "")

if [ -z "$NGROK_URL" ]; then
  echo "Error: ngrok not running or API not accessible. Start ngrok first:"
  echo "  ngrok http 80"
  exit 1
fi

if [ -f "$ENV_FILE" ]; then
  if grep -q "^GATEWAY_PUBLIC_URL=" "$ENV_FILE"; then
    sed -i '' "s|^GATEWAY_PUBLIC_URL=.*|GATEWAY_PUBLIC_URL=$NGROK_URL|" "$ENV_FILE"
  else
    echo "GATEWAY_PUBLIC_URL=$NGROK_URL" >>"$ENV_FILE"
  fi
else
  cp "$PROJECT_ROOT/.env.example" "$ENV_FILE"
  echo "GATEWAY_PUBLIC_URL=$NGROK_URL" >>"$ENV_FILE"
fi

echo "Updated GATEWAY_PUBLIC_URL=$NGROK_URL in .env"
echo "Run: docker compose up -d"
