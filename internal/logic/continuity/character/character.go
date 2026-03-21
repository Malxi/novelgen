package character

import (
	"regexp"
	"sort"
	"strings"

	"novelgen/internal/models"
)

// CharacterCheckResult is a lightweight heuristic check for character consistency.
type CharacterCheckResult struct {
	HasIssue    bool
	Issues      []string
	Suggestions []string
}

// CharacterPresenceDetails provides structured info useful for auto-fixing.
type CharacterPresenceDetails struct {
	MissingExpected  []string
	UnexpectedInOpen []string
}

var (
	reCJKName   = regexp.MustCompile(`([\p{Han}]{2,4})`)
	reCharPatch = regexp.MustCompile(`(?s)<CHARACTER_PRESENCE_PATCH>\s*(.*?)\s*</CHARACTER_PRESENCE_PATCH>`)
)

// CheckCharacterPresence validates that characters expected in the chapter appear in the draft.
func CheckCharacterPresence(chapter *models.Chapter, draft string, knownCharacters []string) *CharacterCheckResult {
	res := &CharacterCheckResult{}
	if chapter == nil {
		return res
	}
	text := strings.TrimSpace(draft)
	if text == "" {
		return res
	}

	knownSet := make(map[string]bool, len(knownCharacters))
	for _, n := range knownCharacters {
		n = strings.TrimSpace(n)
		if n != "" {
			knownSet[n] = true
		}
	}

	// Expected characters should appear
	for _, n := range chapter.Characters {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !strings.Contains(text, n) {
			res.HasIssue = true
			res.Issues = append(res.Issues, "大纲角色未在草稿中出现: "+n)
			res.Suggestions = append(res.Suggestions, "【PATCH 请求：CHARACTER_PRESENCE_PATCH】在开头引入角色: "+n)
		}
	}

	// Opening shouldn't heavily feature unexpected known character
	open := firstNRunes(text, 800)
	allowed := map[string]bool{}
	for _, n := range chapter.Characters {
		if strings.TrimSpace(n) != "" {
			allowed[n] = true
		}
	}

	unexpected := []string{}
	for name := range knownSet {
		if allowed[name] {
			continue
		}
		cnt := strings.Count(open, name)
		if cnt >= 2 {
			unexpected = append(unexpected, name)
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		res.HasIssue = true
		res.Issues = append(res.Issues, "开头出现非大纲角色: "+strings.Join(unexpected, ", "))
		res.Suggestions = append(res.Suggestions, "【PATCH 请求：CHARACTER_PRESENCE_PATCH】调整开头，避免过早引入: "+strings.Join(unexpected, ", "))
	}

	return res
}

// CheckCharacterPresenceDetailed returns both the check result and structured details.
func CheckCharacterPresenceDetailed(chapter *models.Chapter, draft string, knownCharacters []string) (*CharacterCheckResult, *CharacterPresenceDetails) {
	res := CheckCharacterPresence(chapter, draft, knownCharacters)
	d := &CharacterPresenceDetails{}
	if chapter == nil {
		return res, d
	}
	text := strings.TrimSpace(draft)
	if text == "" {
		return res, d
	}

	for _, n := range chapter.Characters {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !strings.Contains(text, n) {
			d.MissingExpected = append(d.MissingExpected, n)
		}
	}

	open := firstNRunes(text, 800)
	allowed := map[string]bool{}
	for _, n := range chapter.Characters {
		if strings.TrimSpace(n) != "" {
			allowed[n] = true
		}
	}
	knownSet := make(map[string]bool, len(knownCharacters))
	for _, n := range knownCharacters {
		n = strings.TrimSpace(n)
		if n != "" {
			knownSet[n] = true
		}
	}
	for name := range knownSet {
		if allowed[name] {
			continue
		}
		if strings.Count(open, name) >= 2 {
			d.UnexpectedInOpen = append(d.UnexpectedInOpen, name)
		}
	}

	return res, d
}

// ExtractCharacterPresencePatch extracts the patch content.
func ExtractCharacterPresencePatch(text string) (string, bool) {
	m := reCharPatch.FindStringSubmatch(text)
	if len(m) < 2 {
		return "", false
	}
	p := strings.TrimSpace(m[1])
	if p == "" {
		return "", false
	}
	return p, true
}

// InsertCharacterPresencePatch inserts a short patch near the start of the chapter.
func InsertCharacterPresencePatch(original string, patch string) string {
	orig := strings.TrimSpace(original)
	p := strings.TrimSpace(patch)
	if p == "" {
		return original
	}
	if orig == "" {
		return p
	}
	return p + "\n\n" + orig
}

// ValidateCharacterPresencePatchOutput validates that a patch block exists.
func ValidateCharacterPresencePatchOutput(text string) (ok bool, reasons []string) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return false, []string{"输出为空"}
	}
	if _, ok := ExtractCharacterPresencePatch(clean); !ok {
		return false, []string{"未找到 <CHARACTER_PRESENCE_PATCH>...</CHARACTER_PRESENCE_PATCH> 补丁块"}
	}
	return true, nil
}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
