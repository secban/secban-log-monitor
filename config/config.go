package config

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"gitlab.com/secban/secban-agent/configloader"
)

const (
	fileName    = "monitor.yaml"
	readersDir  = "readers.d"
	watchersDir = "watchers.d"
	matchersDir = "matchers.d"
)

type Reader struct {
	Name     string   `koanf:"name"`
	Command  string   `koanf:"command"`
	Args     []string `koanf:"args"`
	TestArgs []string `koanf:"test_args"`
	Env      []string `koanf:"env"`
}

type Watcher struct {
	Name     string   `koanf:"name"`
	Reader   string   `koanf:"reader"`
	Command  string   `koanf:"command"`
	Args     []string `koanf:"args"`
	TestArgs []string `koanf:"test_args"`
	Env      []string `koanf:"env"`
	IPRegex  string   `koanf:"ip_regex"`
}

type Matcher struct {
	Name          string   `koanf:"name"`
	Watcher       string   `koanf:"watcher"`
	Rule          string   `koanf:"rule"`
	MatchRegexes  []string `koanf:"match_regexes"`
	IgnoreRegexes []string `koanf:"ignore_regexes"`
}

type ParsedConfig struct {
	Profiler string             `koanf:"profiler"`
	AgentURL string             `koanf:"agent_url"`
	LogLevel string             `koanf:"log_level"`
	Readers  map[string]Reader  `koanf:"readers"`
	Watchers map[string]Watcher `koanf:"watchers"`
	Matchers map[string]Matcher `koanf:"matchers"`
}

type Config struct {
	AgentURL string
	Profiler string
	LogLevel zerolog.Level
	Readers  map[string]Reader
	Watchers map[string]Watcher
	Matchers map[string]Matcher
}

func LoadConfig(configDir string) (Config, error) {
	mainConfigPath := path.Join(configDir, fileName)
	scanDirs := []string{
		path.Join(configDir, readersDir),
		path.Join(configDir, watchersDir),
		path.Join(configDir, matchersDir),
	}

	parsed, parseErr := configloader.ParseConfig(defaultParsedConfig(), mainConfigPath, scanDirs)
	if parseErr != nil {
		return Config{}, parseErr
	}

	transformed, transformErr := transformConfig(parsed)
	if transformErr != nil {
		return Config{}, transformErr
	}

	return transformed, nil
}

func transformConfig(parsed ParsedConfig) (Config, error) {
	agentURL, resolveErr := resolveAgentURL(parsed.AgentURL)
	if resolveErr != nil {
		return Config{}, resolveErr
	}

	for k, v := range parsed.Readers {
		v.Name = k
		parsed.Readers[k] = v
	}

	for k, v := range parsed.Watchers {
		v.Name = k
		parsed.Watchers[k] = v
	}

	for k, v := range parsed.Matchers {
		v.Name = k
		parsed.Matchers[k] = v
	}

	resolveWatchersErr := resolveWatchers(parsed.Watchers, parsed.Readers)
	if resolveWatchersErr != nil {
		return Config{}, resolveWatchersErr
	}

	validateErr := validateConfig(parsed.Watchers, parsed.Matchers)
	if validateErr != nil {
		return Config{}, validateErr
	}

	config := Config{
		AgentURL: agentURL,
		Profiler: parsed.Profiler,
		Readers:  parsed.Readers,
		Watchers: parsed.Watchers,
		Matchers: parsed.Matchers,
	}

	return config, nil
}

func resolveAgentURL(agentURL string) (string, error) {
	agentURL = strings.TrimRight(agentURL, "/")
	if strings.HasPrefix(agentURL, "unix://") {
		return agentURL, nil
	}

	parsedURL, parseURLErr := url.Parse(agentURL)
	if parseURLErr != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("invalid agent_url %q", agentURL)
	}

	return agentURL, nil
}

