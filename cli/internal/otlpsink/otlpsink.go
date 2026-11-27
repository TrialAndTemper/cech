// Package otlpsink receives the OpenTelemetry logs and metrics that the
// harnesses export during an eval run, over OTLP/HTTP with protobuf
// encoding, and keeps only the fields the run results need (eval-runs
// spec criteria 24 and 25). `cech eval` serves one Sink on the host for a
// batch, and each model run exports to its own path:
//
//	/<run-id>/v1/logs
//	/<run-id>/v1/metrics
//
// Claude Code appends those signal paths to OTEL_EXPORTER_OTLP_ENDPOINT,
// and Codex takes each signal's full URL.
package otlpsink

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// MaxBody is the largest export body, before or after gzip, that the
// sink reads; a larger one is answered 413.
const MaxBody = 8 << 20

// protobufType is the only content type the sink accepts: both harnesses
// send protobuf (criterion 24).
const protobufType = "application/x-protobuf"

// The names below are what the sink reads from each harness's telemetry,
// as the harness data file's entries claude-code-run-telemetry and
// codex-run-telemetry record them (criterion 32).
const (
	// claudeAPIRequest is Claude Code's log event for one model request.
	// Its name may come as the record's event name, its event.name
	// attribute or its body, with or without the claude_code. prefix.
	claudeAPIRequest = "claude_code.api_request"
	// claudeInput, claudeOutput, claudeCacheRead and claudeCacheCreation
	// are the token counts on claudeAPIRequest; their sum is the whole
	// input of the request.
	claudeInput         = "input_tokens"
	claudeOutput        = "output_tokens"
	claudeCacheRead     = "cache_read_tokens"
	claudeCacheCreation = "cache_creation_tokens"
	// claudeCost is the cost in USD on claudeAPIRequest.
	claudeCost = "cost_usd"
	// claudeSequence orders Claude Code's events within a session.
	claudeSequence = "event.sequence"
	// claudeQuerySource names the loop a request belongs to, and
	// claudeMainLoop is its value for the main agent loop under -p.
	claudeQuerySource = "query_source"
	claudeMainLoop    = "sdk"

	// codexUserPrompt is Codex's log event for the prompt of a turn.
	codexUserPrompt = "codex.user_prompt"
	// codexSSEEvent is Codex's log event for one server-sent event, and
	// codexKind with the value codexCompleted marks the one that ends a
	// model request and carries its token counts. Codex 0.171.0 sends
	// its requests over a websocket, so its codex.api_request events
	// count only its model list lookups, not model requests.
	codexSSEEvent  = "codex.sse_event"
	codexKind      = "event.kind"
	codexCompleted = "response.completed"
	// codexInput is a request's whole input, codexCached the part of it
	// read from the cache, and codexOutput its output.
	codexInput  = "input_token_count"
	codexCached = "cached_token_count"
	codexOutput = "output_token_count"
	// codexSkillInjected is Codex's metric of the skills added to a run,
	// and codexSkill the attribute that names the skill.
	codexSkillInjected = "codex.skill.injected"
	codexSkill         = "skill"

	// attempt is the retry attempt of a request, on both harnesses.
	attempt = "attempt"
	// eventName is the attribute that names a log event when the record
	// leaves its own event name empty.
	eventName = "event.name"
	// skillName, pluginName and pluginVersion name what a run loaded, on
	// Claude Code's events and metrics.
	skillName     = "skill.name"
	pluginName    = "plugin.name"
	pluginVersion = "plugin.version"
)

// runID is the form of a run's path segment.
var runID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ErrNoTelemetry is the error of Sink.Run for a run that exported
// nothing, which the runner counts as a run that errors (criterion 24).
var ErrNoTelemetry = errors.New("no telemetry received")

// Plugin is a plugin a run loaded, with its version where the harness
// gave one.
type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Telemetry is what the sink kept of one run's exports; it holds no other
// field (criterion 24).
type Telemetry struct {
	// InputTokens is the input that no cache served, summed over the
	// run's requests; with CacheReadTokens and CacheCreationTokens it
	// makes the whole input. Codex counts cached input inside its input
	// count, so for Codex it is that count less the cached one.
	InputTokens int64
	// OutputTokens is the output summed over the run's requests.
	OutputTokens int64
	// CacheReadTokens is the input read from the cache.
	CacheReadTokens int64
	// CacheCreationTokens is the input written to the cache; only Claude
	// Code reports it.
	CacheCreationTokens int64
	// CostUSD is the cost in USD, valid when CostReported is set; only
	// Claude Code reports it.
	CostUSD      float64
	CostReported bool
	// Requests is the number of model requests, counting retries.
	Requests int
	// MaxAttempt is the highest retry attempt seen, 0 when none was.
	MaxAttempt int64
	// Skills are the names of the skills seen, sorted.
	Skills []string
	// Plugins are the plugins seen, sorted by name and version.
	Plugins []Plugin
	// FirstInput is the whole input of the first request of the main
	// agent loop, valid when FirstInputSeen is set (criterion 25).
	FirstInput     int64
	FirstInputSeen bool
}

