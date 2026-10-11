package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"novelgen/internal/rpg/dsl"
)

type goldenSpec struct {
	ExpectedIssues []goldenExpectedIssue `json:"expected_issues"`
}

type goldenExpectedIssue struct {
	ID                  string   `json:"id"`
	MustDetect          bool     `json:"must_detect"`
	Chapter             string   `json:"chapter,omitempty"`
	Type                string   `json:"type,omitempty"`
	Severity            string   `json:"severity,omitempty"`
	DescriptionContains []string `json:"description_contains,omitempty"`
}

type goldenEvaluation struct {
	Path          string        `json:"path"`
	Expected      int           `json:"expected"`
	Detected      int           `json:"detected"`
	Missed        int           `json:"missed"`
	Recall        float64       `json:"recall"`
	Matches       []goldenMatch `json:"matches,omitempty"`
	MissingIssues []string      `json:"missing_issues,omitempty"`
}

type goldenMatch struct {
	ExpectedID  string `json:"expected_id"`
	Chapter     string `json:"chapter,omitempty"`
	Type        string `json:"type,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Description string `json:"description,omitempty"`
}

func loadGoldenSpec(path string) (*goldenSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec goldenSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func evaluateGolden(path string, spec *goldenSpec, issues []dsl.SimulationIssue) goldenEvaluation {
	eval := goldenEvaluation{Path: path}
	if spec == nil {
		return eval
	}

	for _, expected := range spec.ExpectedIssues {
		if !expected.MustDetect {
			continue
		}
		eval.Expected++
		if issue, ok := findGoldenIssueMatch(expected, issues); ok {
			eval.Detected++
			eval.Matches = append(eval.Matches, goldenMatch{
				ExpectedID:  expected.ID,
				Chapter:     issue.Chapter,
				Type:        string(issue.Type),
				Severity:    string(issue.Severity),
				Description: issue.Description,
			})
		} else {
			eval.Missed++
			eval.MissingIssues = append(eval.MissingIssues, expected.ID)
		}
	}
	if eval.Expected > 0 {
		eval.Recall = float64(eval.Detected) / float64(eval.Expected)
	}
	return eval
}

func findGoldenIssueMatch(expected goldenExpectedIssue, issues []dsl.SimulationIssue) (dsl.SimulationIssue, bool) {
	for _, issue := range issues {
		if expected.Chapter != "" && issue.Chapter != expected.Chapter {
			continue
		}
		if expected.Type != "" && string(issue.Type) != expected.Type {
			continue
		}
		if expected.Severity != "" && string(issue.Severity) != expected.Severity {
			continue
		}
		if !containsAll(issue.Description, expected.DescriptionContains) {
			continue
		}
		return issue, true
	}
	return dsl.SimulationIssue{}, false
}

func containsAll(text string, needles []string) bool {
	for _, needle := range needles {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		if !strings.Contains(text, needle) {
			return false
		}
	}
	return true
}

func formatGoldenSummary(eval *goldenEvaluation) string {
	if eval == nil || eval.Expected == 0 {
		return ""
	}
	return fmt.Sprintf("Golden benchmark: detected=%d/%d missed=%d recall=%.2f",
		eval.Detected, eval.Expected, eval.Missed, eval.Recall)
}
