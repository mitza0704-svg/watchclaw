#!/usr/bin/env bash
# One-time setup on zmfbot: installs a systemd timer that auto-deploys Watchclaw
# from GitHub every 2 minutes. Run once; after that zmfbot self-updates on every
# push — no SSH from Dell needed.
set -euo pipefail

sudo tee /etc/systemd/system/watchclaw-autodeploy.service >/dev/null <<EOF
[Unit]
Description=Watchclaw auto-deploy from GitHub
After=network-online.target docker.service

[Service]
Type=oneshot
User=$USER
ExecStart=/bin/bash /opt/watchclaw/src/infra/auto-deploy.sh
EOF

sudo tee /etc/systemd/system/watchclaw-autodeploy.timer >/dev/null <<EOF
[Unit]
Description=Run Watchclaw auto-deploy every 2 minutes

[Timer]
OnBootSec=60
OnUnitActiveSec=120
Unit=watchclaw-autodeploy.service

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now watchclaw-autodeploy.timer
echo "=== timer installed ==="
systemctl status watchclaw-autodeploy.timer --no-pager | head -4
echo "=== first run now ==="
sudo systemctl start watchclaw-autodeploy.service
sleep 2
journalctl -u watchclaw-autodeploy.service --no-pager -n 5
