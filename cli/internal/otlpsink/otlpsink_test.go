package otlpsink

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// str, num and dbl make attributes of each value kind.
func str(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func num(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

func dbl(k string, v float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}}
}

// logsRequest encodes records as the collector's export request, which is
// what the harnesses send.
func logsRequest(t *testing.T, records ...*logspb.LogRecord) []byte {
	t.Helper()
	b, err := proto.Marshal(&collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: records}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// metricsRequest encodes metrics as the collector's export request.
func metricsRequest(t *testing.T, metrics ...*metricspb.Metric) []byte {
	t.Helper()
	b, err := proto.Marshal(&colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// sum makes a sum metric with one data point per attribute set.
func sum(name string, sets ...[]*commonpb.KeyValue) *metricspb.Metric {
	points := make([]*metricspb.NumberDataPoint, 0, len(sets))
	for _, attrs := range sets {
		points = append(points, &metricspb.NumberDataPoint{Attributes: attrs, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1}})
	}
	return &metricspb.Metric{Name: name, Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{DataPoints: points}}}
}

// post sends body to the sink at path and returns the status and the
// response body.
func post(t *testing.T, srv *httptest.Server, path, contentType, encoding string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	if encoding != "" {
		req.Header.Set("Content-Encoding", encoding)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Error(cerr)
		}
	}()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, out
}

// export posts body and fails unless the sink answers 200 with an empty
// protobuf response.
func export(t *testing.T, srv *httptest.Server, path string, body []byte) {
	t.Helper()
	status, out := post(t, srv, path, "application/x-protobuf", "", body)
	if status != http.StatusOK || len(out) != 0 {
		t.Fatalf("POST %s: got %d %q; want 200 and an empty response", path, status, out)
	}
}

// TestClaudeCode checks what the sink keeps of a Claude Code run: tokens,
// cost, requests and retries from its api_request events, the skills and
// plugins on its events and metrics, the first request of the main loop
// by event.sequence, and nothing else, such as user.email (criteria 24
// and 25).
func TestClaudeCode(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	sink, ok := srv.Config.Handler.(*Sink)
	if !ok {
		t.Fatal("handler is not a Sink")
	}

	export(t, srv, "/run-1/v1/logs", logsRequest(t,
		// A request of another loop comes first and is not the first
		// request of the main loop.
		&logspb.LogRecord{
			Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "claude_code.api_request"}},
			Attributes: []*commonpb.KeyValue{
				str("event.name", "api_request"), num("event.sequence", 1), str("query_source", "compact"),
				str("input_tokens", "5"), str("output_tokens", "1"), str("cache_read_tokens", "0"),
				str("cache_creation_tokens", "0"), str("cost_usd", "0.25"),
				str("user.email", "someone@example.com"), str("session.id", "abc"), str("prompt", "secret"),
			},
		},
		&logspb.LogRecord{
			EventName: "claude_code.api_request",
			Attributes: []*commonpb.KeyValue{
				num("event.sequence", 4), str("query_source", "sdk"),
				num("input_tokens", 100), num("output_tokens", 20), num("cache_read_tokens", 300),
				num("cache_creation_tokens", 7), dbl("cost_usd", 0.5),
			},
		},
		&logspb.LogRecord{
			EventName: "claude_code.api_request",
			Attributes: []*commonpb.KeyValue{
				num("event.sequence", 3), str("query_source", "sdk"), num("attempt", 2),
				num("input_tokens", 10), num("output_tokens", 2), num("cache_read_tokens", 1000),
				num("cache_creation_tokens", 30),
			},
		},
		&logspb.LogRecord{
			EventName:  "claude_code.api_error",
			Attributes: []*commonpb.KeyValue{num("attempt", 3), str("user.account_uuid", "u-1")},
		},
		&logspb.LogRecord{
			EventName:  "claude_code.skill_activated",
			Attributes: []*commonpb.KeyValue{str("skill.name", "acme-review"), str("organization.id", "org-1")},
		},
		&logspb.LogRecord{
			EventName:  "claude_code.plugin_loaded",
			Attributes: []*commonpb.KeyValue{str("plugin.name", "acme-tools"), str("plugin.version", "1.2.0")},
		},
	))

	// The metrics come gzipped, as an exporter may send them.
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(metricsRequest(t, sum("claude_code.token.usage",
		[]*commonpb.KeyValue{str("skill.name", "acme-plan"), str("plugin.name", "acme-tools"), str("user.email", "someone@example.com")},
		[]*commonpb.KeyValue{str("plugin.name", "other")},
	))); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if status, out := post(t, srv, "/run-1/v1/metrics", "application/x-protobuf", "gzip", gz.Bytes()); status != http.StatusOK || len(out) != 0 {
		t.Fatalf("gzipped metrics: got %d %q; want 200 and an empty response", status, out)
	}

	got, err := sink.Run("run-1")
	if err != nil {
		t.Fatal(err)
	}
	want := Telemetry{
		InputTokens:         115,
		OutputTokens:        23,
		CacheReadTokens:     1300,
		CacheCreationTokens: 37,
		CostUSD:             0.75,
		CostReported:        true,
		Requests:            3,
		MaxAttempt:          3,
		Skills:              []string{"acme-plan", "acme-review"},
		Plugins:             []Plugin{{Name: "acme-tools"}, {Name: "acme-tools", Version: "1.2.0"}, {Name: "other"}},
		FirstInput:          1040,
		FirstInputSeen:      true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Run:\n got %+v\nwant %+v", got, want)
	}
}

