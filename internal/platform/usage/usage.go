// Package usage records per-call LLM token usage so long creative runs can be
// audited and costed after the fact.
package usage

import (
	"sort"
	"strings"
	"time"
)

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

// Reading and appending the ledger lives in internal/platform/store
// (Root.ReadUsage / Root.AppendUsage): the ledger's location is part of the
// project layout, and this package stays a pure record/aggregate layer.

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
