package cmd

import "strings"

func transitionFixFeedback(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Teleport 复检失败（必须修复）\n")
	for _, it := range issues {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		sb.WriteString("- " + it + "\n")
	}
	sb.WriteString("\n请重新生成 TRANSITION_BRIDGE，确保开头不再触发瞬移判定。")
	return sb.String()
}
