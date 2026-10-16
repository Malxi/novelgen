// The context section entry point: it routes a context query type to the
// workflow-specific bundle implemented in the tool_context_* files.

package toolplane

import (
	"fmt"
	"strings"
)

func QueryContext(ctx ToolProjectContext, queryType, id, name string) ToolResponse {
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
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported context type %q", queryType)}}
	}
}
