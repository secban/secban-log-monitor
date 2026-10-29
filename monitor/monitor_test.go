package monitor_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/monitor"
	"gitlab.com/secban/secban-log-monitor/reporter"
	"gitlab.com/secban/secban-log-monitor/watcher"
)

type mockReporter struct {
	mu      sync.Mutex
	reports []reportItem
}

type reportItem struct {
	ip   string
	rule string
	line string
}

func (m *mockReporter) Report(_ context.Context, params reporter.ReporterParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reports = append(
		m.reports,
		reportItem{ip: params.IP, rule: params.Rule, line: params.Line},
	)

	return nil
}

type mockWatcher struct {
	name    string
	trigger func(handler watcher.Handler)
}

func (w *mockWatcher) Name() string {
	return w.name
}

func (w *mockWatcher) Watch(ctx context.Context, handler watcher.Handler) error {
	if w.trigger != nil {
		w.trigger(handler)
	}

	<-ctx.Done()

	return ctx.Err()
}

func TestMonitorNew(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AgentURL: "http://127.0.0.1:1024",
		LogLevel: 1,
		Watchers: map[string]config.Watcher{
			"w1": {
				Name:    "w1",
				Command: "tail",
				IPRegex: "(?P<ip>\\S+)",
			},
		},
		Matchers: map[string]config.Matcher{
			"m1": {
				Name:         "m1",
				Watcher:      "w1",
				MatchRegexes: []string{"err"},
				Rule:         "bad",
			},
		},
	}

	_, err := monitor.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating monitor: %v", err)
	}
}

func TestMonitorStartAndReport(t *testing.T) {
	t.Parallel()

	rep := &mockReporter{}

	watcherOne := &mockWatcher{
		name: "testWatcher",
		trigger: func(handler watcher.Handler) {
			handler(watcher.HandlerParam{
				IP:      "192.0.2.5",
				Rule:    "bad",
				Matcher: "testMatcher",
				Line:    "bad request from 192.0.2.5",
			})
		},
	}

	monitorInstance := monitor.NewWithCustom([]watcher.WatchRunner{watcherOne}, rep)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		monitorInstance.Start(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not shut down in time")
	}

	rep.mu.Lock()
	defer rep.mu.Unlock()

	if len(rep.reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(rep.reports))
	}

	if rep.reports[0].ip != "192.0.2.5" {
		t.Errorf("expected ip 192.0.2.5, got %s", rep.reports[0].ip)
	}

	if rep.reports[0].rule != "bad" {
		t.Errorf("expected rule bad, got %s", rep.reports[0].rule)
	}
}
