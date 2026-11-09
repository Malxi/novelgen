package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

// CustomRule is one user-authored check under .novelgen/rules/*.json.
type CustomRule struct {
	ID         string `json:"id"`
	Target     string `json:"target"`
	Severity   string `json:"severity"`
	Kind       string `json:"kind"`
	Pattern    string `json:"pattern"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// LoadCustomRules reads .novelgen/rules/*.json. A missing rules directory is
// not an error.
func LoadCustomRules(root *store.Root) ([]CustomRule, error) {
	if root == nil {
		return nil, store.ErrNilRoot
	}
	dir := root.RulesDir()
	exists, err := root.ExistsErr(dir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	rules := make([]CustomRule, 0)
	if err := root.Walk(dir, func(path string, info os.FileInfo) error {
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}
		data, err := root.Read(path)
		if err != nil {
			return err
		}
		parsed, err := parseCustomRules(data)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		rules = append(rules, parsed...)
		return nil
	}); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

func parseCustomRules(data []byte) ([]CustomRule, error) {
	var list []CustomRule
	if err := json.Unmarshal(data, &list); err == nil {
		return validateCustomRules(list)
	}
	var one CustomRule
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, fmt.Errorf("expected a rule object or array: %w", err)
	}
	return validateCustomRules([]CustomRule{one})
}

func validateCustomRules(rules []CustomRule) ([]CustomRule, error) {
	out := make([]CustomRule, 0, len(rules))
	for _, rule := range rules {
		rule.ID = strings.TrimSpace(rule.ID)
		if rule.ID == "" {
			return nil, fmt.Errorf("rule id is required")
		}
		rule.Target = normalizeCustomTarget(rule.Target)
		if rule.Target == "" {
			rule.Target = "all"
		}
		if rule.Target != "all" && rule.Target != "setup" && rule.Target != "outline" && rule.Target != "chapter" {
			return nil, fmt.Errorf("rule %q has unsupported target %q", rule.ID, rule.Target)
		}
		rule.Severity = normalizeCustomSeverity(rule.Severity)
		rule.Kind = strings.ToLower(strings.TrimSpace(rule.Kind))
		if rule.Kind == "" {
			rule.Kind = "regex"
		}
		if rule.Kind != "regex" && rule.Kind != "contains" {
			return nil, fmt.Errorf("rule %q has unsupported kind %q", rule.ID, rule.Kind)
		}
		if strings.TrimSpace(rule.Pattern) == "" {
			return nil, fmt.Errorf("rule %q has an empty pattern", rule.ID)
		}
		if rule.Kind == "regex" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return nil, fmt.Errorf("rule %q has an invalid pattern: %w", rule.ID, err)
			}
		}
		if strings.TrimSpace(rule.Message) == "" {
			rule.Message = "custom rule " + rule.ID + " matched"
		}
		out = append(out, rule)
	}
	return out, nil
}

// RunToolCustomRules evaluates project rules against one target.
func RunToolCustomRules(root *store.Root, target, scope, id string) (*ToolCheckResult, error) {
	target = normalizeCustomTarget(target)
	if target == "" {
		target = "outline"
	}
	if target != "setup" && target != "outline" && target != "chapter" {
		return nil, fmt.Errorf("custom rules support targets setup, outline, chapter; got %q", target)
	}
	rules, err := LoadCustomRules(root)
	if err != nil {
		return nil, err
	}
	text, err := customTargetText(root, target, id)
	if err != nil {
		return nil, err
	}
	result := &ToolCheckResult{
		Kind:   "custom",
		Target: target,
		Scope:  strings.TrimSpace(scope),
		ID:     strings.TrimSpace(id),
		OK:     true,
		Meta: map[string]interface{}{
			"custom_rule_count": len(rules),
			"project_root":      root.Dir(),
		},
	}
	for _, rule := range rules {
		if rule.Target != "all" && rule.Target != target {
			continue
		}
		matched := false
		if rule.Kind == "contains" {
			matched = strings.Contains(text, rule.Pattern)
		} else {
			matched = regexp.MustCompile(rule.Pattern).MatchString(text)
		}
		if !matched {
			continue
		}
		targetID := strings.TrimSpace(id)
		if targetID == "" {
			targetID = target
		}
		result.Issues = append(result.Issues, models.ReviewSuggestion{
			Category:   rule.ID,
			TargetID:   targetID,
			TargetName: target,
			Issue:      rule.Message,
			Suggestion: rule.Suggestion,
			Priority:   rule.Severity,
		})
	}
	result.Summary = summarizeCustomFindings(result.Issues)
	result.Blocking = result.Summary.Critical > 0 || result.Summary.High > 0
	result.OK = !result.Blocking
	result.Score = customScore(result.Summary)
	return result, nil
}

func customTargetText(root *store.Root, target, id string) (string, error) {
	if root == nil {
		return "", store.ErrNilRoot
	}
	switch target {
	case "setup":
		return readOptionalText(root, root.SetupJSON(), root.SetupMarkdown())
	case "outline":
		return readOptionalText(root, root.OutlineJSON())
	case "chapter":
		id = strings.TrimSpace(id)
		if id == "" {
			return "", fmt.Errorf("chapter custom checks require --id")
		}
		content, _, err := root.LoadChapterContent(id)
		if err != nil {
			return "", err
		}
		return content, nil
	default:
		return "", fmt.Errorf("unsupported custom target %q", target)
	}
}

func readOptionalText(root *store.Root, paths ...string) (string, error) {
	for _, path := range paths {
		exists, err := root.ExistsErr(path)
		if err != nil {
			return "", err
		}
		if !exists {
			continue
		}
		data, err := root.Read(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", fmt.Errorf("target artifact not found")
}

func summarizeCustomFindings(issues []models.ReviewSuggestion) ToolCheckSummary {
	summary := ToolCheckSummary{Total: len(issues)}
	for _, issue := range issues {
		switch normalizeCustomSeverity(issue.Priority) {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "low":
			summary.Low++
		default:
			summary.Medium++
		}
	}
	return summary
}

func customScore(summary ToolCheckSummary) float64 {
	score := 100.0 - float64(summary.Critical*20+summary.High*10+summary.Medium*4+summary.Low)
	if score < 0 {
		return 0
	}
	return score
}

func normalizeCustomTarget(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "story-setup", "storysetup":
		return "setup"
	default:
		return value
	}
}

func normalizeCustomSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
