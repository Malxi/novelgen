// Package runmetrics records a deterministic, append-only timeline for one
// command invocation and summarizes the resulting Agent runs.
//
// The ledger intentionally stores execution facts, not model-authored quality
// judgments. Prompts and responses remain in the existing ai_calls stream; this
// ledger records their sizes and links back through the run id.
package runmetrics

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"novelgen/internal/platform/envutil"
	"novelgen/internal/platform/store"
)

const (
	SchemaVersion = 1

	EventRunStart   = "run_start"
	EventRunEnd     = "run_end"
	EventAgentCall  = "agent_call"
	EventSkillLoad  = "skill_load"
	EventTool       = "tool"
	EventStuck      = "stuck"
	EventRunResume  = "resume"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"

	// DefaultRunRetention bounds logs/runs/. A creative project accumulates one
	// ledger per command invocation, so without a cap the directory grows until
	// listing it becomes the slowest part of the dashboard.
	DefaultRunRetention = 500

	maxRunIDLength = 128
)

var errRunFinished = errors.New("run is already finished")

// Event is one immutable line in logs/runs/<run_id>.jsonl.
//
// Command is redacted before it is written. Prompt and response bodies are not
// included; PromptChars/ResponseChars let a report show context growth without
// duplicating potentially sensitive creative text.
type Event struct {
	Version           int               `json:"version"`
	RunID             string            `json:"run_id"`
	SpanID            string            `json:"span_id,omitempty"`
	ParentSpanID      string            `json:"parent_span_id,omitempty"`
	Event             string            `json:"event"`
	Time              time.Time         `json:"time"`
	Status            string            `json:"status,omitempty"`
	Agent             string            `json:"agent,omitempty"`
	Model             string            `json:"model,omitempty"`
	Kind              string            `json:"kind,omitempty"`
	Command           string            `json:"command,omitempty"`
	NormalizedCommand string            `json:"normalized_command,omitempty"`
	Skills            []string          `json:"skills,omitempty"`
	LoadedSkills      []string          `json:"loaded_skills,omitempty"`
	MissingSkills     []string          `json:"missing_skills,omitempty"`
	ToolName          string            `json:"tool_name,omitempty"`
	Allowed           *bool             `json:"allowed,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	Error             string            `json:"error,omitempty"`
	DurationMS        int64             `json:"duration_ms,omitempty"`
	PromptTokens      int               `json:"prompt_tokens,omitempty"`
	CompletionTokens  int               `json:"completion_tokens,omitempty"`
	TotalTokens       int               `json:"total_tokens,omitempty"`
	PromptChars       int               `json:"prompt_chars,omitempty"`
	SystemPromptChars int               `json:"system_prompt_chars,omitempty"`
	ResponseChars     int               `json:"response_chars,omitempty"`
	LiveLogPath       string            `json:"live_log_path,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// Run is the shared recorder for one CLI invocation. It is safe for concurrent
// workers because every append is serialized and the store keeps the path.
type Run struct {
	root      *store.Root
	id        string
	command   string
	startedAt time.Time

	mu       sync.Mutex
	finished bool
	counter  atomic.Uint64
}

type runContextKey struct{}

// Start creates a run and appends its run_start event. A nil root or disabled
// metrics returns a nil Run and no error; metrics must never block creative work.
func Start(ctx context.Context, root *store.Root, command string) (context.Context, *Run, error) {
	if metricsDisabled() || root == nil || root.Dir() == "" {
		return ctx, nil, nil
	}
	run := &Run{
		root:      root,
		id:        newRunID(command),
		command:   strings.TrimSpace(command),
		startedAt: time.Now(),
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, runContextKey{}, run)
	if err := run.Emit(Event{
		Event:    EventRunStart,
		Status:   StatusRunning,
		Command:  run.command,
		Metadata: map[string]string{"cwd": root.Dir()},
	}); err != nil {
		return ctx, run, err
	}
	pruneRuns(root, retentionLimit(), run.id)
	return ctx, run, nil
}

// FromContext returns the current run recorder, when one was installed by Start.
func FromContext(ctx context.Context) *Run {
	if ctx == nil {
		return nil
	}
	run, _ := ctx.Value(runContextKey{}).(*Run)
	return run
}

func (r *Run) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}

