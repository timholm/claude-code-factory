#!/bin/bash
# Syncs Claude Code credentials from your Mac into the K8s cluster.
# Run this after `claude login` or when tokens expire.
#
# Usage: ./scripts/sync-claude-creds.sh

set -euo pipefail

NAMESPACE="factory"
SECRET_NAME="claude-credentials"
CREDS_FILE="$HOME/.claude/.credentials.json"

if [ ! -f "$CREDS_FILE" ]; then
    echo "ERROR: $CREDS_FILE not found. Run 'claude login' first."
    exit 1
fi

# Check if token is expired
EXPIRES_AT=$(python3 -c "import json; d=json.load(open('$CREDS_FILE')); print(d.get('claudeAiOauth',{}).get('expiresAt',0))" 2>/dev/null || echo "0")
NOW_MS=$(python3 -c "import time; print(int(time.time()*1000))")

if [ "$EXPIRES_AT" -lt "$NOW_MS" ]; then
    echo "WARNING: Token is expired. Run 'claude login' to refresh, then run this script again."
    exit 1
fi

EXPIRES_HUMAN=$(python3 -c "from datetime import datetime; print(datetime.fromtimestamp($EXPIRES_AT/1000).strftime('%Y-%m-%d %H:%M'))")
echo "Token valid until: $EXPIRES_HUMAN"

# Create/update the secret
kubectl create secret generic "$SECRET_NAME" \
    --namespace "$NAMESPACE" \
    --from-file=credentials.json="$CREDS_FILE" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "Claude credentials synced to $NAMESPACE/$SECRET_NAME"
