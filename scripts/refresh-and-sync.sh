#!/bin/bash
# Auto-refresh Claude credentials and sync to K8s cluster.
# Called by launchd every 2 hours.

set -euo pipefail

LOG="/tmp/claude-creds-sync.log"
CREDS="$HOME/.claude/.credentials.json"
NAMESPACE="factory"

echo "$(date): Starting credential sync" >> "$LOG"

# Trigger a token refresh by running a trivial Claude command
claude -p "echo hello" --max-turns 1 --output-format text > /dev/null 2>&1 || true

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" > /dev/null 2>&1; then
    echo "$(date): K8s namespace $NAMESPACE not found, skipping" >> "$LOG"
    exit 0
fi

# Force-replace the secret (delete + create, not apply)
if [ -f "$CREDS" ]; then
    kubectl delete secret claude-credentials --namespace "$NAMESPACE" >> "$LOG" 2>&1 || true
    kubectl create secret generic claude-credentials \
        --namespace "$NAMESPACE" \
        --from-file=credentials.json="$CREDS" >> "$LOG" 2>&1
    
    # Restart ALL pods that use Claude credentials
    for dep in idea-engine-loop factory-pipeline factory-pilot; do
        kubectl rollout restart deployment "$dep" --namespace "$NAMESPACE" >> "$LOG" 2>&1 || true
    done
    
    echo "$(date): Credentials synced + pods restarted" >> "$LOG"
else
    echo "$(date): No credentials file found" >> "$LOG"
fi