// NewSpanID returns a stable, readable identifier for one model/tool span.
func (r *Run) NewSpanID(prefix string) string {
	if r == nil {
		return ""
	}
	prefix = sanitizePart(prefix)
	if prefix == "" {
		prefix = "span"
	}
	return fmt.Sprintf("%s-%04d", prefix, r.counter.Add(1))
}

// Emit appends one event. It is deliberately best-effort for callers that run
// inside a creative stage; observers can inspect the returned error in tests.
func (r *Run) Emit(event Event) error {
	if r == nil || r.root == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished && event.Event != EventRunResume {
		return errRunFinished
	}
	// A resume event reopens the ledger; the next run_end owns the final state.
	r.finished = false
	return r.appendLocked(event)
}

// appendLocked writes one event while r.mu is held. Callers that finish the run
// reserve r.finished before calling it, so two concurrent Finish calls cannot
// both append a run_end.
func (r *Run) appendLocked(event Event) error {
	event.Version = SchemaVersion
	if event.RunID == "" {
		event.RunID = r.id
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	if event.Command != "" {
		event.Command = MaskCommand(event.Command)
	}
	if event.NormalizedCommand == "" && event.Command != "" {
		event.NormalizedCommand = NormalizeCommand(event.Command)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := r.root.Append(r.root.RunMetricsFile(r.id), append(data, '\n'), store.FileMode); err != nil {
		return err
	}
	if event.Event == EventRunEnd {
		r.finished = true
	}
	return nil
}

// Finish appends the terminal run event. It is safe to call more than once and
// from several goroutines: the finished flag is reserved under the same lock
// that serializes appends, so exactly one run_end is written.
func (r *Run) Finish(status string, err error) {
	if r == nil || r.root == nil {
		return
	}
	if strings.TrimSpace(status) == "" {
		if err != nil {
			status = StatusFailed
		} else {
			status = StatusCompleted
		}
	}
	event := Event{
		Event:      EventRunEnd,
		Status:     status,
		DurationMS: time.Since(r.startedAt).Milliseconds(),
		Command:    r.command,
	}
	if err != nil {
		event.Error = err.Error()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	if appendErr := r.appendLocked(event); appendErr != nil {
		// The terminal event never landed; allow a later attempt instead of
		// leaving the ledger permanently open.
		r.finished = false
	}
}

// FinishFromContext finishes the run attached to ctx, if any.
func FinishFromContext(ctx context.Context, status string, err error) {
	FromContext(ctx).Finish(status, err)
}

// ListOptions filters run summaries.
type ListOptions struct {
	Limit int
	Since time.Time
	Stuck bool
}

// RunSummary is the compact representation used by the CLI and Web UI.
type RunSummary struct {
	RunID            string        `json:"run_id"`
	Command          string        `json:"command,omitempty"`
	Status           string        `json:"status"`
	StartedAt        string        `json:"started_at,omitempty"`
	FinishedAt       string        `json:"finished_at,omitempty"`
	DurationMS       int64         `json:"duration_ms,omitempty"`
	Events           int           `json:"events"`
	AgentCalls       int           `json:"agent_calls"`
	ToolCalls        int           `json:"tool_calls"`
	ToolDenied       int           `json:"tool_denied,omitempty"`
	Retries          int           `json:"retries,omitempty"`
	StuckEvents      int           `json:"stuck_events,omitempty"`
	PromptTokens     int           `json:"prompt_tokens,omitempty"`
	CompletionTokens int           `json:"completion_tokens,omitempty"`
	TotalTokens      int           `json:"total_tokens,omitempty"`
	Agents           []string      `json:"agents,omitempty"`
	Models           []string      `json:"models,omitempty"`
	Skills           []string      `json:"skills,omitempty"`
	Commands         []string      `json:"commands,omitempty"`
	Stuck            []StuckSignal `json:"stuck,omitempty"`
	Errors           []string      `json:"errors,omitempty"`
	LiveLog          string        `json:"live_log,omitempty"`
}

type StuckSignal struct {
	Reason string `json:"reason"`
	Event  string `json:"event,omitempty"`
	Count  int    `json:"count,omitempty"`
}

// RunDetail contains the summary and the complete timeline.
type RunDetail struct {
	Summary RunSummary `json:"summary"`
	Events  []Event    `json:"events"`
}

// List reads all run ledgers under the project and returns newest first.
func List(root *store.Root, opts ListOptions) ([]RunSummary, error) {
	if root == nil {
		return nil, store.ErrNilRoot
	}
	dir := root.RunMetricsDir()
	entries, err := root.FS().ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []RunSummary{}, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".jsonl"))
	}
	// Run ids start with a fixed-width timestamp, so name order is time order.
	// When no post-read filter is active, slice before parsing so --limit bounds
	// the IO instead of only the returned slice.
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if opts.Limit > 0 && !opts.Stuck && opts.Since.IsZero() && len(ids) > opts.Limit {
		ids = ids[:opts.Limit]
	}
	summaries := make([]RunSummary, 0, len(ids))
	for _, runID := range ids {
		summary, err := summarizeFile(root, runID)
		if err != nil {
			return nil, err
		}
		if !opts.Since.IsZero() {
			started, _ := time.Parse(time.RFC3339Nano, summary.StartedAt)
			if !started.IsZero() && started.Before(opts.Since) {
				continue
			}
		}
		if opts.Stuck && summary.StuckEvents == 0 {
			continue
		}
		summaries = append(summaries, summary)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].StartedAt == summaries[j].StartedAt {
			return summaries[i].RunID > summaries[j].RunID
		}
		return summaries[i].StartedAt > summaries[j].StartedAt
	})
	if opts.Limit > 0 && len(summaries) > opts.Limit {
		summaries = summaries[:opts.Limit]
	}
	return summaries, nil
}

