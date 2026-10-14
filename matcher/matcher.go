package matcher

import (
	"errors"
	"fmt"
	"regexp"
)

type Matcher struct {
	name          string
	rule          string
	matchRegexes  []*regexp.Regexp
	ignoreRegexes []*regexp.Regexp
}

func New(
	name string,
	rule string,
	matchRegexes []string,
	ignoreRegexes []string,
) (*Matcher, error) {
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	if rule == "" {
		return nil, errors.New("rule cannot be empty")
	}

	if len(matchRegexes) == 0 {
		return nil, errors.New("match_regexes cannot be empty")
	}

	compiledMatchRegexes := make([]*regexp.Regexp, 0, len(matchRegexes))

	for idx, pattern := range matchRegexes {
		compiled, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			return nil, fmt.Errorf(
				"failed to compile match_regexes[%d] %q: %w",
				idx,
				pattern,
				compileErr,
			)
		}

		compiledMatchRegexes = append(compiledMatchRegexes, compiled)
	}

	compiledIgnoreRegexes := make([]*regexp.Regexp, 0, len(ignoreRegexes))

	for idx, pattern := range ignoreRegexes {
		compiled, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			return nil, fmt.Errorf(
				"failed to compile ignore_regexes[%d] %q: %w",
				idx,
				pattern,
				compileErr,
			)
		}

		compiledIgnoreRegexes = append(compiledIgnoreRegexes, compiled)
	}

	return &Matcher{
		name:          name,
		rule:          rule,
		matchRegexes:  compiledMatchRegexes,
		ignoreRegexes: compiledIgnoreRegexes,
	}, nil
}

func (m *Matcher) Name() string {
	return m.name
}

func (m *Matcher) Rule() string {
	return m.rule
}

func (m *Matcher) Match(line string) bool {
	matched := false

	for _, r := range m.matchRegexes {
		if r.MatchString(line) {
			matched = true

			break
		}
	}

	if !matched {
		return false
	}

	for _, r := range m.ignoreRegexes {
		if r.MatchString(line) {
			return false
		}
	}

	return true
}
