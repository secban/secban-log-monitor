package reporter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.com/thequietbot/secban/secban-log-monitor/reporter"
)

func TestReporterSuccess(t *testing.T) {
	t.Parallel()

	var (
		receivedPath    string
		receivedMessage string
	)

	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			receivedPath = request.URL.Path
			receivedMessage = request.URL.Query().Get("message")

			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"message":"report received"}`))
		}),
	)
	defer server.Close()

	rep := reporter.New(server.URL, 1*time.Second)

	reportErr := rep.Report(context.Background(), reporter.ReporterParams{
		IP:   "192.0.2.100",
		Rule: "bad",
		Line: "test alert message",
	})
	if reportErr != nil {
		t.Fatalf("unexpected report error: %v", reportErr)
	}

	if receivedPath != "/report/192.0.2.100/bad" {
		t.Errorf("expected path /report/192.0.2.100/bad, got %s", receivedPath)
	}

	if !strings.Contains(receivedMessage, "test alert message") {
		t.Errorf("expected message 'test alert message', got %s", receivedMessage)
	}
}

func TestReporterErrorStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, "invalid ip address", http.StatusBadRequest)
		}),
	)
	defer server.Close()

	rep := reporter.New(server.URL, 1*time.Second)

	reportErr := rep.Report(context.Background(), reporter.ReporterParams{
		IP:   "invalid-ip",
		Rule: "bad",
		Line: "",
	})
	if reportErr == nil {
		t.Fatal("expected error on 400 bad request, got nil")
	}
}
