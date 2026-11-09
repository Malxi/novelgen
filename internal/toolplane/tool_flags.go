// The flag structs of the tool plane commands: one per command, bound to it in init() of tool.go.

package toolplane

import ()

var ToolCheckFlags struct {
	Target      string
	Scope       string
	ID          string
	MinPriority string
	Category    string
	MaxIssues   int
	TargetWords int
}

var ToolPatchFlags struct {
	Target         string
	ID             string
	PatchJSON      string
	PatchBuffer    string
	Task           string
	Apply          bool
	DryRun         bool
	RefreshDerived bool
	TargetWords    int
}

// ToolPatchMaxIssues is separate from ToolPatchFlags because tests and callers
// construct ToolPatchFlags literals; the cap is an output-only concern.
var ToolPatchMaxIssues int

var ToolPatchBufferFlags struct {
	ID       string
	Text     string
	Stdin    bool
	MaxChars int
}

var ToolRefreshFlags struct {
	ID        string
	BatchSize int
}
