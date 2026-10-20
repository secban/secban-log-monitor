#!/bin/sh
set -e

mkdir -p /var/run/secban
chown secban:secban /var/run/secban
chmod 0770 /var/run/secban

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable secban-log-monitor.service || true
fi