// Read loads one run by id. The id is validated rather than silently rewritten:
// a caller that passes a path-like or malformed id must get an error, not a
// different run's timeline.
func Read(root *store.Root, runID string) (RunDetail, error) {
	if root == nil {
		return RunDetail{}, store.ErrNilRoot
	}
	if !ValidRunID(runID) {
		return RunDetail{}, fmt.Errorf("invalid run id %q", runID)
	}
	events, err := readEvents(root, runID)
	if err != nil {
		return RunDetail{}, err
	}
	return RunDetail{Summary: summarize(runID, events), Events: events}, nil
}

// ValidRunID reports whether id is a safe ledger file name. New ids are built by
// newRunID; this guards explicit ids arriving from the CLI or HTTP boundary.
func ValidRunID(id string) bool {
	if id == "" || len(id) > maxRunIDLength || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func summarizeFile(root *store.Root, runID string) (RunSummary, error) {
	events, err := readEvents(root, runID)
	if err != nil {
		return RunSummary{}, err
	}
	return summarize(runID, events), nil
}

func readEvents(root *store.Root, runID string) ([]Event, error) {
	data, err := root.Read(root.RunMetricsFile(runID))
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	events := make([]Event, 0, 64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			events = append(events, Event{
				Version: SchemaVersion,
				RunID:   runID,
				Event:   "metric_parse_error",
				Time:    time.Now(),
				Error:   err.Error(),
			})
			continue
		}
		if event.Version > SchemaVersion {
			events = append(events, Event{
				Version: SchemaVersion,
				RunID:   runID,
				Event:   "metric_parse_error",
				Time:    time.Now(),
				Error:   fmt.Sprintf("unsupported schema version %d (this build reads up to %d)", event.Version, SchemaVersion),
			})
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan run %s: %w", runID, err)
	}
	return events, nil
}

func summarize(runID string, events []Event) RunSummary {
	summary := RunSummary{RunID: runID, Status: StatusRunning}
	agents, models, skills, commands := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	allowedCommandCounts := map[string]int{}
	stuckSignals := map[string]*StuckSignal{}
	addStuck := func(reason string) {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return
		}
		if signal := stuckSignals[reason]; signal != nil {
			signal.Count++
			return
		}
		stuckSignals[reason] = &StuckSignal{Reason: reason, Count: 1}
		summary.StuckEvents++
	}
	hasStructuredPre := false
	for _, event := range events {
		if event.Event == EventTool && event.Kind == "pre" {
			hasStructuredPre = true
			break
		}
	}
	for _, event := range events {
		summary.Events++
		if summary.Command == "" && event.Command != "" {
			summary.Command = event.Command
		}
		if event.Event == EventRunStart {
			summary.StartedAt = event.Time.Format(time.RFC3339Nano)
		}
		if event.Event == EventRunEnd {
			summary.FinishedAt = event.Time.Format(time.RFC3339Nano)
			summary.DurationMS = event.DurationMS
			summary.Status = event.Status
			if summary.Status == "" {
				summary.Status = StatusCompleted
			}
		}
		if event.Agent != "" {
			agents[event.Agent] = true
		}
		if event.Model != "" {
			models[event.Model] = true
		}
		for _, skill := range append(append([]string(nil), event.Skills...), event.LoadedSkills...) {
			if strings.TrimSpace(skill) != "" {
				skills[skill] = true
			}
		}
		if event.NormalizedCommand != "" {
			commands[event.NormalizedCommand] = true
		}
		switch event.Event {
		case EventAgentCall:
			summary.AgentCalls++
			summary.PromptTokens += event.PromptTokens
			summary.CompletionTokens += event.CompletionTokens
			summary.TotalTokens += event.TotalTokens
			if event.Status == "error" {
				summary.Retries++
			}
		case EventTool:
			if event.Kind == "post" || (hasStructuredPre && event.Kind == "permission") {
				continue
			}
			summary.ToolCalls++
			if event.Allowed != nil && !*event.Allowed {
				summary.ToolDenied++
			} else if event.NormalizedCommand != "" {
				allowedCommandCounts[event.NormalizedCommand]++
			}
		case EventStuck:
			addStuck(event.Reason)
			summary.Retries++
		}
		if event.Error != "" && len(summary.Errors) < 8 && !contains(summary.Errors, event.Error) {
			summary.Errors = append(summary.Errors, event.Error)
		}
		if event.LiveLogPath != "" {
			summary.LiveLog = event.LiveLogPath
		}
	}
	summary.Agents = sortedKeys(agents)
	summary.Models = sortedKeys(models)
	summary.Skills = sortedKeys(skills)
	summary.Commands = sortedKeys(commands)
	for command, count := range allowedCommandCounts {
		if count >= 3 {
			addStuck(fmt.Sprintf("repeated_command:%s (%d)", command, count))
		}
	}
	reasons := make([]string, 0, len(stuckSignals))
	for reason := range stuckSignals {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		summary.Stuck = append(summary.Stuck, *stuckSignals[reason])
	}
	return summary
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func metricsDisabled() bool {
	return envutil.Bool("NOVELGEN_DISABLE_RUN_METRICS", false)
}

// retentionLimit reads NOVELGEN_RUN_METRICS_KEEP; 0 disables pruning.
func retentionLimit() int {
	if raw := strings.TrimSpace(envutil.String("NOVELGEN_RUN_METRICS_KEEP")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			return value
		}
	}
	return DefaultRunRetention
}

