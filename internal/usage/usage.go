// Package usage records per-call LLM token usage so long creative runs can be
// audited and costed after the fact.
package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RelativeLogPath is where entries are appended inside a project.
const RelativeLogPath = "logs/usage.jsonl"

// Entry is one recorded LLM call.
type Entry struct {
	Time             time.Time `json:"time"`
	Agent            string    `json:"agent"`
	Command          string    `json:"command,omitempty"`
	Kind             string    `json:"kind,omitempty"`
	Model            string    `json:"model,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	DurationMS       int64     `json:"duration_ms,omitempty"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
}

// Append adds one entry to <root>/logs/usage.jsonl. Failures are returned so
// callers can decide whether to surface them; usage logging is never fatal.
func Append(root string, entry Entry) error {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	path := filepath.Join(root, filepath.FromSlash(RelativeLogPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create usage log dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open usage log: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal usage entry: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write usage entry: %w", err)
	}
	return nil
}

// Read loads every entry for a project. A missing log is not an error.
func Read(root string) ([]Entry, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	path := filepath.Join(root, filepath.FromSlash(RelativeLogPath))
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	entries := []Entry{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip unreadable lines instead of failing the whole report.
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Bucket aggregates tokens for one grouping key.
type Bucket struct {
	Key              string `json:"key"`
	Calls            int    `json:"calls"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Failures         int    `json:"failures,omitempty"`
}

// Summary is the aggregated view of a usage log.
type Summary struct {
	Calls            int      `json:"calls"`
	FailedCalls      int      `json:"failed_calls"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	FirstCall        string   `json:"first_call,omitempty"`
	LastCall         string   `json:"last_call,omitempty"`
	Failures         []Bucket `json:"failures,omitempty"`
	ByAgent          []Bucket `json:"by_agent"`
	ByModel          []Bucket `json:"by_model"`
	ByDay            []Bucket `json:"by_day"`
}

// Summarize groups entries by agent, model, and day. Entries are expected to be
// chronological but do not have to be.
func Summarize(entries []Entry) Summary {
	summary := Summary{ByAgent: []Bucket{}, ByModel: []Bucket{}, ByDay: []Bucket{}}
	byAgent := map[string]*Bucket{}
	byModel := map[string]*Bucket{}
	byDay := map[string]*Bucket{}
	failures := map[string]*Bucket{}

	var first, last time.Time
	for _, entry := range entries {
		summary.Calls++
		summary.PromptTokens += entry.PromptTokens
		summary.CompletionTokens += entry.CompletionTokens
		summary.TotalTokens += entry.TotalTokens
		if !entry.Time.IsZero() {
			if first.IsZero() || entry.Time.Before(first) {
				first = entry.Time
			}
			if last.IsZero() || entry.Time.After(last) {
				last = entry.Time
			}
		}

		add(byAgent, nonEmpty(entry.Agent, "unknown"), entry)
		add(byModel, nonEmpty(entry.Model, "unknown"), entry)
		if !entry.Time.IsZero() {
			add(byDay, entry.Time.Format("2006-01-02"), entry)
		}
		if !entry.Success {
			summary.FailedCalls++
			key := entry.Error
			if key == "" {
				key = "unspecified error"
			}
			add(failures, key, entry)
		}
	}

	summary.FirstCall = formatTime(first)
	summary.LastCall = formatTime(last)
	summary.ByAgent = sortedBuckets(byAgent)
	summary.ByModel = sortedBuckets(byModel)
	summary.ByDay = sortedBuckets(byDay)
	summary.Failures = sortedBuckets(failures)
	return summary
}

func add(target map[string]*Bucket, key string, entry Entry) {
	bucket, ok := target[key]
	if !ok {
		bucket = &Bucket{Key: key}
		target[key] = bucket
	}
	bucket.Calls++
	bucket.PromptTokens += entry.PromptTokens
	bucket.CompletionTokens += entry.CompletionTokens
	bucket.TotalTokens += entry.TotalTokens
	if !entry.Success {
		bucket.Failures++
	}
}

// sortedBuckets orders buckets by descending token use, then by key so output
// is stable.
func sortedBuckets(source map[string]*Bucket) []Bucket {
	buckets := make([]Bucket, 0, len(source))
	for _, bucket := range source {
		buckets = append(buckets, *bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].TotalTokens != buckets[j].TotalTokens {
			return buckets[i].TotalTokens > buckets[j].TotalTokens
		}
		if buckets[i].Calls != buckets[j].Calls {
			return buckets[i].Calls > buckets[j].Calls
		}
		return buckets[i].Key < buckets[j].Key
	})
	return buckets
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

// FilterSince keeps entries at or after the given time.
func FilterSince(entries []Entry, since time.Time) []Entry {
	if since.IsZero() {
		return entries
	}
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Time.IsZero() || !entry.Time.Before(since) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