func resolveWatchers(watchers map[string]Watcher, readers map[string]Reader) error {
	for watcherName, watcherConfig := range watchers {
		if watcherConfig.Reader == "" {
			return fmt.Errorf("watcher %q has no reader specified", watcherName)
		}

		readerConfig, readerExists := readers[watcherConfig.Reader]
		if !readerExists {
			return fmt.Errorf(
				"watcher %q references unknown reader %q",
				watcherName,
				watcherConfig.Reader,
			)
		}

		if readerConfig.Command == "" {
			return fmt.Errorf("reader %q has no command specified", watcherConfig.Reader)
		}

		watcherConfig.Command = readerConfig.Command
		watcherConfig.Env = append(slicesClone(readerConfig.Env), watcherConfig.Env...)

		mergedArgs := make([]string, 0, len(readerConfig.Args)+len(watcherConfig.Args))
		mergedArgs = append(mergedArgs, readerConfig.Args...)
		mergedArgs = append(mergedArgs, watcherConfig.Args...)

		mergedTestArgs := make([]string, 0, len(readerConfig.TestArgs)+len(watcherConfig.Args))
		mergedTestArgs = append(mergedTestArgs, readerConfig.TestArgs...)
		mergedTestArgs = append(mergedTestArgs, watcherConfig.Args...)

		watcherConfig.Args = mergedArgs
		watcherConfig.TestArgs = mergedTestArgs

		watchers[watcherName] = watcherConfig
	}

	return nil
}

func slicesClone[T any](src []T) []T {
	if src == nil {
		return nil
	}

	clone := make([]T, len(src))
	copy(clone, src)

	return clone
}

func validateConfig(watchers map[string]Watcher, matchers map[string]Matcher) error {
	// if len(watchers) == 0 {
	// 	return errors.New("no watchers configured")
	// }
	validateWatchersErr := validateWatchers(watchers)
	if validateWatchersErr != nil {
		return validateWatchersErr
	}

	validateMatchersErr := validateMatchers(watchers, matchers)
	if validateMatchersErr != nil {
		return validateMatchersErr
	}

	return nil
}

func validateWatchers(watchers map[string]Watcher) error {
	for watcherName, watcherConfig := range watchers {
		if watcherConfig.Command == "" {
			return fmt.Errorf("watcher %q has no command specified", watcherName)
		}

		if watcherConfig.IPRegex == "" {
			return fmt.Errorf("watcher %q has no ip_regex specified", watcherName)
		}

		compiledIP, compileIPErr := regexp.Compile(watcherConfig.IPRegex)
		if compileIPErr != nil {
			return fmt.Errorf(
				"watcher %q invalid ip_regex %q: %w",
				watcherName,
				watcherConfig.IPRegex,
				compileIPErr,
			)
		}

		if compiledIP.NumSubexp() < 1 {
			return fmt.Errorf(
				"watcher %q ip_regex %q must have at least one capture group for ip",
				watcherName,
				watcherConfig.IPRegex,
			)
		}
	}

	return nil
}

func validateMatchers(watchers map[string]Watcher, matchers map[string]Matcher) error {
	for matcherName, matcherConfig := range matchers {
		if matcherConfig.Watcher == "" {
			return fmt.Errorf("matcher %q has no watcher specified", matcherName)
		}

		if _, watcherExists := watchers[matcherConfig.Watcher]; !watcherExists {
			return fmt.Errorf(
				"matcher %q references unknown watcher %q",
				matcherName,
				matcherConfig.Watcher,
			)
		}

		if matcherConfig.Rule == "" {
			return fmt.Errorf("matcher %q has no rule specified", matcherName)
		}

		if len(matcherConfig.MatchRegexes) == 0 {
			return fmt.Errorf("matcher %q has no match_regexes specified", matcherName)
		}

		validateMatchErr := validateRegexList(
			matcherName,
			"match_regexes",
			matcherConfig.MatchRegexes,
		)
		if validateMatchErr != nil {
			return validateMatchErr
		}

		validateIgnoreErr := validateRegexList(
			matcherName,
			"ignore_regexes",
			matcherConfig.IgnoreRegexes,
		)
		if validateIgnoreErr != nil {
			return validateIgnoreErr
		}
	}

	return nil
}

func validateRegexList(matcherName string, fieldName string, regexes []string) error {
	for idx, pattern := range regexes {
		if pattern == "" {
			return fmt.Errorf("matcher %q %s[%d] cannot be empty", matcherName, fieldName, idx)
		}

		_, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			return fmt.Errorf(
				"matcher %q invalid %s[%d] %q: %w",
				matcherName,
				fieldName,
				idx,
				pattern,
				compileErr,
			)
		}
	}

	return nil
}
