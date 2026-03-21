package transition

import (
	"regexp"
	"strings"

	"novelgen/internal/models"
)

// TransitionCheckResult is a lightweight heuristic check for chapter-to-chapter transitions.
type TransitionCheckResult struct {
	HasIssue    bool
	Issues      []string
	Suggestions []string
}

var (
	reLoc               = regexp.MustCompile(`(?m)^(?:\s*)(?:地点|场景|位置)[:：]\s*(.+)$`)
	reTime              = regexp.MustCompile(`(?m)(?:\d+分钟后|\d+小时后|翌日|次日|第二天|隔天|翌晨|清晨|傍晚|夜里|当晚|几天后|数日后|转眼|片刻后|不久后)`)
	reOpeningTransition = regexp.MustCompile(`(?m)(与此同时|另一边|镜头一转|画面一转|话分两头|转场|切回|切到)`)
	reBridgeBlock       = regexp.MustCompile(`(?s)<TRANSITION_BRIDGE>\s*(.*?)\s*</TRANSITION_BRIDGE>`)
)

// CheckTeleportOpening uses recap anchors + outline summary to detect likely teleport transitions.
func CheckTeleportOpening(prevRecap *models.ChapterRecap, currentChapter *models.Chapter, currentText string) *TransitionCheckResult {
	res := &TransitionCheckResult{}
	if prevRecap == nil || currentChapter == nil {
		return res
	}

	summary := strings.TrimSpace(currentChapter.Summary)
	// If the outline summary clearly indicates a transition, don't flag.
	if reTime.MatchString(summary) || strings.Contains(summary, "转场") || strings.Contains(summary, "来到") || strings.Contains(summary, "前往") || strings.Contains(summary, "赶往") {
		return res
	}

	open := firstNChars(strings.TrimSpace(currentText), 800)
	// If the opening itself clearly declares a transition, don't flag.
	if reOpeningTransition.MatchString(open) {
		return res
	}

	prevLoc := strings.TrimSpace(prevRecap.Location)
	currentLoc := strings.TrimSpace(currentChapter.Location)

	// Heuristic #1: if outline declares a different location than the previous recap
	if prevLoc != "" && currentLoc != "" && currentLoc != prevLoc {
		res.HasIssue = true
		res.Issues = append(res.Issues, "本章大纲地点与上一章结尾地点不一致，但本章摘要/开头未声明转场/跳时（疑似瞬移转场）")
		res.Suggestions = append(res.Suggestions, "【PATCH 请求：TRANSITION_BRIDGE】建议在开头补一个转场桥段：用上一章最后一幕/最后一句话起手，交代为何离开上一地点、如何到达新地点、经过了多久")
		return res
	}

	// Heuristic #2: Extract explicit location marker from opening if present
	m := reLoc.FindStringSubmatch(open)
	if len(m) >= 2 {
		openingLoc := strings.TrimSpace(m[1])
		if prevLoc != "" && openingLoc != "" && openingLoc != prevLoc {
			res.HasIssue = true
			res.Issues = append(res.Issues, "章节开头地点与上一章结尾地点不一致，但本章摘要/开头未声明转场/跳时")
			res.Suggestions = append(res.Suggestions, "【PATCH 请求：TRANSITION_BRIDGE】建议在开头补一个转场桥段：用上一章最后一幕/最后一句话起手，交代为何离开上一地点、如何到达新地点、经过了多久")
		}
	}

	return res
}

// ExtractTransitionBridge extracts the bridge text inside <TRANSITION_BRIDGE> tags.
func ExtractTransitionBridge(text string) (string, bool) {
	m := reBridgeBlock.FindStringSubmatch(text)
	if len(m) < 2 {
		return "", false
	}
	b := strings.TrimSpace(m[1])
	if b == "" {
		return "", false
	}
	return b, true
}

// InsertTransitionBridge inserts bridge text at the start of chapter content.
func InsertTransitionBridge(original string, bridge string) string {
	orig := strings.TrimSpace(original)
	b := strings.TrimSpace(bridge)
	if b == "" {
		return original
	}
	if orig == "" {
		return b
	}
	return b + "\n\n" + orig
}

// ValidateTransitionBridgeOutput validates that a patch block exists and is well-formed.
func ValidateTransitionBridgeOutput(text string) (ok bool, reasons []string) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return false, []string{"输出为空"}
	}

	m := reBridgeBlock.FindStringSubmatch(clean)
	if len(m) < 2 {
		return false, []string{"未找到 <TRANSITION_BRIDGE>...</TRANSITION_BRIDGE> 补丁块"}
	}

	bridge := strings.TrimSpace(m[1])
	if bridge == "" {
		return false, []string{"TRANSITION_BRIDGE 内容为空"}
	}

	// Bridge should be close to the beginning (within first ~1500 chars)
	idx := strings.Index(clean, "<TRANSITION_BRIDGE>")
	if idx > 1500 {
		reasons = append(reasons, "TRANSITION_BRIDGE 不在开头附近（疑似未按要求作为开场补丁）")
	}

	return len(reasons) == 0, reasons
}

func firstNChars(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