// TestCodex checks what the sink keeps of a Codex run: cached input split
// out of the input count, requests from completed responses, skills from
// the skill.injected metric, and the first completed response by time
// from the user prompt on as the first request (criteria 24 and 25). The
// records carry an event name of the place in Codex's source, as Codex
// 0.171.0 sends.
func TestCodex(t *testing.T) {
	sink := New()
	srv := httptest.NewServer(sink)
	defer srv.Close()

	completed := func(ts uint64, in, cached, out int64) *logspb.LogRecord {
		return &logspb.LogRecord{
			TimeUnixNano: ts, EventName: "event otel/src/events/session_telemetry.rs:1187",
			Attributes: []*commonpb.KeyValue{
				str("event.name", "codex.sse_event"), str("event.kind", "response.completed"),
				num("input_token_count", in), num("cached_token_count", cached), num("output_token_count", out),
				str("user.email", "someone@example.com"),
			},
		}
	}
	export(t, srv, "/codex-1/v1/logs", logsRequest(t,
		// Codex warms its connection with a request before the prompt.
		completed(50, 2, 0, 0),
		completed(200, 900, 800, 40),
		&logspb.LogRecord{Attributes: []*commonpb.KeyValue{str("event.name", "codex.api_request"), num("attempt", 1)}},
		&logspb.LogRecord{Attributes: []*commonpb.KeyValue{str("event.name", "codex.api_request"), num("attempt", 2)}},
		// Another kind of event carries no tokens to keep.
		&logspb.LogRecord{Attributes: []*commonpb.KeyValue{
			str("event.name", "codex.sse_event"), str("event.kind", "response.output_text.delta"), num("input_token_count", 9999),
		}},
	))
	// The earlier response, and the prompt, arrive in a later export.
	export(t, srv, "/codex-1/v1/logs", logsRequest(t,
		completed(100, 600, 0, 10),
		&logspb.LogRecord{TimeUnixNano: 90, EventName: "event otel/src/events/session_telemetry.rs:1234", Attributes: []*commonpb.KeyValue{str("event.name", "codex.user_prompt")}},
	))
	export(t, srv, "/codex-1/v1/metrics", metricsRequest(t,
		sum("codex.skill.injected", []*commonpb.KeyValue{str("skill", "acme-review"), str("status", "ok")}),
	))

	got, err := sink.Run("codex-1")
	if err != nil {
		t.Fatal(err)
	}
	want := Telemetry{
		InputTokens:     702,
		OutputTokens:    50,
		CacheReadTokens: 800,
		Requests:        3,
		MaxAttempt:      2,
		Skills:          []string{"acme-review"},
		Plugins:         []Plugin{},
		FirstInput:      600,
		FirstInputSeen:  true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Run:\n got %+v\nwant %+v", got, want)
	}
}

