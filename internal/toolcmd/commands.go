// Package toolcmd holds the canonical builders for the `novelgen tool ...`
// commands. The tool plane advertises these commands in its navigation output
// and the agents instruct the SDK agent to run them; keeping one builder for
// each command shape means both sides always agree on flag spelling and flag
// order.
package toolcmd

import (
	"fmt"
	"strconv"
	"strings"
)

// ChapterCheckOptions selects the optional flags of a focused chapter check.
type ChapterCheckOptions struct {
	Kind        string // "all" (default), "quality" or "simulation"
	Category    string
	MinPriority string
	MaxIssues   int
	TargetWords int
}

// ChapterCheck builds `novelgen tool check KIND --target chapter --scope chapter
// --id <id>` with the optional flags in their canonical order:
// --category, --min-priority, --max-issues, --target-words.
func ChapterCheck(chapterID string, opts ChapterCheckOptions) string {
	kind := strings.TrimSpace(opts.Kind)
	if kind == "" {
		kind = "all"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "novelgen tool check %s --target chapter --scope chapter --id %q", kind, chapterID)
	if category := strings.TrimSpace(opts.Category); category != "" {
		b.WriteString(" --category " + category)
	}
	if priority := strings.TrimSpace(opts.MinPriority); priority != "" {
		b.WriteString(" --min-priority " + priority)
	}
	if opts.MaxIssues > 0 {
		b.WriteString(" --max-issues " + strconv.Itoa(opts.MaxIssues))
	}
	if opts.TargetWords > 0 {
		b.WriteString(" --target-words " + strconv.Itoa(opts.TargetWords))
	}
	return b.String()
}

// ChapterDSLRefresh builds `novelgen tool refresh chapter-dsl --id <id>`.
func ChapterDSLRefresh(chapterID string) string {
	return fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID)
}
