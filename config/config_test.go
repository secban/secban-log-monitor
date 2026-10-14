package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/thequietbot/secban/secban-log-monitor/config"
)

func TestParseConfigValid(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
agent_url: "http://127.0.0.1:2048"
log_level: "debug"
watchers:
  nginx:
    command: "tail"
    args: ["-f", "/var/log/nginx/access.log"]
    ip_regex: '^(?P<ip>\S+) - -'
matchers:
  nginx_bad:
    watcher: "nginx"
    rule: "bad"
    match_regexes:
      - '404'
      - '403'
`)

	conf := loadConfig(t, configDir)
	assertAgentAndLogValues(t, conf, "http://127.0.0.1:2048", "debug")
	assertWatcherConfig(t, conf)
	assertMatcherConfig(t, conf)
}

func TestParseConfigDirs(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeConfigFile(t, `
log_level: "info"
`, configDir)

	writeDirEntry(t, configDir, "watchers.d", "journal.yaml", `
journal:
  command: "journalctl"
  args: ["-f"]
  env: ["LANG=C"]
  ip_regex: 'from (?P<ip>\S+)'
`)

	writeDirEntry(t, configDir, "matchers.d", "ssh.yaml", `
ssh_evil:
  watcher: "journal"
  rule: "evil"
  match_regexes:
    - 'failed password'
`)

	conf := loadConfig(t, configDir)
	if conf.AgentURL != "http://127.0.0.1:1024" {
		t.Errorf("expected default agent_url, got %s", conf.AgentURL)
	}

	watcherConfig, exists := conf.Watchers["journal"]
	if !exists {
		t.Fatal("expected journal watcher")
	}

	if len(watcherConfig.Env) != 1 || watcherConfig.Env[0] != "LANG=C" {
		t.Errorf("expected env [LANG=C], got %v", watcherConfig.Env)
	}

	matcherConfig, matcherExists := conf.Matchers["ssh_evil"]
	if !matcherExists {
		t.Fatal("expected ssh_evil matcher")
	}

	if matcherConfig.Rule != "evil" {
		t.Errorf("expected rule evil, got %s", matcherConfig.Rule)
	}
}

func TestParseConfigInvalidRegex(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
watchers:
  broken:
    command: "tail"
    ip_regex: '(?P<ip>\S+)'
matchers:
  m1:
    watcher: "broken"
    rule: "bad"
    match_regexes:
      - '[invalid'
`)

	_, parseErr := config.ParseConfig(configDir)
	if parseErr == nil {
		t.Fatal("expected parse error for invalid regex, got nil")
	}
}

func TestParseConfigUnknownWatcher(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
watchers:
  w1:
    command: "tail"
    ip_regex: '(?P<ip>\S+)'
matchers:
  m1:
    watcher: "unknown"
    rule: "bad"
    match_regexes:
      - 'error'
`)

	_, parseErr := config.ParseConfig(configDir)
	if parseErr == nil {
		t.Fatal("expected parse error for unknown watcher, got nil")
	}
}

func TestParseConfigMissingIPCaptureGroup(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
watchers:
  broken:
    command: "tail"
    ip_regex: 'no-groups-here'
`)

	_, parseErr := config.ParseConfig(configDir)
	if parseErr == nil {
		t.Fatal("expected parse error for ip_regex without capture groups, got nil")
	}
}

