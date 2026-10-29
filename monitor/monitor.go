package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/matcher"
	"gitlab.com/secban/secban-log-monitor/reporter"
	"gitlab.com/secban/secban-log-monitor/watcher"
)

const defaultTimeout = 1 * time.Second

type Monitor struct {
	watchers []watcher.WatchRunner
	reporter reporter.Reporter
}

func New(cfg config.Config) (Monitor, error) {
	rep := reporter.New(cfg.AgentURL, defaultTimeout)

	watcherMatchers := make(map[string][]*matcher.Matcher)

	for _, mCfg := range cfg.Matchers {
		matcherInstance, newMatcherErr := matcher.New(
			mCfg.Name,
			mCfg.Rule,
			mCfg.MatchRegexes,
			mCfg.IgnoreRegexes,
		)
		if newMatcherErr != nil {
			return Monitor{}, fmt.Errorf("failed to create matcher %q: %w", mCfg.Name, newMatcherErr)
		}

		watcherMatchers[mCfg.Watcher] = append(watcherMatchers[mCfg.Watcher], matcherInstance)
	}

	watchers := make([]watcher.WatchRunner, 0, len(cfg.Watchers))

	for _, wCfg := range cfg.Watchers {
		matchers := watcherMatchers[wCfg.Name]

		w, newErr := watcher.New(wCfg, matchers)
		if newErr != nil {
			return Monitor{}, fmt.Errorf("failed to create watcher %q: %w", wCfg.Name, newErr)
		}

		watchers = append(watchers, w)
	}

	return Monitor{
		watchers: watchers,
		reporter: rep,
	}, nil
}

func NewWithCustom(watchers []watcher.WatchRunner, rep reporter.Reporter) Monitor {
	return Monitor{
		watchers: watchers,
		reporter: rep,
	}
}

func (m Monitor) Start(ctx context.Context) {
	var waitGroup sync.WaitGroup

	log.Info().Int("watchers_count", len(m.watchers)).Msg("starting monitor")

	for _, watcherInstance := range m.watchers {
		waitGroup.Add(1)

		go func(currentWatcher watcher.WatchRunner) {
			defer waitGroup.Done()

			watchErr := currentWatcher.Watch(
				ctx,
				func(param watcher.HandlerParam) {
					params := reporter.ReporterParams{
						IP:          param.IP,
						Rule:        param.Rule,
						WatcherName: currentWatcher.Name(),
						MatcherName: param.Matcher,
						Line:        param.Line,
					}

					reportErr := m.reporter.Report(ctx, params)
					if reportErr != nil {
						log.Warn().
							Str("watcher", currentWatcher.Name()).
							Str("ip", param.IP).
							Str("rule", param.Rule).
							Str("matcher", param.Matcher).
							Err(reportErr).
							Msg("failed to report offender to agent")
					}
				},
			)

			if watchErr != nil && ctx.Err() == nil {
				log.Error().
					Str("watcher", currentWatcher.Name()).
					Err(watchErr).
					Msg("watcher stopped with error")
			}
		}(watcherInstance)
	}

	<-ctx.Done()

	log.Info().Msg("context cancelled, waiting for watchers to exit")

	waitGroup.Wait()

	log.Info().Msg("all watchers stopped cleanly")
}
