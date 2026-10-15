package functest_test

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-log-monitor/matcher"
	"gopkg.in/yaml.v2"
)

//go:embed data
var testdata embed.FS

//go:embed testmatchers
var testmatchers embed.FS

type testExpects struct {
	Ignored int `yaml:"ignored"`
	Lines   int `yaml:"lines"`
	Matches int `yaml:"matches"`
}

type testMatcher struct {
	Data          string      `yaml:"data"`
	Expects       testExpects `yaml:"expects"`
	IgnoreRegexes []string    `yaml:"ignore_regexes"`
	IPRegex       string      `yaml:"ip_regex"`
	MatchRegexes  []string    `yaml:"match_regexes"`
	Name          string      `yaml:"name"`
}

func TestMatcher(t *testing.T) {
	t.Parallel()

	for _, targetMatcher := range getTargetMatchers(t) {
		t.Run(targetMatcher, func(t *testing.T) {
			t.Parallel()

			matcherConfig := loadMatcherConfig(t, targetMatcher)

			matcherInstance, matcherErr := matcher.New(
				matcherConfig.Name,
				"testrule",
				matcherConfig.MatchRegexes,
				matcherConfig.IgnoreRegexes,
			)
			if matcherErr != nil {
				t.Fatalf("failed to create matcher %s: %v", matcherConfig.Name, matcherErr)
			}

			data := loadTestData(t, matcherConfig.Data)
			matches := 0
			ignored := 0
			lines := 0

			for _, line := range splitLines(data) {
				matched := matcherInstance.Match(line)
				if !matched {
					log.Info().Bool("matched", matched).Str("line", line).Msg("")
				}

				lines++

				if matched {
					matches++
				} else {
					ignored++
				}
			}

			if lines != matcherConfig.Expects.Lines {
				t.Errorf("expected %d lines, got %d", matcherConfig.Expects.Lines, lines)
			}

			if matches != matcherConfig.Expects.Matches {
				t.Errorf("expected %d matches, got %d", matcherConfig.Expects.Matches, matches)
			}

			if ignored != matcherConfig.Expects.Ignored {
				t.Errorf("expected %d ignored, got %d", matcherConfig.Expects.Ignored, ignored)
			}

			if matches+ignored != matcherConfig.Expects.Lines {
				t.Errorf(
					"expected total of %d lines to be processed, got %d",
					matcherConfig.Expects.Lines,
					matches+ignored,
				)
			}
		})
	}
}

func loadTestData(t *testing.T, fileName string) []byte {
	t.Helper()

	data, readErr := testdata.ReadFile("data/" + fileName)
	if readErr != nil {
		t.Fatalf("failed reading test data %s: %v", fileName, readErr)
	}

	return data
}

func getTargetMatchers(t *testing.T) []string {
	t.Helper()

	entries, readDirErr := testmatchers.ReadDir("testmatchers")
	if readDirErr != nil {
		t.Fatalf("failed reading testmatchers directory: %v", readDirErr)
	}

	var matchers []string

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		matchers = append(matchers, strings.TrimSuffix(entry.Name(), ".yaml"))
	}

	return matchers
}

func loadMatcherConfig(t *testing.T, targetMatcher string) testMatcher {
	t.Helper()

	data, readErr := testmatchers.ReadFile("testmatchers/" + targetMatcher + ".yaml")
	if readErr != nil {
		t.Fatalf("failed reading matcher config for %s: %v", targetMatcher, readErr)
	}

	var parsed testMatcher

	unmarshalErr := yaml.Unmarshal(data, &parsed)
	if unmarshalErr != nil {
		t.Fatalf("failed unmarshaling matcher config for %s: %v", targetMatcher, unmarshalErr)
	}

	return parsed
}

func splitLines(data []byte) []string {
	var lines []string

	var currentLine []byte

	for _, b := range data {
		if b == '\n' {
			lines = append(lines, string(currentLine))
			currentLine = []byte{}
		} else {
			currentLine = append(currentLine, b)
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, string(currentLine))
	}

	return lines
}
