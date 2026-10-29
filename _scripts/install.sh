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

# Create user for secban in the secban group.
if ! id -u secban >/dev/null 2>&1; then
    useradd --no-create-home --system --shell /sbin/nologin --gid secban secban
fi

# Add secban to log-reading groups if they exist.
for grp in adm log systemd-journal; do
    if getent group "$grp" >/dev/null 2>&1; then
        usermod -a -G "$grp" secban || true
    fi
done

# Install the secban-log-monitor daemon binary.
cp "./secban-log-monitor" "/usr/sbin/"
chmod 0755 "/usr/sbin/secban-log-monitor"

# Install the secban-checkmatch helper binary.
cp "./secban-log-checkmatch" "/usr/bin/"
chmod 0755 "/usr/bin/secban-log-checkmatch"

# Install the example agent configuration.
cp -ar "./etc/secban/" "/etc/secban/"

# Install systemd service.
if [ ! -z "$(which systemctl)" ]; then
    if [ ! -f /etc/systemd/system/secban-log-monitor.service ]; then
        cp "./secban-log-monitor.service" "/etc/systemd/system/secban-log-monitor.service"
    fi
    systemctl daemon-reload
    systemctl enable secban-log-monitor.service
fi

# Install OpenRC service.
if [ ! -z "$(which rc-update)" ]; then
    if [ ! -f /etc/init.d/secban-log-monitor ]; then
        cp "./secban-log-monitor.rc" "/etc/init.d/secban-log-monitor"
        chmod +x "/etc/init.d/secban-log-monitor"
    fi
    rc-update add secban-log-monitor default
fi
