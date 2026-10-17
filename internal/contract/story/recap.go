package models

// ChapterRecap is a compact, canonical summary of a chapter for continuity.
// It is designed to be fed into the next chapter as high-signal context.
type ChapterRecap struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	// Scene anchor
	Location string   `json:"location"`
	Time     string   `json:"time"` // e.g. "same night", "next morning", "immediately after"
	Present  []string `json:"present"`
	// What actually happened (not what was planned)
	PlotBeats []string `json:"plot_beats"`
	Decisions []string `json:"decisions"`
	Reveals   []string `json:"reveals"`
	// Continuity locks
	Unresolved []string `json:"unresolved"`
	Promises   []string `json:"promises"`
	Items      []string `json:"items"`  // ownership/status changes in plain text
	Status     []string `json:"status"` // injuries, mood, power level, etc.
	// The last moment of the chapter
	LastLine    string `json:"last_line"`
	Cliffhanger string `json:"cliffhanger"`
	// Optional: suggestion for next chapter opening
	NextOpeningHint string `json:"next_opening_hint"`
}