// pruneRuns deletes the oldest ledgers beyond keep. It is best-effort: metrics
// must never fail a creative command, and the current run is never removed.
func pruneRuns(root *store.Root, keep int, currentID string) {
	if root == nil || keep <= 0 {
		return
	}
	entries, err := root.FS().ReadDir(root.RunMetricsDir())
	if err != nil || len(entries) <= keep {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".jsonl")
		if runID == currentID {
			continue
		}
		names = append(names, runID)
	}
	if len(names)+1 <= keep {
		return
	}
	sort.Strings(names)
	removeCount := len(names) + 1 - keep
	if removeCount > len(names) {
		removeCount = len(names)
	}
	for _, runID := range names[:removeCount] {
		_ = root.Remove(root.RunMetricsFile(runID))
	}
}

func newRunID(command string) string {
	slug := sanitizePart(command)
	if slug == "" {
		slug = "run"
	}
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		random = []byte{0, 0, 0, uint8(time.Now().UnixNano())}
	}
	return fmt.Sprintf("%s_%s_%s", time.Now().Format("20060102_150405"), slug, hex.EncodeToString(random))
}

func sanitizePart(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			if b.Len() > 0 {
				b.WriteByte('_')
			}
		}
	}
	return strings.Trim(b.String(), "._")
}

