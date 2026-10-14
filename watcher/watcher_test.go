package watcher_test

import (
	"context"
	"strings"
	"testing"

	"gitlab.com/thequietbot/secban/secban-log-monitor/config"
	"gitlab.com/thequietbot/secban/secban-log-monitor/matcher"
	"gitlab.com/thequietbot/secban/secban-log-monitor/watcher"
)

func TestWatcherNew(t *testing.T) {
	t.Parallel()

	validCfg := config.Watcher{
		Name:     "nginx",
		Command:  "tail",
		Args:     []string{"-f", "/var/log/nginx/access.log"},
		TestArgs: []string{"-n", "100", "/var/log/nginx/access.log"},
		IPRegex:  `^(?P<ip>\S+) - -`,
	}

	m, matcherErr := matcher.New("nginx_bad", "bad", []string{`404`, `403`}, nil)
	if matcherErr != nil {
		t.Fatalf("unexpected error creating matcher: %v", matcherErr)
	}

	watcherInstance, err := watcher.New(validCfg, []*matcher.Matcher{m})
	if err != nil {
		t.Fatalf("unexpected error creating watcher: %v", err)
	}

	if watcherInstance.Name() != "nginx" {
		t.Errorf("expected name nginx, got %s", watcherInstance.Name())
	}

	if len(watcherInstance.Matchers()) != 1 {
		t.Errorf("expected 1 matcher, got %d", len(watcherInstance.Matchers()))
	}
}

func TestWatcherProcessStream(t *testing.T) {
	t.Parallel()

	mAlert, alertErr := matcher.New("alert", "critical", []string{`ALERT`}, nil)
	if alertErr != nil {
		t.Fatalf("unexpected error creating alert matcher: %v", alertErr)
	}

	mWarn, warnErr := matcher.New("warn", "warning", []string{`WARN`}, nil)
	if warnErr != nil {
		t.Fatalf("unexpected error creating warn matcher: %v", warnErr)
	}

	watcherInstance, err := watcher.New(config.Watcher{
		Name:    "test",
		Command: "cat",
		IPRegex: `(?P<ip>\d+\.\d+\.\d+\.\d+)`,
	}, []*matcher.Matcher{mAlert, mWarn})
	if err != nil {
		t.Fatalf("unexpected error creating watcher: %v", err)
	}

	streamData := "IGNORE this line\nALERT 10.0.0.1\nIGNORE other\nWARN 10.0.0.2\n"

	var (
		reportedIPs   []string
		reportedRules []string
	)

	processErr := watcherInstance.ProcessStream(
		strings.NewReader(streamData),
		func(param watcher.HandlerParam) {
			reportedIPs = append(reportedIPs, param.IP)
			reportedRules = append(reportedRules, param.Rule)
			_ = param.Matcher
			_ = param.Line
		},
	)
	if processErr != nil {
		t.Fatalf("unexpected process stream error: %v", processErr)
	}

	if len(reportedIPs) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reportedIPs))
	}

	if reportedIPs[0] != "10.0.0.1" || reportedIPs[1] != "10.0.0.2" {
		t.Errorf("unexpected ips: %v", reportedIPs)
	}

	if reportedRules[0] != "critical" || reportedRules[1] != "warning" {
		t.Errorf("unexpected rules: %v", reportedRules)
	}
}

func TestWatcherCheck(t *testing.T) {
	t.Parallel()

	matcherInstance, matcherErr := matcher.New("testmatch", "bad", []string{`TESTMATCH`}, nil)
	if matcherErr != nil {
		t.Fatalf("unexpected error creating matcher: %v", matcherErr)
	}

	watcherInstance, err := watcher.New(config.Watcher{
		Name:     "echo-test",
		Command:  "echo",
		Args:     []string{"DEFAULT 192.168.1.1"},
		TestArgs: []string{"TESTMATCH 10.10.10.10"},
		IPRegex:  `(?P<ip>\d+\.\d+\.\d+\.\d+)`,
	}, []*matcher.Matcher{matcherInstance})
	if err != nil {
		t.Fatalf("unexpected error creating watcher: %v", err)
	}

	var foundIP string

	checkErr := watcherInstance.Check(
		context.Background(),
		func(param watcher.HandlerParam) {
			foundIP = param.IP
		},
	)
	if checkErr != nil {
		t.Fatalf("unexpected check error: %v", checkErr)
	}

	if foundIP != "10.10.10.10" {
		t.Errorf("expected found ip 10.10.10.10 from test args, got %s", foundIP)
	}
}

func TestWatcherProcessStreamWithIgnoreRegexes(t *testing.T) {
	t.Parallel()

	matcherInstance, matcherErr := matcher.New(
		"test",
		"bad",
		[]string{`ALERT`},
		[]string{`internal`},
	)
	if matcherErr != nil {
		t.Fatalf("unexpected error creating matcher: %v", matcherErr)
	}

	watcherInstance, watcherErr := watcher.New(config.Watcher{
		Name:    "test-ignore",
		Command: "cat",
		IPRegex: `(?P<ip>\d+\.\d+\.\d+\.\d+)`,
	}, []*matcher.Matcher{matcherInstance})
	if watcherErr != nil {
		t.Fatalf("unexpected error creating watcher: %v", watcherErr)
	}

	streamData := "ALERT 10.0.0.1 internal traffic\nALERT 10.0.0.2 external attack\n"

	var reportedIPs []string

	processErr := watcherInstance.ProcessStream(
		strings.NewReader(streamData),
		func(param watcher.HandlerParam) {
			_ = param.Matcher
			reportedIPs = append(reportedIPs, param.IP)
		},
	)
	if processErr != nil {
		t.Fatalf("unexpected process stream error: %v", processErr)
	}

	if len(reportedIPs) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reportedIPs))
	}

	if reportedIPs[0] != "10.0.0.2" {
		t.Errorf("unexpected ip: %s", reportedIPs[0])
	}
}
