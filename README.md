# SecBan Log Monitor

SecBan Log Monitor is a log surveillance daemon for the SecBan framework. It watches log streams via external commands (`tail`, `journalctl`), extracts client IP addresses, evaluates lines against pattern matchers, and reports offending IPs to the SecBan agent via HTTP or UNIX domain socket.

## Features

- Log stream monitoring using standard tools (`journalctl`, `tail -F`)
- Separation between log source watchers and detection matchers
- Regex extraction of IPv4 and IPv6 addresses
- Match and ignore regex filters per matcher
- Direct reporting to SecBan agent over UNIX socket or HTTP
- Command-line testing tool (`secban-checkmatch`) to validate rules against recent logs
- Systemd service integration

## Installation

Pre-compiled release packages are published in the GitLab package registry.

### Download and Extract

```sh
# Set target version, OS, and architecture
VERSION="v0.1.0" # replace with desired version
OS="linux"
ARCH="amd64" # or arm64

# Download archive from GitLab Generic Package Registry
curl -sSL -o "secban-log-monitor.tar.gz" \
  "https://gitlab.com/api/v4/projects/secban%2Fsecban-log-monitor/packages/generic/secban-log-monitor/${VERSION}/secban-log-monitor-${OS}-${ARCH}-${VERSION}.tar.gz"

# Extract archive
tar -xzf "secban-log-monitor.tar.gz" -C /tmp/secban-install
cd /tmp/secban-install
```

### Install

Run the included install script as `root`:

```sh
sudo ./install.sh
```

The script performs the following steps:
1. Creates `secban` system group and `secban-log-monitor` system user.
2. Adds `secban-log-monitor` to log-reading groups (`adm`, `log`, `systemd-journal`).
3. Installs daemon `/usr/sbin/secban-log-monitor`.
4. Installs test utility `/usr/bin/secban-checkmatch`.
5. Copies initial configuration files to `/etc/secban/` (if not already present).
6. Installs and enables systemd service `secban-log-monitor.service`.

### Start Service

```sh
sudo systemctl start secban-log-monitor.service
sudo systemctl status secban-log-monitor.service
```

## Configuration

Configuration resides in `/etc/secban/`.

```
/etc/secban/
├── monitor.yaml       # Main daemon configuration
├── watchers.d/        # Log stream definitions (*.yaml)
└── matchers.d/        # Detection patterns and rule mappings (*.yaml)
```

### Main Configuration (`monitor.yaml`)

```yaml
# Agent endpoint to report offenders
agent_url: "unix:///var/run/secban/agent.sock"

# Logging level: trace, debug, info, warn, error
log_level: "info"
```

### Watchers (`watchers.d/*.yaml`)

Watchers execute commands that stream log output. They define how to read lines and how to extract the client IP.

| Field | Type | Description |
|---|---|---|
| `command` | string | Command binary to execute (e.g. `journalctl`, `tail`). |
| `args` | list of strings | Arguments used during daemon operation (streaming mode). |
| `test_args` | list of strings | Arguments used by `secban-checkmatch` (non-streaming, bounded output). |
| `ip_regex` | string | Regular expression containing named capture group `(?P<ip>...)` to extract target IP. |
| `env` | list of strings | Optional environment variables passed to command (e.g. `LANG=C`). |

### Matchers (`matchers.d/*.yaml`)

Matchers evaluate lines produced by a watcher against detection rules.

| Field | Type | Description |
|---|---|---|
| `watcher` | string | Name of associated watcher. |
| `rule` | string | Rule name sent to SecBan agent (must match rule configured in agent). |
| `match_regexes` | list of strings | Regex patterns that trigger a report when matched. |
| `ignore_regexes` | list of strings | Optional regex patterns to ignore even if `match_regexes` matched. |

## Examples

### 1. SSH Monitoring via `journalctl`

**Watcher** (`/etc/secban/watchers.d/sshd-journalctl.yaml`):

```yaml
sshd-journalctl:
  command: "journalctl"
  args:
    - "-afb"
    - "-p"
    - "info"
    - "-n0"
    - "-o"
    - "cat"
    - "SYSLOG_FACILITY=4"
    - "SYSLOG_FACILITY=10"
  test_args:
    - "-b"
    - "-p"
    - "info"
    - "-n500"
    - "-o"
    - "cat"
    - "SYSLOG_FACILITY=4"
    - "SYSLOG_FACILITY=10"
  env:
    - "LANG=C"
  ip_regex: '(?i:from|with|by|user\s+\S+)\s+(?P<ip>(?:[0-9a-fA-F]{1,4}:[0-9a-fA-F:]*[0-9a-fA-F]|::[0-9a-fA-F:]*|\d{1,3}(?:\.\d{1,3}){3}))(?:%\S+)?(?::\s|\s|$)'
```

**Matcher** (`/etc/secban/matchers.d/sshd.yaml`):

```yaml
sshd:
  watcher: "sshd-journalctl"
  rule: "standard"
  match_regexes:
    - "(?i) failure|invalid user|not allowed|login refused|authentication error|attempts exceeded|no supported authentication|auth fail|bad protocol|did not receive|unable to negotiate|illegal user|unknown user|invalid format"
    - '(?i:closed|disconnect|error|failed|reset).*\[preauth\]'
  ignore_regexes:
    - "closed by ::1 port"
```

### 2. NGINX Access Log Monitoring via `tail`

**Watcher** (`/etc/secban/watchers.d/nginx-combined.yaml`):

```yaml
nginx-combined:
  command: "tail"
  args:
    - "--follow=name"
    - "--retry"
    - "--quiet"
    - "-n0"
    - "/var/log/nginx/access.log"
  test_args:
    - "-n"
    - "500"
    - "/var/log/nginx/access.log"
  ip_regex: '^(?P<ip>\S+) - -'
```

**Matcher** (`/etc/secban/matchers.d/nginx-not-found.yaml`):

```yaml
nginx-not-found:
  watcher: "nginx-combined"
  rule: "not_found"
  match_regexes:
    - '" (?:404) '
  ignore_regexes:
    - 'GET /favicon\.ico HTTP'
```

### 3. Testing Patterns with `secban-checkmatch`

Run `secban-checkmatch` to test watchers and matchers against recent logs without sending reports:

```sh
secban-checkmatch -config /etc/secban sshd-journalctl
```

