// The context section entry point: it routes a context query type to the
// workflow-specific bundle implemented in the tool_context_* files.

package toolplane

import (
	"fmt"
	"strings"
)

type QueryContextOptions struct {
	Intent   string
	Focus    string
	Limit    int
	MaxChars int
}

func QueryContext(ctx ToolProjectContext, queryType, id, name string) ToolResponse {
	return QueryContextWithOptions(ctx, queryType, id, name, QueryContextOptions{})
}

func QueryContextWithOptions(ctx ToolProjectContext, queryType, id, name string, opts QueryContextOptions) ToolResponse {
	contextType := normalizeContextType(queryType)
	switch contextType {
	case "craft-character", "craft-item", "craft-location", "craft-organization":
		return queryCraftElementContext(ctx, strings.TrimPrefix(contextType, "craft-"), name)
	case "outline-volume":
		return queryOutlineVolumeContext(ctx, id, name)
	case "outline-repair":
		return queryOutlineRepairContext(ctx, id, name)
	case "outline-global-repair":
		return QueryOutlineGlobalRepairContext(ctx, name)
	case "recap-repair":
		return queryRecapRepairContext(ctx, id)
	case "chapter-write":
		return queryChapterWriteContext(ctx, id)
	case "chapter-repair":
		return queryChapterRepairContext(ctx, id, name)
	case "narrative-memory":
		if ctx.Root == nil || ctx.Root.Dir() == "" {
			return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"narrative memory requires a project root"}}
		}
		result, warnings, err := queryNarrativeMemory(ctx.Root, memoryQueryOptions{
			ChapterID: id,
			Intent:    opts.Intent,
			Focus:     firstMemoryFocus(name, opts.Focus),
			MaxChars:  opts.MaxChars,
			Limit:     opts.Limit,
		})
		if err != nil {
			return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: append(warnings, err.Error())}
		}
		return memoryToolResponse(result, warnings)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported context type %q", queryType)}}
	}
}

func firstMemoryFocus(name, focus string) string {
	if strings.TrimSpace(focus) != "" {
		return strings.TrimSpace(focus)
	}
	return strings.TrimSpace(name)
}
