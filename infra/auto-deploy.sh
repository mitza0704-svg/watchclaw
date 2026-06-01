#!/usr/bin/env bash
# Watchclaw auto-deploy — run by a systemd timer on zmfbot.
# Pulls the repo; if there's a new commit, rebuilds + restarts the isolated stack.
# This is what decouples deploys from any direct Dell->zmfbot access: I push to
# GitHub from anywhere, zmfbot pulls itself.
set -euo pipefail

SRC="/opt/watchclaw/src"
cd "$SRC"

before=$(git rev-parse HEAD 2>/dev/null || echo none)
git fetch --quiet origin master || exit 0
git reset --hard --quiet origin/master
after=$(git rev-parse HEAD)

if [ "$before" != "$after" ]; then
  echo "$(date -Is) new commit $after — redeploying"
  cd "$SRC/infra"
  docker compose up -d --build
  echo "$(date -Is) redeploy done"
else
  echo "$(date -Is) up to date ($after)"
fi
