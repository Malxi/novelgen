package shared

// NextChapterInfo holds info about upcoming chapters for foreshadowing. Both
// the legacy draft stage and the write stage include it in their prompts.
type NextChapterInfo struct {
	ID      string `json:"id" desc:"Chapter ID"`
	Title   string `json:"title" desc:"Chapter title"`
	Summary string `json:"summary" desc:"Chapter summary"`
}
