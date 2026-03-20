package cmd

import "strings"

func formatReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, r := range reasons {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		sb.WriteString("- " + r + "\n")
	}
	return sb.String()
}