// first is the candidate for a run's first request: the lowest key wins,
// and among equal keys the one that arrived first. For Codex the key is
// a time, and only requests from the user prompt on count, since Codex
// warms its connection with a request of its own before it.
type first struct {
	key   int64
	input int64
	set   bool
	// from is the time of the user prompt, valid when fromSet is set;
	// held is every Codex request seen, kept until the prompt's time is
	// known, which may arrive in a later export.
	from    int64
	fromSet bool
	held    []first
}

// offer keeps input when key is lower than the one kept.
func (f *first) offer(key, input int64) {
	if !f.set || key < f.key {
		f.key, f.input, f.set = key, input, true
	}
}

// offerFromPrompt holds a Codex request at time key, which counts once
// the prompt's time is known and key is not before it.
func (f *first) offerFromPrompt(key, input int64) {
	f.held = append(f.held, first{key: key, input: input})
}

// prompt records the time of the user prompt, keeping the earliest.
func (f *first) prompt(ts int64) {
	if !f.fromSet || ts < f.from {
		f.from, f.fromSet = ts, true
	}
}

// result returns the first request's input, if one counts.
func (f *first) result() (int64, bool) {
	best := first{key: f.key, input: f.input, set: f.set}
	if f.fromSet {
		for _, h := range f.held {
			if h.key >= f.from && (!best.set || h.key < best.key) {
				best = first{key: h.key, input: h.input, set: true}
			}
		}
	}
	return best.input, best.set
}

// run is what the sink holds for one run id.
type run struct {
	t        Telemetry
	skills   map[string]bool
	plugins  map[Plugin]bool
	first    first
	received int
	rejected int
}

// Sink is an http.Handler that receives OTLP/HTTP exports for many runs
// at once and keeps each run's telemetry apart. Its zero value is not
// ready; use New.
type Sink struct {
	mu   sync.Mutex
	runs map[string]*run
}

// New returns an empty Sink.
func New() *Sink {
	return &Sink{runs: map[string]*run{}}
}

// Run returns what the sink kept for the run id. The error is
// ErrNoTelemetry when the run exported nothing, and an error that names
// the count when the sink rejected any of the run's exports, since its
// telemetry is then incomplete.
func (s *Sink) Run(id string) (Telemetry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runs[id]
	if r == nil || (r.received == 0 && r.rejected == 0) {
		return Telemetry{}, fmt.Errorf("run %q: %w", id, ErrNoTelemetry)
	}
	t := r.t
	t.Skills = make([]string, 0, len(r.skills))
	for name := range r.skills {
		t.Skills = append(t.Skills, name)
	}
	slices.Sort(t.Skills)
	t.Plugins = make([]Plugin, 0, len(r.plugins))
	for p := range r.plugins {
		t.Plugins = append(t.Plugins, p)
	}
	slices.SortFunc(t.Plugins, func(a, b Plugin) int {
		return strings.Compare(a.Name+"\x00"+a.Version, b.Name+"\x00"+b.Version)
	})
	t.FirstInput, t.FirstInputSeen = r.first.result()
	if r.rejected > 0 {
		return t, fmt.Errorf("run %q: %d of its exports were rejected", id, r.rejected)
	}
	return t, nil
}

// ServeHTTP receives one export: 200 with an empty response message once
// it is kept, 404 for a path that is not a run's logs or metrics, 405 for
// a method other than POST, 415 for a body that is not protobuf, 413 for
// one over MaxBody, and 400 for one that does not decode.
func (s *Sink) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	id, signal, ok := parsePath(req.URL.Path)
	if !ok {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := s.receive(req, id, signal)
	if err != nil {
		s.reject(id)
		http.Error(w, err.Error(), status)
		return
	}
	// An empty Export*ServiceResponse encodes as no bytes: full success.
	w.Header().Set("Content-Type", protobufType)
	w.WriteHeader(http.StatusOK)
}

