#!/usr/bin/env bash
# Watchclaw — isolated deploy on zmfbot. Clones to /opt/watchclaw (separate from
# /opt/openclaw-stack), runs as its own Docker container/network/volume.
# Idempotent: re-run to update.
set -euo pipefail

REPO="https://github.com/mitza0704-svg/watchclaw.git"
ROOT="/opt/watchclaw"

echo ">> preparing $ROOT (isolated from OpenClaw)"
sudo mkdir -p "$ROOT"
sudo chown "$USER" "$ROOT"

if [ -d "$ROOT/src/.git" ]; then
  echo ">> updating existing checkout"
  cd "$ROOT/src" && git pull --ff-only
else
  echo ">> cloning"
  git clone "$REPO" "$ROOT/src"
fi

# Stop the earlier systemd binary deploy (if present) to free port 8787.
if systemctl list-unit-files | grep -q '^watchclaw\.service'; then
  echo ">> stopping old systemd deploy"
  sudo systemctl disable --now watchclaw || true
fi

echo ">> building + starting isolated container"
cd "$ROOT/src/infra"
docker compose up -d --build

echo ">> status"
docker compose ps
echo ">> health"
sleep 3
curl -fsS http://localhost:8787/healthz && echo " <- watchclaw OK (isolated)"
