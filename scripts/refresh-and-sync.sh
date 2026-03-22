#!/bin/bash
# Auto-refresh Claude credentials and sync to K8s cluster.
# Called by launchd every 4 hours.

set -euo pipefail

LOG="/tmp/claude-creds-sync.log"
CREDS="$HOME/.claude/.credentials.json"
NAMESPACE="factory"

echo "$(date): Starting credential sync" >> "$LOG"

# Trigger a token refresh by running a trivial Claude command
# This uses the refresh token to get a new access token
claude -p "echo hello" --max-turns 1 --output-format text > /dev/null 2>&1 || true

# Check if namespace exists (cluster might not be reachable)
if ! kubectl get namespace "$NAMESPACE" > /dev/null 2>&1; then
    echo "$(date): K8s namespace $NAMESPACE not found, skipping" >> "$LOG"
    exit 0
fi

# Sync to cluster
if [ -f "$CREDS" ]; then
    kubectl create secret generic claude-credentials \
        --namespace "$NAMESPACE" \
        --from-file=credentials.json="$CREDS" \
        --dry-run=client -o yaml | kubectl apply -f - >> "$LOG" 2>&1
    echo "$(date): Credentials synced successfully" >> "$LOG"
else
    echo "$(date): No credentials file found" >> "$LOG"
fi