// parsePath splits /<run-id>/v1/<signal> and reports whether it names a
// run's logs or metrics.
func parsePath(path string) (id, signal string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[1] != "v1" || !runID.MatchString(parts[0]) {
		return "", "", false
	}
	if parts[2] != "logs" && parts[2] != "metrics" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

// receive reads and decodes one export and keeps its fields, returning
// the status to answer with when it fails.
func (s *Sink) receive(req *http.Request, id, signal string) (int, error) {
	media, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || media != protobufType {
		return http.StatusUnsupportedMediaType, fmt.Errorf("content type must be %s", protobufType)
	}
	body, status, err := readBody(req)
	if err != nil {
		return status, err
	}
	// LogsData and MetricsData are the same message on the wire as
	// ExportLogsServiceRequest and ExportMetricsServiceRequest, without
	// the collector packages' gRPC code.
	switch signal {
	case "logs":
		var data logspb.LogsData
		if err := proto.Unmarshal(body, &data); err != nil {
			return http.StatusBadRequest, fmt.Errorf("decoding logs: %w", err)
		}
		s.keep(id, func(r *run) { r.addLogs(&data) })
	default:
		var data metricspb.MetricsData
		if err := proto.Unmarshal(body, &data); err != nil {
			return http.StatusBadRequest, fmt.Errorf("decoding metrics: %w", err)
		}
		s.keep(id, func(r *run) { r.addMetrics(&data) })
	}
	return http.StatusOK, nil
}

// readBody reads the request body, gunzipping it when it says so, up to
// MaxBody bytes each way.
func readBody(req *http.Request) ([]byte, int, error) {
	body, err := io.ReadAll(io.LimitReader(req.Body, MaxBody+1))
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("reading body: %w", err)
	}
	if len(body) > MaxBody {
		return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("body over %d bytes", MaxBody)
	}
	switch req.Header.Get("Content-Encoding") {
	case "", "identity":
		return body, http.StatusOK, nil
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("reading gzip body: %w", err)
		}
		body, err = io.ReadAll(io.LimitReader(zr, MaxBody+1))
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("reading gzip body: %w", err)
		}
		if len(body) > MaxBody {
			return nil, http.StatusRequestEntityTooLarge, fmt.Errorf("body over %d bytes", MaxBody)
		}
		return body, http.StatusOK, nil
	default:
		return nil, http.StatusUnsupportedMediaType, errors.New("content encoding must be gzip or none")
	}
}

// keep applies add to the run id under the lock.
func (s *Sink) keep(id string, add func(*run)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.get(id)
	add(r)
	r.received++
}

// reject counts a rejected export for the run id.
func (s *Sink) reject(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.get(id).rejected++
}

// get returns the run id, adding it; the caller holds the lock.
func (s *Sink) get(id string) *run {
	r := s.runs[id]
	if r == nil {
		r = &run{skills: map[string]bool{}, plugins: map[Plugin]bool{}}
		s.runs[id] = r
	}
	return r
}

// addLogs keeps the fields of each log record in data.
func (r *run) addLogs(data *logspb.LogsData) {
	for _, rl := range data.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			for _, rec := range sl.GetLogRecords() {
				r.addRecord(rec)
			}
		}
	}
}

// addRecord keeps the fields of one log record.
func (r *run) addRecord(rec *logspb.LogRecord) {
	attrs := attrMap(rec.GetAttributes())
	r.addNames(attrs)
	if n, ok := intAttr(attrs, attempt); ok && n > r.t.MaxAttempt {
		r.t.MaxAttempt = n
	}
	switch name := recordName(rec, attrs); name {
	case claudeAPIRequest, strings.TrimPrefix(claudeAPIRequest, "claude_code."):
		r.t.Requests++
		in, _ := intAttr(attrs, claudeInput)
		read, _ := intAttr(attrs, claudeCacheRead)
		created, _ := intAttr(attrs, claudeCacheCreation)
		out, _ := intAttr(attrs, claudeOutput)
		r.t.InputTokens += in
		r.t.CacheReadTokens += read
		r.t.CacheCreationTokens += created
		r.t.OutputTokens += out
		if cost, ok := floatAttr(attrs, claudeCost); ok {
			r.t.CostUSD += cost
			r.t.CostReported = true
		}
		if src, _ := stringAttr(attrs, claudeQuerySource); src == claudeMainLoop {
			if seq, ok := intAttr(attrs, claudeSequence); ok {
				r.first.offer(seq, in+read+created)
			}
		}
	case codexUserPrompt:
		r.first.prompt(recordTime(rec))
	case codexSSEEvent:
		if kind, _ := stringAttr(attrs, codexKind); kind != codexCompleted {
			return
		}
		r.t.Requests++
		in, _ := intAttr(attrs, codexInput)
		cached, _ := intAttr(attrs, codexCached)
		out, _ := intAttr(attrs, codexOutput)
		r.t.InputTokens += in - cached
		r.t.CacheReadTokens += cached
		r.t.OutputTokens += out
		// Codex gives no sequence number, so the earliest record from the
		// user prompt on wins, and among records with the same time, the
		// one that came first.
		r.first.offerFromPrompt(recordTime(rec), in)
	}
}