// TestRunsApart checks that each run id keeps its own telemetry, and that
// a run that exported nothing reports ErrNoTelemetry (criterion 24).
func TestRunsApart(t *testing.T) {
	sink := New()
	srv := httptest.NewServer(sink)
	defer srv.Close()

	request := func(in int64) []byte {
		return logsRequest(t, &logspb.LogRecord{
			EventName:  "claude_code.api_request",
			Attributes: []*commonpb.KeyValue{num("input_tokens", in)},
		})
	}
	export(t, srv, "/a/v1/logs", request(1))
	export(t, srv, "/b/v1/logs", request(2))
	export(t, srv, "/a/v1/logs", request(4))

	for id, want := range map[string]int64{"a": 5, "b": 2} {
		got, err := sink.Run(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.InputTokens != want || got.FirstInputSeen {
			t.Errorf("run %s: got %d input tokens, first seen %v; want %d, not seen", id, got.InputTokens, got.FirstInputSeen, want)
		}
	}
	if _, err := sink.Run("c"); !errors.Is(err, ErrNoTelemetry) {
		t.Errorf("run c: got %v; want ErrNoTelemetry", err)
	}
}

// TestRejected checks the status for each kind of export the sink does
// not keep, and that a rejected export makes its run's telemetry an
// error (criterion 24).
func TestRejected(t *testing.T) {
	sink := New()
	srv := httptest.NewServer(sink)
	defer srv.Close()

	valid := logsRequest(t)
	for _, tc := range []struct {
		name, method, path, contentType, encoding string
		body                                      []byte
		want                                      int
	}{
		{"traces", http.MethodPost, "/r/v1/traces", "application/x-protobuf", "", valid, http.StatusNotFound},
		{"no run id", http.MethodPost, "/v1/logs", "application/x-protobuf", "", valid, http.StatusNotFound},
		{"run id with a dot first", http.MethodPost, "/../v1/logs", "application/x-protobuf", "", valid, http.StatusNotFound},
		{"extra segment", http.MethodPost, "/r/x/v1/logs", "application/x-protobuf", "", valid, http.StatusNotFound},
		{"get", http.MethodGet, "/r/v1/logs", "application/x-protobuf", "", nil, http.StatusMethodNotAllowed},
		{"json", http.MethodPost, "/r/v1/logs", "application/json", "", []byte("{}"), http.StatusUnsupportedMediaType},
		{"no content type", http.MethodPost, "/r/v1/metrics", "", "", valid, http.StatusUnsupportedMediaType},
		{"zstd", http.MethodPost, "/r/v1/logs", "application/x-protobuf", "zstd", valid, http.StatusUnsupportedMediaType},
		{"bad gzip", http.MethodPost, "/r/v1/logs", "application/x-protobuf", "gzip", []byte("not gzip"), http.StatusBadRequest},
		{"too large", http.MethodPost, "/r/v1/logs", "application/x-protobuf", "", make([]byte, MaxBody+1), http.StatusRequestEntityTooLarge},
		{"not protobuf", http.MethodPost, "/r/v1/metrics", "application/x-protobuf", "", []byte{0xff, 0xff}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, bytes.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.encoding != "" {
				req.Header.Set("Content-Encoding", tc.encoding)
			}
			rec := httptest.NewRecorder()
			sink.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("got %d; want %d", rec.Code, tc.want)
			}
		})
	}

	// The rejected exports for run r make its telemetry an error even
	// after a good export, while the paths that name no run add none.
	export(t, srv, "/r/v1/logs", valid)
	if _, err := sink.Run("r"); err == nil || errors.Is(err, ErrNoTelemetry) || !strings.Contains(err.Error(), "6 of its exports were rejected") {
		t.Errorf("run r: got %v; want 6 rejected exports", err)
	}
}

// TestCodexCapture replays the exports of one real Codex 0.171.0 run in
// the eval image, with the login's email and account id replaced, so the
// sink is checked against what Codex sends and not only against records
// built by hand: event names in the event.name attribute, a warm-up
// request before the prompt, and one completed response per request
// (criteria 24 and 32, the harness data file's entry
// codex-run-telemetry).
func TestCodexCapture(t *testing.T) {
	sink := New()
	srv := httptest.NewServer(sink)
	defer srv.Close()
	files, err := filepath.Glob(filepath.Join("testdata", "codex-0.171.0", "*.pb"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no capture: %v", err)
	}
	for _, f := range files {
		body, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatal(rerr)
		}
		export(t, srv, "/codex-1/v1/logs", body)
	}
	got, err := sink.Run("codex-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests != 2 || !got.FirstInputSeen || got.FirstInput < 1000 || got.InputTokens+got.CacheReadTokens < got.FirstInput {
		t.Errorf("telemetry %+v; want 2 requests, and the first after the prompt, not the warm-up of a few tokens", got)
	}
}

// TestClaudeCodeCapture replays the logs and metrics of one real Claude
// Code 2.1.330 run in the eval image, with its ids, prompt and reply
// replaced, so the sink is checked against what Claude Code sends: one
// API request, whose three input counts make the first request's whole
// input, a cost, and the plugins loaded with the run's plugin's version
// (criteria 24 and 32, the harness data file's entry
// claude-code-run-telemetry).
func TestClaudeCodeCapture(t *testing.T) {
	sink := New()
	srv := httptest.NewServer(sink)
	defer srv.Close()
	files, err := filepath.Glob(filepath.Join("testdata", "claude-code-2.1.330", "*.pb"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no capture: %v", err)
	}
	for _, f := range files {
		body, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatal(rerr)
		}
		signal := "logs"
		if strings.Contains(f, "metrics") {
			signal = "metrics"
		}
		export(t, srv, "/claude-1/v1/"+signal, body)
	}
	got, err := sink.Run("claude-1")
	if err != nil {
		t.Fatal(err)
	}
	whole := got.InputTokens + got.CacheReadTokens + got.CacheCreationTokens
	if got.Requests != 1 || !got.FirstInputSeen || got.FirstInput != whole || whole < 1000 || !got.CostReported || got.CostUSD <= 0 {
		t.Errorf("telemetry %+v; want one request whose input counts make the first input, and a cost", got)
	}
	if !slices.Contains(got.Plugins, Plugin{Name: "cech-example", Version: "0.0.0-dev"}) {
		t.Errorf("plugins %v; want cech-example 0.0.0-dev", got.Plugins)
	}
}
