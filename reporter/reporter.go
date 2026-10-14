package reporter

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var _ Reporter = (*AgentReporter)(nil)

type AgentReporter struct {
	agentURL string
	client   *http.Client
}

type ReporterParams struct {
	IP          string
	Rule        string
	WatcherName string
	MatcherName string
	Line        string
}

type Reporter interface {
	Report(ctx context.Context, params ReporterParams) error
}

type Message struct {
	WatcherName string `json:"watcher_name"`
	MatcherName string `json:"matcher_name"`
	Line        string `json:"line"`
}

func New(agentURL string, timeout time.Duration) *AgentReporter {
	cleanURL := strings.TrimRight(agentURL, "/")

	client := &http.Client{
		Timeout: timeout,
	}

	requestBaseURL := cleanURL

	if socketPath, ok := strings.CutPrefix(cleanURL, "unix://"); ok {
		requestBaseURL = "http://localhost"

		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{}
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		}
	}

	return &AgentReporter{
		agentURL: requestBaseURL,
		client:   client,
	}
}

func (r *AgentReporter) Report(ctx context.Context, params ReporterParams) error {
	fullURL := r.buildReportURL(params)

	log.Debug().
		Str("ip", params.IP).
		Str("rule", params.Rule).
		Str("watcher", params.WatcherName).
		Str("matcher", params.MatcherName).
		Str("url", fullURL).
		Str("line", params.Line).
		Msg("sending report to agent")

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, nil)
	if reqErr != nil {
		return fmt.Errorf("failed to create report request: %w", reqErr)
	}

	resp, doErr := r.client.Do(req)
	if doErr != nil {
		return fmt.Errorf("failed to send report to agent: %w", doErr)
	}
	defer resp.Body.Close()

	return r.handleResponse(resp, params, fullURL)
}

func (r *AgentReporter) buildReportURL(params ReporterParams) string {
	reportPath := fmt.Sprintf(
		"/report/%s/%s",
		url.PathEscape(params.IP),
		url.PathEscape(params.Rule),
	)
	fullURL := r.agentURL + reportPath

	message := fmt.Sprintf(
		"%s - %s : %s",
		params.WatcherName,
		params.MatcherName,
		params.Line,
	)

	urlParams := url.Values{}
	urlParams.Set("message", message)
	fullURL += "?" + urlParams.Encode()

	return fullURL
}

func (r *AgentReporter) handleResponse(
	resp *http.Response,
	params ReporterParams,
	fullURL string,
) error {
	if resp.StatusCode == http.StatusOK {
		log.Info().
			Str("ip", params.IP).
			Str("rule", params.Rule).
			Str("watcher", params.WatcherName).
			Str("matcher", params.MatcherName).
			Msg("offender reported")

		return nil
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf(
			"agent returned unexpected status %d (read error: %w)",
			resp.StatusCode,
			readErr,
		)
	}

	log.Error().
		Str("url", fullURL).
		Str("line", params.Line).
		Msgf(
			"agent returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)

	return fmt.Errorf(
		"agent returned unexpected status %d: %s",
		resp.StatusCode,
		strings.TrimSpace(string(body)),
	)
}
