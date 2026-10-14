#!/bin/bash
set -ex

REPO=$(git rev-parse --show-toplevel)

rm -rf build
mkdir -p build

BUILD_OPTIONS='-trimpath'
LDFLAGS="-ldflags=-w -s"

go build "$BUILD_OPTIONS" -o "$REPO/build/secban-log-monitor" "$LDFLAGS" "$REPO/cmd/monitor/monitor.go"
go build "$BUILD_OPTIONS" -o "$REPO/build/secban-log-checkmatch" "$LDFLAGS" "$REPO/cmd/checkmatch/checkmatch.go"

cp "$REPO/scripts/install.sh" "$REPO/build/"
cp "$REPO/scripts/systemd/secban-log-monitor.service" "$REPO/build/"
cp "$REPO/config/monitor.yaml" "$REPO/build/"
cp -r "$REPO/config/watchers.d" "$REPO/build/"
cp -r "$REPO/config/matchers.d" "$REPO/build/"

chmod +x "$REPO/build/install.sh"