// recordName returns the event a log record reports: its event.name
// attribute, else its event name, else its body. The attribute comes
// first, since Codex 0.171.0 fills the event name with the place in its
// source that logged the record.
func recordName(rec *logspb.LogRecord, attrs map[string]*commonpb.AnyValue) string {
	if name, ok := stringAttr(attrs, eventName); ok && name != "" {
		return name
	}
	if name := rec.GetEventName(); name != "" {
		return name
	}
	return rec.GetBody().GetStringValue()
}

// recordTime returns when a log record happened, or when it was observed
// when the harness left the time out, in nanoseconds.
func recordTime(rec *logspb.LogRecord) int64 {
	ts := rec.GetTimeUnixNano()
	if ts == 0 {
		ts = rec.GetObservedTimeUnixNano()
	}
	if ts > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(ts)
}

// addMetrics keeps the skill and plugin names on each data point in data.
func (r *run) addMetrics(data *metricspb.MetricsData) {
	for _, rm := range data.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				for _, attrs := range pointAttrs(m) {
					a := attrMap(attrs)
					r.addNames(a)
					if m.GetName() == codexSkillInjected {
						if name, ok := stringAttr(a, codexSkill); ok && name != "" {
							r.skills[name] = true
						}
					}
				}
			}
		}
	}
}

// pointAttrs returns the attributes of each data point of m, whatever its
// kind.
func pointAttrs(m *metricspb.Metric) [][]*commonpb.KeyValue {
	var out [][]*commonpb.KeyValue
	for _, p := range m.GetSum().GetDataPoints() {
		out = append(out, p.GetAttributes())
	}
	for _, p := range m.GetGauge().GetDataPoints() {
		out = append(out, p.GetAttributes())
	}
	for _, p := range m.GetHistogram().GetDataPoints() {
		out = append(out, p.GetAttributes())
	}
	for _, p := range m.GetExponentialHistogram().GetDataPoints() {
		out = append(out, p.GetAttributes())
	}
	for _, p := range m.GetSummary().GetDataPoints() {
		out = append(out, p.GetAttributes())
	}
	return out
}

// addNames keeps the skill and plugin named in attrs.
func (r *run) addNames(attrs map[string]*commonpb.AnyValue) {
	if name, ok := stringAttr(attrs, skillName); ok && name != "" {
		r.skills[name] = true
	}
	if name, ok := stringAttr(attrs, pluginName); ok && name != "" {
		version, _ := stringAttr(attrs, pluginVersion)
		r.plugins[Plugin{Name: name, Version: version}] = true
	}
}

// attrMap indexes attrs by key.
func attrMap(attrs []*commonpb.KeyValue) map[string]*commonpb.AnyValue {
	m := make(map[string]*commonpb.AnyValue, len(attrs))
	for _, kv := range attrs {
		m[kv.GetKey()] = kv.GetValue()
	}
	return m
}

// stringAttr returns the attribute key as a string, if it is one.
func stringAttr(attrs map[string]*commonpb.AnyValue, key string) (string, bool) {
	v, ok := attrs[key].GetValue().(*commonpb.AnyValue_StringValue)
	if !ok {
		return "", false
	}
	return v.StringValue, true
}

// intAttr returns the attribute key as a whole number: harnesses may send
// a count as an integer, a double or a decimal string.
func intAttr(attrs map[string]*commonpb.AnyValue, key string) (int64, bool) {
	switch v := attrs[key].GetValue().(type) {
	case *commonpb.AnyValue_IntValue:
		return v.IntValue, true
	case *commonpb.AnyValue_DoubleValue:
		return int64(v.DoubleValue), true
	case *commonpb.AnyValue_StringValue:
		n, err := strconv.ParseInt(strings.TrimSpace(v.StringValue), 10, 64)
		return n, err == nil
	}
	return 0, false
}

// floatAttr returns the attribute key as a number, from a double, an
// integer or a decimal string.
func floatAttr(attrs map[string]*commonpb.AnyValue, key string) (float64, bool) {
	switch v := attrs[key].GetValue().(type) {
	case *commonpb.AnyValue_DoubleValue:
		return v.DoubleValue, true
	case *commonpb.AnyValue_IntValue:
		return float64(v.IntValue), true
	case *commonpb.AnyValue_StringValue:
		f, err := strconv.ParseFloat(strings.TrimSpace(v.StringValue), 64)
		return f, err == nil
	}
	return 0, false
}
