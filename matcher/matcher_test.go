package matcher_test

import (
	"testing"

	"gitlab.com/secban/secban-log-monitor/matcher"
)

func TestMatcherNewValid(t *testing.T) {
	t.Parallel()

	matcherInstance, err := matcher.New("nginx_404", "bad", []string{`404`, `403`}, nil)
	if err != nil {
		t.Fatalf("unexpected error creating matcher: %v", err)
	}

	if matcherInstance.Name() != "nginx_404" {
		t.Errorf("expected name nginx_404, got %s", matcherInstance.Name())
	}

	if matcherInstance.Rule() != "bad" {
		t.Errorf("expected rule bad, got %s", matcherInstance.Rule())
	}
}

func TestMatcherMatch(t *testing.T) {
	t.Parallel()

	matcherInstance, err := matcher.New(
		"sshd_auth",
		"evil",
		[]string{`Failed password`, `Invalid user`},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error creating matcher: %v", err)
	}

	lineMatch := "Oct 06 12:00:00 host sshd[123]: Failed password for root from 192.0.2.10 port 5555 ssh2"
	if !matcherInstance.Match(lineMatch) {
		t.Fatal("expected line to match")
	}

	lineNonMatch := "Oct 06 12:00:00 host sshd[123]: Accepted publickey for root from 192.0.2.10 port 5555 ssh2"
	if matcherInstance.Match(lineNonMatch) {
		t.Fatal("expected non-matching line to return false")
	}
}

func TestMatcherIgnore(t *testing.T) {
	t.Parallel()

	matcherInstance, newMatcherErr := matcher.New(
		"sshd_ignore",
		"evil",
		[]string{`Failed password`, `Invalid user`},
		[]string{`testuser`, `192\.0\.2\.99`},
	)
	if newMatcherErr != nil {
		t.Fatalf("unexpected error creating matcher: %v", newMatcherErr)
	}

	lineIgnoredUser := "Oct 06 12:00:00 host sshd[123]: Failed password for testuser from 192.0.2.10 port 5555 ssh2"
	if matcherInstance.Match(lineIgnoredUser) {
		t.Fatal("expected ignored user line to return false")
	}

	lineIgnoredIP := "Oct 06 12:00:00 host sshd[123]: Failed password for root from 192.0.2.99 port 5555 ssh2"
	if matcherInstance.Match(lineIgnoredIP) {
		t.Fatal("expected ignored ip line to return false")
	}

	lineValid := "Oct 06 12:00:00 host sshd[123]: Failed password for root from 192.0.2.10 port 5555 ssh2"
	if !matcherInstance.Match(lineValid) {
		t.Fatal("expected non-ignored line to match")
	}
}

func TestMatcherInvalidIgnoreRegex(t *testing.T) {
	t.Parallel()

	_, newMatcherErr := matcher.New("test", "bad", []string{`404`}, []string{`[invalid`})
	if newMatcherErr == nil {
		t.Fatal("expected error for invalid ignore_regex, got nil")
	}
}
