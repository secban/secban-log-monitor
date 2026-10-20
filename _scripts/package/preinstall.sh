#!/bin/sh
set -e

if ! getent group secban >/dev/null 2>&1; then
    groupadd --system secban
fi

if ! id -u secban >/dev/null 2>&1; then
    useradd --no-create-home --system --shell /sbin/nologin --gid secban secban
fi

for grp in adm log systemd-journal; do
    if getent group "$grp" >/dev/null 2>&1; then
        usermod -a -G "$grp" secban || true
    fi
done
