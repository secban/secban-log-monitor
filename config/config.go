package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	fileName        = "monitor.yaml"
	defaultAgentURL = "http://127.0.0.1:1024"
	defaultLogLevel = "info"
	readersDir      = "readers.d"
	watchersDir     = "watchers.d"
	matchersDir     = "matchers.d"
)

type Reader struct {
	Name     string   `yaml:"name"`
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	TestArgs []string `yaml:"test_args"`
	Env      []string `yaml:"env"`
}

type Watcher struct {
	Name     string   `yaml:"name"`
	Reader   string   `yaml:"reader"`
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	TestArgs []string `yaml:"test_args"`
	Env      []string `yaml:"env"`
	IPRegex  string   `yaml:"ip_regex"`
}

type Matcher struct {
	Name          string   `yaml:"name"`
	Watcher       string   `yaml:"watcher"`
	Rule          string   `yaml:"rule"`
	MatchRegexes  []string `yaml:"match_regexes"`
	IgnoreRegexes []string `yaml:"ignore_regexes"`
}

type ParsedConfig struct {
	AgentURL string             `yaml:"agent_url"`
	LogLevel string             `yaml:"log_level"`
	Readers  map[string]Reader  `yaml:"readers"`
	Watchers map[string]Watcher `yaml:"watchers"`
	Matchers map[string]Matcher `yaml:"matchers"`
}

type Config struct {
	AgentURL string
	LogLevel string
	Readers  map[string]Reader
	Watchers map[string]Watcher
	Matchers map[string]Matcher
}

func ParseConfig(configDir string) (*Config, error) {
	mainConfigPath := path.Join(configDir, fileName)

	var parsed ParsedConfig

	configData, readErr := os.ReadFile(mainConfigPath)
	if readErr != nil {
		return nil, readErr
	}

	unmarshalErr := yaml.Unmarshal(configData, &parsed)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	agentURL, resolveErr := resolveAgentURL(parsed.AgentURL)
	if resolveErr != nil {
		return nil, resolveErr
	}

	logLevel := parsed.LogLevel
	if logLevel == "" {
		logLevel = defaultLogLevel
	}

	readersMap, loadReadersErr := loadReaders(configDir, parsed.Readers)
	if loadReadersErr != nil {
		return nil, loadReadersErr
	}

	watchersMap, loadWatchersErr := loadWatchers(configDir, parsed.Watchers)
	if loadWatchersErr != nil {
		return nil, loadWatchersErr
	}

	matchersMap, loadMatchersErr := loadMatchers(configDir, parsed.Matchers)
	if loadMatchersErr != nil {
		return nil, loadMatchersErr
	}

	resolveWatchersErr := resolveWatchers(watchersMap, readersMap)
	if resolveWatchersErr != nil {
		return nil, resolveWatchersErr
	}

	validateErr := validateConfig(watchersMap, matchersMap)
	if validateErr != nil {
		return nil, validateErr
	}

	config := Config{
		AgentURL: agentURL,
		LogLevel: logLevel,
		Readers:  readersMap,
		Watchers: watchersMap,
		Matchers: matchersMap,
	}

	return &config, nil
}

func resolveAgentURL(agentURL string) (string, error) {
	if agentURL == "" {
		agentURL = defaultAgentURL
	}

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

func readYAMLDir[T any](dirPath string) (map[string]T, error) {
	result := make(map[string]T)

	entries, readDirErr := os.ReadDir(dirPath)
	if readDirErr != nil {
		if os.IsNotExist(readDirErr) {
			return result, nil
		}

		return nil, readDirErr
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dirPath, name)

		data, readFileErr := os.ReadFile(filePath)
		if readFileErr != nil {
			return nil, readFileErr
		}

		var items map[string]T

		unmarshalErr := yaml.Unmarshal(data, &items)
		if unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filePath, unmarshalErr)
		}

		maps.Copy(result, items)
	}

	return result, nil
}

func loadReaders(configDir string, base map[string]Reader) (map[string]Reader, error) {
	merged := make(map[string]Reader)

	for k, v := range base {
		v.Name = k
		merged[k] = v
	}

	dirReaders, readDirErr := readYAMLDir[Reader](filepath.Join(configDir, readersDir))
	if readDirErr != nil {
		return nil, readDirErr
	}

	for k, v := range dirReaders {
		v.Name = k
		merged[k] = v
	}

	return merged, nil
}

func loadWatchers(configDir string, base map[string]Watcher) (map[string]Watcher, error) {
	merged := make(map[string]Watcher)

	for k, v := range base {
		v.Name = k
		merged[k] = v
	}

	dirWatchers, readDirErr := readYAMLDir[Watcher](filepath.Join(configDir, watchersDir))
	if readDirErr != nil {
		return nil, readDirErr
	}

	for k, v := range dirWatchers {
		v.Name = k
		merged[k] = v
	}

	return merged, nil
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

func loadMatchers(configDir string, base map[string]Matcher) (map[string]Matcher, error) {
	merged := make(map[string]Matcher)

	for k, v := range base {
		v.Name = k
		merged[k] = v
	}

	dirMatchers, readDirErr := readYAMLDir[Matcher](filepath.Join(configDir, matchersDir))
	if readDirErr != nil {
		return nil, readDirErr
	}

	for k, v := range dirMatchers {
		v.Name = k
		merged[k] = v
	}

	return merged, nil
}

func validateConfig(watchers map[string]Watcher, matchers map[string]Matcher) error {
	if len(watchers) == 0 {
		return errors.New("no watchers configured")
	}

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