func TestParseConfigIgnoreRegexes(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
watchers:
  nginx:
    command: "tail"
    ip_regex: '^(?P<ip>\S+) - -'
matchers:
  nginx_m:
    watcher: "nginx"
    rule: "bad"
    match_regexes:
      - '404'
    ignore_regexes:
      - 'favicon\.ico'
      - 'robots\.txt'
`)

	conf := loadConfig(t, configDir)

	matcherConfig, exists := conf.Matchers["nginx_m"]
	if !exists {
		t.Fatal("expected nginx_m matcher")
	}

	if len(matcherConfig.IgnoreRegexes) != 2 || matcherConfig.IgnoreRegexes[0] != `favicon\.ico` {
		t.Errorf("unexpected ignore_regexes: %v", matcherConfig.IgnoreRegexes)
	}
}

func TestParseConfigInvalidIgnoreRegex(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, `
watchers:
  broken:
    command: "tail"
    ip_regex: '(?P<ip>\S+)'
matchers:
  broken_m:
    watcher: "broken"
    rule: "bad"
    match_regexes:
      - 'error'
    ignore_regexes:
      - '[invalid'
`)

	_, parseErr := config.ParseConfig(configDir)
	if parseErr == nil {
		t.Fatal("expected parse error for invalid ignore_regex, got nil")
	}
}

func writeConfigFile(t *testing.T, monitorYAML string, configDir ...string) string {
	t.Helper()

	dir := t.TempDir()
	if len(configDir) > 0 {
		dir = configDir[0]
	}

	writeErr := os.WriteFile(filepath.Join(dir, "monitor.yaml"), []byte(monitorYAML), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write monitor.yaml: %v", writeErr)
	}

	return dir
}

func writeDirEntry(
	t *testing.T,
	configDir string,
	subDir string,
	fileName string,
	contents string,
) {
	t.Helper()

	fullDir := filepath.Join(configDir, subDir)

	err := os.Mkdir(fullDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create %s: %v", subDir, err)
	}

	err = os.WriteFile(filepath.Join(fullDir, fileName), []byte(contents), 0o600)
	if err != nil {
		t.Fatalf("failed to write %s: %v", fileName, err)
	}
}

func loadConfig(t *testing.T, configDir string) *config.Config {
	t.Helper()

	conf, parseErr := config.ParseConfig(configDir)
	if parseErr != nil {
		t.Fatalf("unexpected error parsing config: %v", parseErr)
	}

	return conf
}

func assertAgentAndLogValues(
	t *testing.T,
	conf *config.Config,
	expectedAgentURL string,
	expectedLogLevel string,
) {
	t.Helper()

	if conf.AgentURL != expectedAgentURL {
		t.Errorf("expected agent_url %s, got %s", expectedAgentURL, conf.AgentURL)
	}

	if conf.LogLevel != expectedLogLevel {
		t.Errorf("expected log_level %s, got %s", expectedLogLevel, conf.LogLevel)
	}
}

func assertWatcherConfig(t *testing.T, conf *config.Config) {
	t.Helper()

	if len(conf.Watchers) != 1 {
		t.Fatalf("expected 1 watcher, got %d", len(conf.Watchers))
	}

	watcherConfig, exists := conf.Watchers["nginx"]
	if !exists {
		t.Fatal("expected nginx watcher")
	}

	if watcherConfig.Name != "nginx" {
		t.Errorf("expected watcher name nginx, got %s", watcherConfig.Name)
	}

	if watcherConfig.IPRegex != `^(?P<ip>\S+) - -` {
		t.Errorf("expected ip_regex, got %s", watcherConfig.IPRegex)
	}
}

func assertMatcherConfig(t *testing.T, conf *config.Config) {
	t.Helper()

	if len(conf.Matchers) != 1 {
		t.Fatalf("expected 1 matcher, got %d", len(conf.Matchers))
	}

	matcherConfig, exists := conf.Matchers["nginx_bad"]
	if !exists {
		t.Fatal("expected nginx_bad matcher")
	}

	if matcherConfig.Watcher != "nginx" {
		t.Errorf("expected matcher watcher nginx, got %s", matcherConfig.Watcher)
	}

	if matcherConfig.Rule != "bad" {
		t.Errorf("expected matcher rule bad, got %s", matcherConfig.Rule)
	}

	if len(matcherConfig.MatchRegexes) != 2 || matcherConfig.MatchRegexes[0] != "404" {
		t.Errorf("unexpected match_regexes: %v", matcherConfig.MatchRegexes)
	}
}
