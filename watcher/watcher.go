package watcher

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/secban/secban-log-monitor/config"
	"gitlab.com/secban/secban-log-monitor/matcher"
)

const retryInterval = 1 * time.Second

var ErrNoIPMatch = errors.New("no ip found in line")

type HandlerParam struct {
	IP      string
	Rule    string
	Matcher string
	Line    string
}

type Handler func(param HandlerParam)

type Watcher struct {
	name        string
	command     string
	args        []string
	testArgs    []string
	env         []string
	ipRegex     *regexp.Regexp
	ipSubexpIdx int
	matchers    []*matcher.Matcher
}

type WatchRunner interface {
	Name() string
	Watch(ctx context.Context, handler Handler) error
}

func New(cfg config.Watcher, matchers []*matcher.Matcher) (*Watcher, error) {
	if cfg.Command == "" {
		return nil, errors.New("watcher command cannot be empty")
	}

	if cfg.IPRegex == "" {
		return nil, errors.New("watcher ip_regex cannot be empty")
	}

	compiledIPRegex, compileIPErr := regexp.Compile(cfg.IPRegex)
	if compileIPErr != nil {
		return nil, fmt.Errorf("failed to compile ip_regex %q: %w", cfg.IPRegex, compileIPErr)
	}

	ipIdx := -1

	for i, subexpName := range compiledIPRegex.SubexpNames() {
		if subexpName == "ip" {
			ipIdx = i
			break
		}
	}

	if ipIdx == -1 {
		if compiledIPRegex.NumSubexp() < 1 {
			return nil, errors.New("ip_regex must have at least one capture group for ip")
		}

		ipIdx = 1
	}

	watcherInstance := &Watcher{
		name:        cfg.Name,
		command:     cfg.Command,
		args:        cfg.Args,
		testArgs:    cfg.TestArgs,
		env:         cfg.Env,
		ipRegex:     compiledIPRegex,
		ipSubexpIdx: ipIdx,
		matchers:    matchers,
	}

	return watcherInstance, nil
}

func (w *Watcher) Name() string {
	return w.name
}

func (w *Watcher) Matchers() []*matcher.Matcher {
	return w.matchers
}

func (w *Watcher) ExtractIP(line string) (string, error) {
	submatches := w.ipRegex.FindStringSubmatch(line)
	if len(submatches) <= w.ipSubexpIdx {
		return "", ErrNoIPMatch
	}

	ip := submatches[w.ipSubexpIdx]
	if ip == "" {
		return "", ErrNoIPMatch
	}

	return ip, nil
}

func (w *Watcher) ProcessLine(line string) (string, string, string, bool, error) {
	for _, matcherInstance := range w.matchers {
		if !matcherInstance.Match(line) {
			continue
		}

		ip, extractErr := w.ExtractIP(line)
		if extractErr != nil {
			return "", matcherInstance.Rule(), matcherInstance.Name(), true, extractErr
		}

		return ip, matcherInstance.Rule(), matcherInstance.Name(), true, nil
	}

	return "", "", "", false, nil
}

func (w *Watcher) ProcessStream(reader io.Reader, handler Handler) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		ip, rule, matcher, matched, processErr := w.ProcessLine(line)
		if !matched {
			continue
		}

		if processErr != nil {
			log.Warn().
				Str("watcher", w.name).
				Str("line", line).
				Err(processErr).
				Msg("matched line but failed to extract ip")

			continue
		}

		handler(HandlerParam{
			IP:      ip,
			Rule:    rule,
			Matcher: matcher,
			Line:    line,
		})
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		return scanErr
	}

	return nil
}

func (w *Watcher) RunCommand(ctx context.Context, args []string, handler Handler) error {
	cmd := exec.CommandContext(ctx, w.command, args...) //nolint: gosec
	if len(w.env) > 0 {
		cmd.Env = append(os.Environ(), w.env...)
	}

	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", pipeErr)
	}

	startErr := cmd.Start()
	if startErr != nil {
		return fmt.Errorf("failed to start command %q: %w", w.command, startErr)
	}

	processErr := w.ProcessStream(stdout, handler)
	if processErr != nil && !errors.Is(processErr, io.EOF) && !errors.Is(processErr, os.ErrClosed) {
		log.Warn().
			Str("watcher", w.name).
			Err(processErr).
			Msg("stream reading ended with error")
	}

	waitErr := cmd.Wait()
	if waitErr != nil && ctx.Err() == nil {
		return waitErr
	}

	return nil
}

func (w *Watcher) Check(ctx context.Context, handler Handler) error {
	args := w.testArgs
	if len(args) == 0 {
		args = w.args
	}

	log.Info().
		Str("watcher", w.name).
		Str("command", w.command).
		Strs("test_args", args).
		Int("matchers", len(w.matchers)).
		Msg("running watcher in check/test mode")

	runErr := w.RunCommand(ctx, args, handler)
	if runErr != nil {
		return runErr
	}

	return nil
}

func (w *Watcher) Watch(ctx context.Context, handler Handler) error {
	log.Info().
		Str("watcher", w.name).
		Str("command", w.command).
		Strs("args", w.args).
		Int("matchers", len(w.matchers)).
		Msg("starting watcher stream")

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		runErr := w.RunCommand(ctx, w.args, handler)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if runErr != nil {
			log.Warn().
				Str("watcher", w.name).
				Err(runErr).
				Msg("watcher command exited with error, restarting")
		} else {
			log.Info().
				Str("watcher", w.name).
				Msg("watcher command terminated cleanly, restarting")
		}

		sleepErr := sleepWithContext(ctx, retryInterval)
		if sleepErr != nil {
			return sleepErr
		}
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