// keepValueFlags are structural arguments whose values are safe to keep: they
// say *what* was operated on, never the creative payload itself. Every other
// flag value is redacted, so a new free-text flag cannot leak a manuscript into
// logs/runs without an explicit decision here.
var keepValueFlags = map[string]bool{
	"--id": true, "--chapter": true, "--chapter-id": true, "--volume": true,
	"--volume-id": true, "--part": true, "--scope": true, "--target": true,
	"--limit": true, "--since": true, "--dir": true, "--project": true,
	"--from": true, "--to": true, "--kind": true, "--mode": true,
	"--provider": true, "--model": true, "--language": true, "--format": true,
	"--type": true, "--status": true, "--run": true,
}

// booleanFlags never consume the following token.
var booleanFlags = map[string]bool{
	"--apply": true, "--json": true, "--dry-run": true, "--stuck": true,
	"--force": true, "--all": true, "--quiet": true, "--verbose": true,
	"--yes": true, "--help": true, "--no-color": true,
}

// identityValueFlags name one specific entity; their values are dropped from
// the normalized shape so repeated checks of different chapters group together.
var identityValueFlags = map[string]bool{
	"--id": true, "--chapter-id": true, "--volume-id": true, "--name": true,
	"--patch-buffer": true, "--run": true, "--since": true,
}

// NormalizeCommand reduces volatile payloads to a stable command shape so a
// dashboard can group repeated calls without grouping every unique id.
func NormalizeCommand(command string) string {
	tokens := splitCommand(MaskCommand(command))
	out := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "--" {
			break
		}
		if strings.HasPrefix(token, "--") {
			name, value, hasInlineValue := strings.Cut(token, "=")
			name = strings.ToLower(name)
			out = append(out, name)
			if booleanFlags[name] {
				continue
			}
			if hasInlineValue {
				if keepValueFlags[name] && !identityValueFlags[name] {
					out = append(out, value)
				}
				continue
			}
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				next := tokens[i+1]
				i++
				if keepValueFlags[name] && !identityValueFlags[name] {
					out = append(out, next)
				}
			}
			continue
		}
		if isCommandToken(token) {
			out = append(out, strings.ToLower(token))
		}
	}
	return strings.Join(out, " ")
}

// MaskCommand hides content arguments while preserving the command shape needed
// for diagnosis. It keeps structural flag values and command words; every other
// free-form token is replaced, so the safe outcome is the default for flags this
// package has never seen.
func MaskCommand(command string) string {
	tokens := splitCommand(strings.TrimSpace(command))
	out := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "--" {
			// Everything after "--" is positional, i.e. potentially payload.
			out = append(out, "--")
			break
		}
		if strings.HasPrefix(token, "--") {
			name, value, hasInlineValue := strings.Cut(token, "=")
			name = strings.ToLower(name)
			switch {
			case hasInlineValue:
				if keepValueFlags[name] {
					out = append(out, name+"="+value)
				} else {
					out = append(out, name+"=<redacted>")
				}
			case booleanFlags[name]:
				out = append(out, name)
			case keepValueFlags[name]:
				out = append(out, name)
				if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
					i++
					out = append(out, tokens[i])
				}
			default:
				out = append(out, name)
				if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
					i++
					out = append(out, "<redacted>")
				}
			}
			continue
		}
		if isCommandToken(token) {
			out = append(out, token)
		} else {
			out = append(out, "<redacted>")
		}
	}
	return strings.Join(out, " ")
}

// splitCommand tokenizes a command line, honoring single and double quotes so a
// quoted payload stays one token (and is redacted as a unit).
func splitCommand(command string) []string {
	tokens := make([]string, 0, 16)
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// isCommandToken reports whether a bare token looks like a command or
// subcommand word rather than user content (ids, JSON, titles, prose).
func isCommandToken(token string) bool {
	if token == "" || len(token) > 40 {
		return false
	}
	for i, r := range token {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_'):
		default:
			return false
		}
	}
	return true
}
