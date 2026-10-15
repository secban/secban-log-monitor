#!/bin/bash
set -ex

OWNER=$(whoami)
if [ "$OWNER" != "root" ]; then
    echo "This script must be run as root"
    exit 1
fi

# Ensure our location.
OWN_DIR=$(dirname "$0")
cd "$OWN_DIR"

# Create secban group if not exists.
if ! getent group secban >/dev/null 2>&1; then
    groupadd --system secban
fi

# Create user for secban-log-monitor in the secban group.
if ! id -u secban-log-monitor >/dev/null 2>&1; then
    useradd --no-create-home --system --shell /usr/sbin/nologin --gid secban secban-log-monitor
fi

# Add secban-log-monitor to log-reading groups if they exist.
for grp in adm log systemd-journal; do
    if getent group "$grp" >/dev/null 2>&1; then
        usermod -a -G "$grp" secban-log-monitor || true
    fi
done

# Install the secban-log-monitor daemon binary.
cp "./secban-log-monitor" "/usr/sbin/"
chmod 0755 "/usr/sbin/secban-log-monitor"

# Install the secban-checkmatch helper binary.
cp "./secban-log-checkmatch" "/usr/bin/"
chmod 0755 "/usr/bin/secban-log-checkmatch"

# Install configuration.
mkdir -p /etc/secban
if [ ! -f /etc/secban/monitor.yaml ]; then
    cp "./monitor.yaml" "/etc/secban/monitor.yaml"
    mkdir -p /etc/secban/watchers.d
    cp -r ./watchers.d/* "/etc/secban/watchers.d/"
    mkdir -p /etc/secban/matchers.d
    cp -r ./matchers.d/* "/etc/secban/matchers.d/"
fi

# Install systemd service.
if [ ! -f /etc/systemd/system/secban-log-monitor.service ]; then
    cp "./secban-log-monitor.service" "/etc/systemd/system/secban-log-monitor.service"
fi

if [ ! -z "$(which systemctl)" ]; then
    systemctl daemon-reload
    systemctl enable secban-log-monitor.service
fi
