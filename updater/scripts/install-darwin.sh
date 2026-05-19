#!/usr/bin/env bash
#
# Args: <parent-pid> <staged-app-path> <current-app-path>
# Replaces <current-app-path> with <staged-app-path> after <parent-pid> exits,
# then relaunches the .app via `open`.
#
set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "usage: $0 <parent-pid> <staged-app> <current-app>" >&2
    exit 2
fi

PARENT_PID="$1"
STAGED="$2"
APP_PATH="$3"

# Wait up to 30s for the parent (NetCatcher) to exit so we can replace its bundle.
for _ in $(seq 1 60); do
    if ! kill -0 "$PARENT_PID" 2>/dev/null; then
        break
    fi
    sleep 0.5
done

if kill -0 "$PARENT_PID" 2>/dev/null; then
    echo "parent $PARENT_PID still running after 30s" >&2
    exit 11
fi

# Swap the bundle.
if [ ! -d "$STAGED" ]; then
    echo "staged bundle missing: $STAGED" >&2
    exit 12
fi
rm -rf "$APP_PATH"
mv "$STAGED" "$APP_PATH"

# Clear quarantine so the relaunched bundle doesn't trip Gatekeeper.
xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true

# Re-launch.
open -a "$APP_PATH"
