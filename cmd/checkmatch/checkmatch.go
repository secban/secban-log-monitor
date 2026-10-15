package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/matcher"
	"gitlab.com/secban/secban-log-monitor/watcher"
)

func main() {
	err := run()
	if err != nil {
		//nolint: forbidigo
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	var configDir string

	flag.StringVar(&configDir, "config", "/etc/secban", "path to config directory")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		return errors.New("usage: checkmatch [-config <dir>] <watcher_name>")
	}

	targetWatcher := args[0]

	conf, confErr := config.ParseConfig(configDir)
	if confErr != nil {
		return fmt.Errorf("failed to parse config: %w", confErr)
	}

	setLogLevel(conf.LogLevel)

	watcherCfg, watcherExists := conf.Watchers[targetWatcher]
	if !watcherExists {
		return fmt.Errorf("watcher %q not found in config", targetWatcher)
	}

	matchers, matcherErr := buildWatcherMatchers(conf.Matchers, targetWatcher)
	if matcherErr != nil {
		return matcherErr
	}

	watcherInstance, watcherErr := watcher.New(watcherCfg, matchers)
	if watcherErr != nil {
		return fmt.Errorf("failed to create watcher %q: %w", targetWatcher, watcherErr)
	}

	matchCount, matchErr := checkWatcher(targetWatcher, watcherInstance)
	if matchErr != nil {
		return matchErr
	}

	log.Info().
		Str("watcher", targetWatcher).
		Int("matches", matchCount).
		Msg("check completed")

	return nil
}

func setLogLevel(logLevel string) {
	level, parseLevelErr := zerolog.ParseLevel(logLevel)
	if parseLevelErr != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)
}

func buildWatcherMatchers(
	matchersConfig map[string]config.Matcher,
	targetWatcher string,
) ([]*matcher.Matcher, error) {
	var matchers []*matcher.Matcher

	for _, matcherConfig := range matchersConfig {
		if matcherConfig.Watcher != targetWatcher {
			continue
		}

		matcherInstance, matcherErr := matcher.New(
			matcherConfig.Name,
			matcherConfig.Rule,
			matcherConfig.MatchRegexes,
			matcherConfig.IgnoreRegexes,
		)
		if matcherErr != nil {
			return nil, fmt.Errorf(
				"failed to create matcher %q: %w",
				matcherConfig.Name,
				matcherErr,
			)
		}

		matchers = append(matchers, matcherInstance)
	}

	return matchers, nil
}

func checkWatcher(targetWatcher string, watcherInstance *watcher.Watcher) (int, error) {
	matchCount := 0

	checkErr := watcherInstance.Check(
		context.Background(),
		func(param watcher.HandlerParam) {
			matchCount++
			//nolint: forbidigo
			fmt.Printf(
				"[%s] IP: %-15s Rule: %-10s Matcher: %-10s Line: %s\n",
				targetWatcher,
				param.IP,
				param.Rule,
				param.Matcher,
				param.Line,
			)
		},
	)
	if checkErr != nil {
		return 0, fmt.Errorf("check command failed for watcher %q: %w", targetWatcher, checkErr)
	}

	return matchCount, nil
}
