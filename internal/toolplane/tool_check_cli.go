package toolplane

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/fiction/check"
	"novelgen/internal/fiction/domain/project"
)

// runToolCheck is the CLI adapter over the fiction/check engine. It owns flag
// validation, project resolution and JSON output; the deterministic checks and
// their navigation/result contract live in internal/fiction/check.
func runToolCheck(cmd *cobra.Command, args []string) error {
	kind := "all"
	if len(args) > 0 {
		kind = NormalizeKey(args[0])
	}
	if kind == "" {
		kind = "all"
	}
	if kind != "quality" && kind != "simulation" && kind != "all" && kind != "schema" && kind != "custom" {
		return fmt.Errorf("unsupported check kind %q; valid kinds: quality, simulation, all, schema, custom", kind)
	}
	target := NormalizeKey(ToolCheckFlags.Target)
	if target == "" {
		target = "outline"
	}
	if target != "outline" && target != "craft" && target != "setup" && target != "recap" && target != "chapter" {
		return fmt.Errorf("unsupported check target %q; valid targets: outline, setup, craft, chapter, recap", target)
	}
	if (target == "outline" || target == "setup") && kind == "schema" {
		return fmt.Errorf("kind schema is only supported for --target craft or --target recap, not --target %s; use quality/simulation/all instead", target)
	}
	if target == "craft" && kind != "schema" {
		return fmt.Errorf("craft checks currently support only kind schema, got %q", kind)
	}
	if target == "recap" && kind == "simulation" {
		return fmt.Errorf("recap checks do not support kind simulation; use quality, all, or schema")
	}
	if target == "chapter" && kind != "quality" && kind != "simulation" && kind != "all" && kind != "custom" {
		return fmt.Errorf("chapter checks currently support only kind quality, simulation, all, or custom")
	}

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root()
	if kind == "custom" {
		result, err := check.RunToolCustomRules(root, target, ToolCheckFlags.Scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "craft" {
		scope, err := resolveCraftCheckScope(root, ToolCheckFlags.Scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		result, err := check.RunToolCraftSchemaCheckAt(root, scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "recap" {
		result, err := check.RunToolRecapCheckAt(root, kind, ToolCheckFlags.Scope, ToolCheckFlags.ID)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "chapter" {
		result, err := check.RunToolChapterCheckWithTargetWordsAt(root, kind, ToolCheckFlags.Scope, ToolCheckFlags.ID, ToolCheckFlags.TargetWords)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}
	if target == "setup" {
		ctx := check.ProjectContext{Root: root}
		if err := ctx.LoadSetup(); err != nil {
			return err
		}
		result, err := check.RunToolSetupCheck(kind, ctx.Setup)
		if err != nil {
			return err
		}
		addToolCheckMeta(result, "project_root", root.Dir())
		applyToolCheckFlagWarnings(result, target)
		if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}

	ctx := check.ProjectContext{Root: root}
	if err := ctx.LoadSetup(); err != nil {
		return err
	}
	if err := ctx.LoadOutline(); err != nil {
		return err
	}

	result, err := check.RunToolOutlineCheckAt(root, kind, ctx.Setup, ctx.Outline, ToolCheckFlags.Scope, ToolCheckFlags.ID)
	if err != nil {
		return err
	}
	addToolCheckMeta(result, "project_root", root.Dir())
	applyToolCheckFlagWarnings(result, target)
	if err := check.ApplyToolCheckIssueFilters(result, ToolCheckFlags.MinPriority, ToolCheckFlags.Category, ToolCheckFlags.MaxIssues); err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func addToolCheckMeta(result *check.ToolCheckResult, key string, value interface{}) {
	if result == nil || strings.TrimSpace(key) == "" {
		return
	}
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta[key] = value
}

func applyToolCheckFlagWarnings(result *check.ToolCheckResult, target string) {
	if result == nil {
		return
	}
	add := func(message string) {
		result.Warnings = append(result.Warnings, message)
	}
	target = NormalizeKey(target)
	switch target {
	case "setup":
		if scope := check.NormalizeScope(ToolCheckFlags.Scope); scope != "all" {
			add(fmt.Sprintf("--scope %q is not used by setup checks and was ignored", ToolCheckFlags.Scope))
		}
		if strings.TrimSpace(ToolCheckFlags.ID) != "" {
			add("--id is not used by setup checks and was ignored")
		}
	case "outline":
		if check.NormalizeScope(ToolCheckFlags.Scope) == "all" && strings.TrimSpace(ToolCheckFlags.ID) != "" {
			add("--id is ignored when outline scope is all")
		}
	}
	if target != "chapter" && ToolCheckFlags.TargetWords > 0 {
		add("--target-words is only used by chapter checks and was ignored")
	}
}
