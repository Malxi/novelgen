package agents

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// CraftStorySetupSummary is a lightweight version of StorySetup for craft generation
type CraftStorySetupSummary struct {
	ProjectName    string   `json:"project_name" md:"project_name" desc:"Name of the novel project"`
	Genres         []string `json:"genres" md:"genres" desc:"List of story genres"`
	Premise        string   `json:"premise" md:"premise" desc:"Story premise and core concept"`
	Theme          string   `json:"theme" md:"theme" desc:"Central theme of the story"`
	Rules          []string `json:"rules" md:"rules" desc:"Story world rules and constraints"`
	CoreCast       []string `json:"core_cast,omitempty" md:"core_cast,omitempty" desc:"Setup-level core cast seeds"`
	Premises       []string `json:"premises,omitempty" md:"premises,omitempty" desc:"Ability/world systems with progression hints"`
	WorldResources []string `json:"world_resources,omitempty" md:"world_resources,omitempty" desc:"Core resources and scarcity constraints"`
	Storylines     []string `json:"storylines,omitempty" md:"storylines,omitempty" desc:"High-level story contracts and pressures"`
}

// CraftOutlineSummary is a lightweight version of Outline for craft generation
type CraftOutlineSummary struct {
	Parts []CraftPartSummary `json:"parts" md:"parts" desc:"Story parts containing volumes"`
}

// CraftPartSummary is a lightweight part summary
type CraftPartSummary struct {
	Title   string               `json:"title" md:"title" desc:"Part title"`
	Summary string               `json:"summary" md:"summary" desc:"Part summary"`
	Volumes []CraftVolumeSummary `json:"volumes" md:"volumes" desc:"Volumes in this part"`
}

// CraftVolumeSummary is a lightweight volume summary
type CraftVolumeSummary struct {
	Title    string                `json:"title" md:"title" desc:"Volume title"`
	Summary  string                `json:"summary" md:"summary" desc:"Volume summary"`
	Chapters []CraftChapterSummary `json:"chapters" md:"chapters" desc:"Chapters in this volume"`
}

// CraftChapterSummary is a lightweight chapter summary
type CraftChapterSummary struct {
	ID                string              `json:"id" md:"id" desc:"Chapter ID"`
	Title             string              `json:"title" md:"title" desc:"Chapter title"`
	Summary           string              `json:"summary" md:"summary" desc:"Chapter summary"`
	Characters        []string            `json:"characters" md:"characters" desc:"Characters appearing in this chapter"`
	Location          string              `json:"location" md:"location" desc:"Primary location of this chapter"`
	StateAnchor       string              `json:"state_anchor,omitempty" md:"state_anchor,omitempty" desc:"Start-of-chapter protagonist state"`
	Events            []CraftEventSummary `json:"events,omitempty" md:"events,omitempty" desc:"Typed events and state changes"`
	SceneLocations    []string            `json:"scene_locations,omitempty" md:"scene_locations,omitempty" desc:"Scene-level locations"`
	SceneCharacters   []string            `json:"scene_characters,omitempty" md:"scene_characters,omitempty" desc:"Scene-level characters"`
	Enemies           []string            `json:"enemies,omitempty" md:"enemies,omitempty" desc:"Enemy names and levels"`
	ResourceLedger    []string            `json:"resource_ledger,omitempty" md:"resource_ledger,omitempty" desc:"Tracked resource changes"`
	StorylineAdvances []string            `json:"storyline_advances,omitempty" md:"storyline_advances,omitempty" desc:"Storyline state advances"`
}

// CraftEventSummary is a compact event representation for craft prompts.
type CraftEventSummary struct {
	Actor      string `json:"actor,omitempty" md:"actor,omitempty" desc:"Event actor"`
	Action     string `json:"action,omitempty" md:"action,omitempty" desc:"Event action"`
	Target     string `json:"target,omitempty" md:"target,omitempty" desc:"Event target"`
	TargetType string `json:"target_type,omitempty" md:"target_type,omitempty" desc:"Target type for DSL state"`
	Context    string `json:"context,omitempty" md:"context,omitempty" desc:"Event context"`
	Result     string `json:"result,omitempty" md:"result,omitempty" desc:"Event result"`
}

// CraftGenCharactersInput is the input for character generation
type CraftGenCharactersInput struct {
	StorySetup       CraftStorySetupSummary `json:"story_setup" md:"story_setup" desc:"Story setup summary with premise, genres, theme, rules"`
	Outline          CraftOutlineSummary    `json:"outline" md:"outline" desc:"Outline summary with parts, volumes, chapters"`
	RelevantChapters []CraftChapterSummary  `json:"relevant_chapters" md:"relevant_chapters" desc:"Chapters where these characters appear"`
	Characters       []string               `json:"characters" md:"characters" desc:"List of character names to generate"`
	CustomPrompt     string                 `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for generation"`
}

// CraftGenCharactersOutput is the output for character generation
type CraftGenCharactersOutput struct {
	Characters map[string]models.Character `json:"characters" md:"characters" desc:"Generated character profiles keyed by name"`
}

// CraftAgentSDKCharactersPromptInput is the compact task contract for
// Agent SDK character generation. The SDK workflow must query project facts
// through novel tools instead of receiving the whole setup/outline.
type CraftAgentSDKCharactersPromptInput struct {
	Characters      []string `json:"characters" md:"characters" desc:"Character names to generate or complete"`
	CustomPrompt    string   `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional user guidance"`
	ApplyPatches    bool     `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch craft --apply after a successful dry-run"`
	RequiredQueries []string `json:"required_queries" md:"required_queries" desc:"Queries that must be executed before returning"`
	Instructions    []string `json:"instructions" md:"instructions" desc:"Workflow constraints"`
}

type CraftAgentSDKElementsPromptInput struct {
	Target          string   `json:"target" md:"target" desc:"Craft target: item, location, or organization"`
	OutputKey       string   `json:"output_key" md:"output_key" desc:"Required output object key: items, locations, or organizations"`
	Names           []string `json:"names" md:"names" desc:"Element names to generate or complete"`
	CustomPrompt    string   `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional user guidance"`
	ApplyPatches    bool     `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch craft --apply after a successful dry-run"`
	RequiredQueries []string `json:"required_queries" md:"required_queries" desc:"Queries that must be executed before returning"`
	Instructions    []string `json:"instructions" md:"instructions" desc:"Workflow constraints"`
}

type craftAgentSDKCharactersOutput struct {
	Characters map[string]models.Character `json:"characters" md:"characters" desc:"Generated or completed character profiles keyed by requested name"`
}

type craftAgentSDKLocationsOutput struct {
	Locations map[string]models.Location `json:"locations" md:"locations" desc:"Generated or completed locations keyed by requested name"`
}

type craftAgentSDKItemsOutput struct {
	Items map[string]models.Item `json:"items" md:"items" desc:"Generated or completed items keyed by requested name"`
}

type craftAgentSDKOrganizationsOutput struct {
	Organizations map[string]models.Organization `json:"organizations" md:"organizations" desc:"Generated or completed organizations keyed by requested name"`
}

// CraftGenLocationsInput is the input for location generation
type CraftGenLocationsInput struct {
	StorySetup       CraftStorySetupSummary `json:"story_setup" md:"story_setup" desc:"Story setup summary with premise, genres, theme, rules"`
	Outline          CraftOutlineSummary    `json:"outline" md:"outline" desc:"Outline summary with parts, volumes, chapters"`
	RelevantChapters []CraftChapterSummary  `json:"relevant_chapters" md:"relevant_chapters" desc:"Chapters where these locations appear"`
	Locations        []string               `json:"locations" md:"locations" desc:"List of location names to generate"`
	CustomPrompt     string                 `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for generation"`
}

// CraftGenLocationsOutput is the output for location generation
type CraftGenLocationsOutput struct {
	Locations map[string]models.Location `json:"locations" md:"locations" desc:"Generated location descriptions keyed by name"`
}

// CraftGenItemsInput is the input for item generation
type CraftGenItemsInput struct {
	StorySetup       CraftStorySetupSummary `json:"story_setup" md:"story_setup" desc:"Story setup summary with premise, genres, theme, rules"`
	Outline          CraftOutlineSummary    `json:"outline" md:"outline" desc:"Outline summary with parts, volumes, chapters"`
	RelevantChapters []CraftChapterSummary  `json:"relevant_chapters" md:"relevant_chapters" desc:"Chapters where these items appear"`
	Items            []string               `json:"items" md:"items" desc:"List of item names to generate"`
	CustomPrompt     string                 `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for generation"`
}

// CraftGenItemsOutput is the output for item generation
type CraftGenItemsOutput struct {
	Items map[string]models.Item `json:"items" md:"items" desc:"Generated item descriptions keyed by name"`
}

// CraftGenOrganizationsInput is the input for organization generation
type CraftGenOrganizationsInput struct {
	StorySetup       CraftStorySetupSummary `json:"story_setup" md:"story_setup" desc:"Story setup summary with premise, genres, theme, rules"`
	Outline          CraftOutlineSummary    `json:"outline" md:"outline" desc:"Outline summary with parts, volumes, chapters"`
	RelevantChapters []CraftChapterSummary  `json:"relevant_chapters" md:"relevant_chapters" desc:"Chapters where these organizations appear or exert pressure"`
	Organizations    []string               `json:"organizations" md:"organizations" desc:"List of organization or faction names to generate"`
	CustomPrompt     string                 `json:"custom_prompt,omitempty" md:"custom_prompt,omitempty" desc:"Optional custom prompt for generation"`
}

// CraftGenOrganizationsOutput is the output for organization generation
type CraftGenOrganizationsOutput struct {
	Organizations map[string]models.Organization `json:"organizations" md:"organizations" desc:"Generated organization descriptions keyed by name"`
}

// CraftAgent generates detailed story elements (characters, locations, items)
// It wraps BaseAgent to provide type-safe methods
type CraftAgent struct {
	base    *BaseAgent
	setup   *models.StorySetup
	outline *models.Outline
}

// NewCraftAgent creates a new CraftAgent
func NewCraftAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup, outline *models.Outline) *CraftAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "CraftAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &CraftAgent{
		base:    base,
		setup:   setup,
		outline: outline,
	}
}

// SetLanguage sets the output language
func (a *CraftAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// buildStorySetupSummary creates a lightweight summary of StorySetup
func (a *CraftAgent) buildStorySetupSummary() CraftStorySetupSummary {
	summary := CraftStorySetupSummary{
		ProjectName: a.setup.ProjectName,
		Genres:      a.setup.Genres,
		Premise:     a.setup.Premise,
		Theme:       a.setup.Theme,
		Rules:       a.setup.Rules,
	}
	for _, premise := range a.setup.Premises {
		parts := []string{premise.Name, premise.Category, premise.Description}
		if len(premise.Progression) > 0 {
			parts = append(parts, fmt.Sprintf("progression_levels=%d", len(premise.Progression)))
		}
		summary.Premises = append(summary.Premises, joinNonEmpty(parts, " | "))
	}
	for _, resource := range a.setup.WorldResources {
		summary.WorldResources = append(summary.WorldResources, joinNonEmpty([]string{
			resource.Name,
			resource.Category,
			resource.Scarcity,
			resource.Description,
		}, " | "))
	}
	for _, seed := range a.setup.CoreCast {
		summary.CoreCast = append(summary.CoreCast, joinNonEmpty([]string{
			seed.Name,
			seed.Role,
			fmt.Sprintf("importance=%d", seed.Importance),
			seed.StoryFunction,
			seed.RelationshipArc,
			seed.EntryPhase,
			seed.Payoff,
		}, " | "))
	}
	for _, storyline := range a.setup.Storylines {
		summary.Storylines = append(summary.Storylines, joinNonEmpty([]string{
			storyline.Name,
			storyline.Type,
			fmt.Sprintf("importance=%d", storyline.Importance),
			storyline.SetupRole,
			storyline.Desire,
			storyline.Opposition,
			storyline.Stakes,
			storyline.OpenQuestion,
		}, " | "))
	}
	return summary
}

// buildOutlineSummary creates a lightweight summary of Outline
func (a *CraftAgent) buildOutlineSummary() CraftOutlineSummary {
	var parts []CraftPartSummary
	for _, part := range a.outline.Parts {
		partSummary := CraftPartSummary{
			Title:   part.Title,
			Summary: part.Summary,
		}
		for _, vol := range part.Volumes {
			volSummary := CraftVolumeSummary{
				Title:   vol.Title,
				Summary: vol.Summary,
			}
			for _, ch := range vol.Chapters {
				volSummary.Chapters = append(volSummary.Chapters, buildCraftChapterSummary(ch))
			}
			partSummary.Volumes = append(partSummary.Volumes, volSummary)
		}
		parts = append(parts, partSummary)
	}
	return CraftOutlineSummary{Parts: parts}
}

// GenerateCharacters generates detailed character profiles
func (a *CraftAgent) GenerateCharacters(ctx context.Context, names []string, customPrompt string) (map[string]models.Character, error) {
	logger.Section("CRAFT AGENT - Character Generation")
	logger.Info("Characters: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these characters appear
	relevantChapters := a.findChaptersWithCharacters(names)
	logger.Info("Found %d relevant chapters for these characters", len(relevantChapters))

	input := CraftGenCharactersInput{
		StorySetup:       a.buildStorySetupSummary(),
		Outline:          a.buildOutlineSummary(),
		RelevantChapters: relevantChapters,
		Characters:       names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenCharactersOutput
	params := InvokeParams{
		Skills:  []string{"craft-characters"},
		Command: "generate detailed character profiles",
	}

	if err := a.base.Execute(ctx, params, input, &output.Characters); err != nil {
		return nil, err
	}

	output.Characters = normalizeGeneratedCharacters(names, output.Characters)

	logger.Info("✓ Generated %d characters", len(output.Characters))
	return output.Characters, nil
}

// GenerateCharactersWithAgentSDK asks the Claude Agent SDK workflow to gather
// context with novel tools, dry-run craft patches, and return typed character
// cards. Go still validates, normalizes, and saves through the caller.
func (a *CraftAgent) GenerateCharactersWithAgentSDK(ctx context.Context, names []string, customPrompt string, applyPatches bool) (map[string]models.Character, error) {
	logger.Section("CRAFT AGENT SDK - Character Generation")
	logger.Info("Characters: %v", names)

	var output craftAgentSDKCharactersOutput
	promptInput := buildCraftAgentSDKCharactersPromptInput(names, customPrompt, applyPatches)
	params := craftAgentSDKParams("generate character craft using focused project query and patch tools", "craft-character-workflow", "character", 16, applyPatches, names, promptInput.RequiredQueries)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return nil, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(output.Characters); err != nil {
		return nil, fmt.Errorf("agent SDK character craft rejected: %w", err)
	}
	output.Characters = normalizeGeneratedCharacters(names, output.Characters)
	for name, character := range output.Characters {
		if strings.TrimSpace(character.Name) == "" {
			return nil, fmt.Errorf("agent SDK character %q is missing name", name)
		}
	}
	logger.Info("[ok] Agent SDK generated %d characters", len(output.Characters))
	return output.Characters, nil
}

// GenerateLocationsWithAgentSDK asks the Claude Agent SDK workflow to gather
// context with novel tools, dry-run craft patches, and return typed locations.
func (a *CraftAgent) GenerateLocationsWithAgentSDK(ctx context.Context, names []string, customPrompt string, applyPatches bool) (map[string]models.Location, error) {
	logger.Section("CRAFT AGENT SDK - Location Generation")
	logger.Info("Locations: %v", names)

	var output craftAgentSDKLocationsOutput
	promptInput := buildCraftAgentSDKElementsPromptInput("location", "locations", names, customPrompt, applyPatches)
	params := craftAgentSDKParams("generate location craft using focused project query and patch tools", "craft-element-workflow", "location", 16, applyPatches, names, promptInput.RequiredQueries)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return nil, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(output.Locations); err != nil {
		return nil, fmt.Errorf("agent SDK location craft rejected: %w", err)
	}
	output.Locations = normalizeGeneratedLocations(names, output.Locations)
	for name, location := range output.Locations {
		if strings.TrimSpace(location.Name) == "" {
			return nil, fmt.Errorf("agent SDK location %q is missing name", name)
		}
	}
	logger.Info("[ok] Agent SDK generated %d locations", len(output.Locations))
	return output.Locations, nil
}

// GenerateItemsWithAgentSDK asks the Claude Agent SDK workflow to gather
// context with novel tools, dry-run craft patches, and return typed items.
func (a *CraftAgent) GenerateItemsWithAgentSDK(ctx context.Context, names []string, customPrompt string, applyPatches bool) (map[string]models.Item, error) {
	logger.Section("CRAFT AGENT SDK - Item Generation")
	logger.Info("Items: %v", names)

	var output craftAgentSDKItemsOutput
	promptInput := buildCraftAgentSDKElementsPromptInput("item", "items", names, customPrompt, applyPatches)
	params := craftAgentSDKParams("generate item craft using focused project query and patch tools", "craft-element-workflow", "item", 16, applyPatches, names, promptInput.RequiredQueries)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return nil, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(output.Items); err != nil {
		return nil, fmt.Errorf("agent SDK item craft rejected: %w", err)
	}
	output.Items = normalizeGeneratedItems(names, output.Items)
	for name, item := range output.Items {
		if strings.TrimSpace(item.Name) == "" {
			return nil, fmt.Errorf("agent SDK item %q is missing name", name)
		}
	}
	logger.Info("[ok] Agent SDK generated %d items", len(output.Items))
	return output.Items, nil
}

// GenerateOrganizationsWithAgentSDK asks the Claude Agent SDK workflow to gather
// context with novel tools, dry-run craft patches, and return typed organizations.
func (a *CraftAgent) GenerateOrganizationsWithAgentSDK(ctx context.Context, names []string, customPrompt string, applyPatches bool) (map[string]models.Organization, error) {
	logger.Section("CRAFT AGENT SDK - Organization Generation")
	logger.Info("Organizations: %v", names)

	var output craftAgentSDKOrganizationsOutput
	promptInput := buildCraftAgentSDKElementsPromptInput("organization", "organizations", names, customPrompt, applyPatches)
	params := craftAgentSDKParams("generate organization craft using focused project query and patch tools", "craft-element-workflow", "organization", 16, applyPatches, names, promptInput.RequiredQueries)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return nil, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(output.Organizations); err != nil {
		return nil, fmt.Errorf("agent SDK organization craft rejected: %w", err)
	}
	output.Organizations = normalizeGeneratedOrganizations(names, output.Organizations)
	for name, organization := range output.Organizations {
		if strings.TrimSpace(organization.Name) == "" {
			return nil, fmt.Errorf("agent SDK organization %q is missing name", name)
		}
	}
	logger.Info("[ok] Agent SDK generated %d organizations", len(output.Organizations))
	return output.Organizations, nil
}

func craftAgentSDKParams(command, workflowSkill, target string, maxTurns int, applyPatches bool, names []string, requiredQueries []string) InvokeParams {
	if maxTurns <= 0 {
		maxTurns = 12
	}
	target = normalizeCraftAgentSDKTarget(target)
	allowlist := craftAgentSDKToolAllowlist(target, names, requiredQueries, applyPatches)
	evidence := ToolEvidenceRequirement{}
	if len(requiredQueries) > 0 {
		evidence.MinContextQueryCalls = 1
	}
	if len(names) > 0 {
		evidence.MinCheckCalls = 1
	}
	if applyPatches {
		evidence.MinPatchApplyCalls = 1
		evidence.RequirePatchApplyFollowupCheck = true
	}
	return InvokeParams{
		SDKSkills:      []string{"novel-tools-core", workflowSkill},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  allowlist,
		ToolEvidence:   evidence,
		MaxTurns:       maxTurns,
		Timeout:        900,
		Command:        command,
	}
}

func craftAgentSDKToolAllowlist(target string, names []string, requiredQueries []string, applyPatches bool) []string {
	target = normalizeCraftAgentSDKTarget(target)
	seen := map[string]bool{}
	allowlist := []string{}
	add := func(command string) {
		command = strings.TrimSpace(command)
		if command == "" || seen[command] {
			return
		}
		seen[command] = true
		allowlist = append(allowlist, command)
	}
	for _, query := range requiredQueries {
		add(query)
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		add(fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, name))
		patchTool := fmt.Sprintf("novelgen tool patch craft --target %s --id %q", target, name)
		if applyPatches {
			patchTool += " --apply"
		}
		add(patchTool)
	}
	return allowlist
}

func normalizeCraftAgentSDKTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "characters", "character", "":
		return "character"
	case "items", "item":
		return "item"
	case "locations", "location":
		return "location"
	case "organizations", "organization", "org":
		return "organization"
	default:
		return strings.ToLower(strings.TrimSpace(target))
	}
}

func buildCraftAgentSDKCharactersPromptInput(names []string, customPrompt string, applyPatches bool) CraftAgentSDKCharactersPromptInput {
	required := []string{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		required = append(required, fmt.Sprintf("novelgen tool query context --type craft-character --name %q --view brief", name))
	}
	return CraftAgentSDKCharactersPromptInput{
		Characters:      append([]string(nil), names...),
		CustomPrompt:    customPrompt,
		ApplyPatches:    applyPatches,
		RequiredQueries: required,
		Instructions: []string{
			"Execute required_queries first. Use tool outputs as source of truth.",
			"The context query returns setup, existing craft, outline refs, relevant chapters, events, stats, and navigation. Do not run extra project queries outside required_queries; if context is insufficient, keep uncertain fields conservative and note the limitation in notes.",
			craftAgentSDKPatchInstruction(applyPatches),
			craftAgentSDKCheckInstruction(applyPatches),
			craftAgentSDKCharacterFieldInstruction(),
			"When emitting rpg_stats, only use supported keys: str, agi, int, vit, hp, mp, level.",
			"Return only the requested characters. Do not invent unrelated characters.",
			craftAgentSDKWriteInstruction(applyPatches),
		},
	}
}

func buildCraftAgentSDKElementsPromptInput(target, outputKey string, names []string, customPrompt string, applyPatches bool) CraftAgentSDKElementsPromptInput {
	target = normalizeCraftAgentSDKTarget(target)
	required := []string{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		required = append(required, fmt.Sprintf("novelgen tool query context --type craft-%s --name %q --view brief", target, name))
	}
	return CraftAgentSDKElementsPromptInput{
		Target:          target,
		OutputKey:       outputKey,
		Names:           append([]string(nil), names...),
		CustomPrompt:    customPrompt,
		ApplyPatches:    applyPatches,
		RequiredQueries: required,
		Instructions: []string{
			"Execute required_queries first. Use tool outputs as source of truth.",
			"The context query returns setup, existing craft, outline refs or events, relevant chapters, stats, and navigation. Do not run extra project queries outside required_queries; if context is insufficient, keep uncertain fields conservative and note the limitation in notes.",
			craftAgentSDKTargetPatchInstruction(target, applyPatches),
			craftAgentSDKTargetCheckInstruction(target, applyPatches),
			craftAgentSDKTargetFieldInstruction(target),
			fmt.Sprintf("Return only the requested %s entries under the JSON key %q. Do not invent unrelated elements.", target, outputKey),
			craftAgentSDKTargetWriteInstruction(target, applyPatches),
		},
	}
}

func craftAgentSDKPatchInstruction(applyPatches bool) string {
	return craftAgentSDKTargetPatchInstruction("character", applyPatches)
}

func craftAgentSDKTargetPatchInstruction(target string, applyPatches bool) string {
	target = normalizeCraftAgentSDKTarget(target)
	jsonInput := fmt.Sprintf(" For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. Pipe compact literal JSON on stdin instead: `printf '%%s' '<compact-json>' | novelgen tool patch craft --target %s --id <name>`. Use --patch-json only for small ASCII-only patches.", target)
	if applyPatches {
		return fmt.Sprintf("Before final JSON, first dry-run each changed or created %s with `printf '%%s' '<compact-json>' | novelgen tool patch craft --target %s --id <name>`; only after a successful dry-run, repeat the same stdin-piped patch command with `--apply`.%s", target, target, jsonInput)
	}
	return fmt.Sprintf("Before final JSON, dry-run each changed or created %s with `printf '%%s' '<compact-json>' | novelgen tool patch craft --target %s --id <name>` and do not use --apply.%s", target, target, jsonInput)
}

func craftAgentSDKCheckInstruction(applyPatches bool) string {
	return craftAgentSDKTargetCheckInstruction("character", applyPatches)
}

func craftAgentSDKTargetCheckInstruction(target string, applyPatches bool) string {
	target = normalizeCraftAgentSDKTarget(target)
	if applyPatches {
		return fmt.Sprintf("After each successful --apply, run `novelgen tool check schema --target craft --scope %s --id <name>` and fix any blocking schema issue with another dry-run/apply cycle before returning.", target)
	}
	return "Use the schema result embedded in craft patch dry-run output as the validation signal."
}

func craftAgentSDKTargetFieldInstruction(target string) string {
	switch normalizeCraftAgentSDKTarget(target) {
	case "item":
		return "Each item should include name, type, description, appearance, function, and significance. Use rarity/power_level conservatively and only when supported by project facts."
	case "location":
		return "Each location should include name, type, description, appearance, atmosphere, and significance. Use danger_level/resource_tags/encounter_tags conservatively."
	case "organization":
		return "Each organization should include name, type, description, goals, and significance. Keep members/resources/allies/enemies grounded in queried facts."
	default:
		return "When emitting rpg_stats, only use supported keys: str, agi, int, vit, hp, mp, level."
	}
}

func craftAgentSDKCharacterFieldInstruction() string {
	return "Distribute character craft into typed fields instead of dumping it into notes. For protagonist/lead/player characters, ensure personality has 3-6 decision-shaping traits, motivation is concrete, skills cover mundane/tactical competencies, abilities cover special powers with limits, voice gives drafting guidance, and notes stays concise for writer-only continuity constraints."
}

func craftAgentSDKWriteInstruction(applyPatches bool) string {
	return craftAgentSDKTargetWriteInstruction("character", applyPatches)
}

func craftAgentSDKTargetWriteInstruction(target string, applyPatches bool) string {
	target = normalizeCraftAgentSDKTarget(target)
	if applyPatches {
		return fmt.Sprintf("Do not inspect source code or write files directly; the only allowed write is `novelgen tool patch craft --target %s ... --apply` after dry-run validation.", target)
	}
	return "Go will normalize and save; you must not write files or inspect source code."
}

// GenerateLocations generates detailed location descriptions
func (a *CraftAgent) GenerateLocations(ctx context.Context, names []string, customPrompt string) (map[string]models.Location, error) {
	logger.Section("CRAFT AGENT - Location Generation")
	logger.Info("Locations: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these locations appear
	relevantChapters := a.findChaptersWithLocations(names)
	logger.Info("Found %d relevant chapters for these locations", len(relevantChapters))

	input := CraftGenLocationsInput{
		StorySetup:       a.buildStorySetupSummary(),
		Outline:          a.buildOutlineSummary(),
		RelevantChapters: relevantChapters,
		Locations:        names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenLocationsOutput
	params := InvokeParams{
		Skills:  []string{"craft-locations"},
		Command: "generate detailed location descriptions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Locations); err != nil {
		return nil, err
	}

	output.Locations = normalizeGeneratedLocations(names, output.Locations)

	logger.Info("✓ Generated %d locations", len(output.Locations))
	return output.Locations, nil
}

// GenerateItems generates detailed item descriptions
func (a *CraftAgent) GenerateItems(ctx context.Context, names []string, customPrompt string) (map[string]models.Item, error) {
	logger.Section("CRAFT AGENT - Item Generation")
	logger.Info("Items: %v", names)
	logger.Info("Language: %s", a.base.language)

	// Find chapters where these items appear
	relevantChapters := a.findChaptersWithItems(names)
	logger.Info("Found %d relevant chapters for these items", len(relevantChapters))

	input := CraftGenItemsInput{
		StorySetup:       a.buildStorySetupSummary(),
		Outline:          a.buildOutlineSummary(),
		RelevantChapters: relevantChapters,
		Items:            names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenItemsOutput
	params := InvokeParams{
		Skills:  []string{"craft-items"},
		Command: "generate detailed item descriptions",
	}

	if err := a.base.Execute(ctx, params, input, &output.Items); err != nil {
		return nil, err
	}

	output.Items = normalizeGeneratedItems(names, output.Items)

	logger.Info("✓ Generated %d items", len(output.Items))
	return output.Items, nil
}

// GenerateOrganizations generates detailed organization descriptions
func (a *CraftAgent) GenerateOrganizations(ctx context.Context, names []string, customPrompt string) (map[string]models.Organization, error) {
	logger.Section("CRAFT AGENT - Organization Generation")
	logger.Info("Organizations: %v", names)
	logger.Info("Language: %s", a.base.language)

	relevantChapters := a.findChaptersWithOrganizations(names)
	logger.Info("Found %d relevant chapters for these organizations", len(relevantChapters))

	input := CraftGenOrganizationsInput{
		StorySetup:       a.buildStorySetupSummary(),
		Outline:          a.buildOutlineSummary(),
		RelevantChapters: relevantChapters,
		Organizations:    names,
		CustomPrompt:     customPrompt,
	}

	var output CraftGenOrganizationsOutput
	params := InvokeParams{
		Skills:  []string{"craft-organizations"},
		Command: "generate detailed organization profiles",
	}

	if err := a.base.Execute(ctx, params, input, &output.Organizations); err != nil {
		return nil, err
	}

	output.Organizations = normalizeGeneratedOrganizations(names, output.Organizations)

	logger.Info("[ok] Generated %d organizations", len(output.Organizations))
	return output.Organizations, nil
}

// findChaptersWithCharacters finds chapters that mention the given characters.
func (a *CraftAgent) findChaptersWithCharacters(characterNames []string) []CraftChapterSummary {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []CraftChapterSummary
	nameSet := make(map[string]bool)
	for _, name := range characterNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if chapterHasCharacter(ch, nameSet) {
					relevantChapters = append(relevantChapters, buildCraftChapterSummary(ch))
				}
			}
		}
	}

	return relevantChapters
}

// findChaptersWithLocations finds chapters that mention the given locations
func (a *CraftAgent) findChaptersWithLocations(locationNames []string) []CraftChapterSummary {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []CraftChapterSummary
	nameSet := make(map[string]bool)
	for _, name := range locationNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if chapterHasLocation(ch, nameSet) {
					relevantChapters = append(relevantChapters, buildCraftChapterSummary(ch))
				}
			}
		}
	}

	return relevantChapters
}

// findChaptersWithItems finds chapters that mention the given items
func (a *CraftAgent) findChaptersWithItems(itemNames []string) []CraftChapterSummary {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []CraftChapterSummary
	nameSet := make(map[string]bool)
	for _, name := range itemNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if chapterHasItem(ch, nameSet) {
					relevantChapters = append(relevantChapters, buildCraftChapterSummary(ch))
				}
			}
		}
	}

	return relevantChapters
}

// findChaptersWithOrganizations finds chapters that mention the given organizations
func (a *CraftAgent) findChaptersWithOrganizations(organizationNames []string) []CraftChapterSummary {
	if a.outline == nil {
		return nil
	}

	var relevantChapters []CraftChapterSummary
	nameSet := make(map[string]bool)
	for _, name := range organizationNames {
		nameSet[name] = true
	}

	for _, part := range a.outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				if chapterHasOrganization(ch, nameSet) {
					relevantChapters = append(relevantChapters, buildCraftChapterSummary(ch))
				}
			}
		}
	}

	return relevantChapters
}

func buildCraftChapterSummary(ch models.Chapter) CraftChapterSummary {
	summary := CraftChapterSummary{
		ID:         ch.ID,
		Title:      ch.Title,
		Summary:    ch.Summary,
		Characters: append([]string(nil), ch.Characters...),
		Location:   ch.Location,
	}
	summary.StateAnchor = formatStateAnchor(ch.StateAnchor)
	for _, event := range ch.Events {
		summary.Events = append(summary.Events, CraftEventSummary{
			Actor:      event.GetActor(),
			Action:     event.GetAction(),
			Target:     event.GetTarget(),
			TargetType: event.GetTargetType(),
			Context:    event.Context,
			Result:     coalesceText(event.Result, event.Change, event.Details),
		})
	}
	for _, scene := range ch.Scenes {
		if scene.Location != "" {
			summary.SceneLocations = append(summary.SceneLocations, scene.Location)
		}
		summary.SceneCharacters = append(summary.SceneCharacters, scene.Characters...)
	}
	for _, enemy := range ch.Enemies {
		summary.Enemies = append(summary.Enemies, joinNonEmpty([]string{
			enemy.Name,
			enemy.Faction,
			enemy.Tier,
			fmt.Sprintf("level=%d", enemy.Level),
			fmt.Sprintf("count=%d", enemy.Count),
		}, " | "))
	}
	for _, entry := range ch.ResourceLedger {
		summary.ResourceLedger = append(summary.ResourceLedger,
			fmt.Sprintf("%s: %d %+d = %d (%s)", entry.Item, entry.Start, entry.Delta, entry.End, entry.Reason))
	}
	for _, advance := range ch.StorylineAdvances {
		summary.StorylineAdvances = append(summary.StorylineAdvances, joinNonEmpty([]string{
			advance.StorylineName,
			advance.Stage,
			advance.Change,
			advance.Consequence,
			advance.Pressure,
		}, " | "))
	}
	summary.SceneLocations = compactStrings(summary.SceneLocations)
	summary.SceneCharacters = compactStrings(summary.SceneCharacters)
	return summary
}

func chapterHasCharacter(ch models.Chapter, names map[string]bool) bool {
	for _, name := range ch.Characters {
		if names[name] {
			return true
		}
	}
	for _, scene := range ch.Scenes {
		for _, name := range scene.Characters {
			if names[name] {
				return true
			}
		}
		if names[scene.POV] {
			return true
		}
	}
	for _, event := range ch.Events {
		if names[event.GetActor()] || (event.GetTargetType() == models.TargetTypeCharacter && names[event.GetTarget()]) {
			return true
		}
		for _, name := range event.Characters {
			if names[name] {
				return true
			}
		}
	}
	for _, ally := range ch.StateAnchor.Allies {
		if names[ally] {
			return true
		}
	}
	for _, enemy := range ch.Enemies {
		if names[enemy.Name] {
			return true
		}
	}
	return false
}

func chapterHasLocation(ch models.Chapter, names map[string]bool) bool {
	if names[ch.Location] || names[ch.StateAnchor.Location] {
		return true
	}
	for _, scene := range ch.Scenes {
		if names[scene.Location] {
			return true
		}
	}
	for _, event := range ch.Events {
		if event.GetTargetType() == models.TargetTypeLocation && names[event.GetTarget()] {
			return true
		}
		if names[event.Context] {
			return true
		}
	}
	content := ch.Title + " " + ch.Summary
	for name := range names {
		if strings.Contains(content, name) {
			return true
		}
	}
	return false
}

func chapterHasItem(ch models.Chapter, names map[string]bool) bool {
	for _, item := range ch.StateAnchor.KeyItems {
		if names[item] {
			return true
		}
	}
	for _, entry := range ch.ResourceLedger {
		if names[entry.Item] {
			return true
		}
	}
	for _, event := range ch.Events {
		if event.GetTargetType() == models.TargetTypeItem && names[event.GetTarget()] {
			return true
		}
		if event.Type == models.EventTypeItem && names[event.GetTarget()] {
			return true
		}
	}
	content := ch.Title + " " + ch.Summary
	for name := range names {
		if strings.Contains(content, name) {
			return true
		}
	}
	return false
}

func chapterHasOrganization(ch models.Chapter, names map[string]bool) bool {
	for _, enemy := range ch.Enemies {
		if names[enemy.Faction] {
			return true
		}
	}
	for _, advance := range ch.StorylineAdvances {
		if names[advance.StorylineName] {
			return true
		}
	}
	for _, event := range ch.Events {
		if names[event.Context] || names[event.GetTarget()] || names[event.GetActor()] {
			return true
		}
	}
	content := ch.Title + " " + ch.Summary + " " + ch.Conflict
	for name := range names {
		if strings.Contains(content, name) {
			return true
		}
	}
	return false
}

func normalizeGeneratedCharacters(requested []string, generated map[string]models.Character) map[string]models.Character {
	out := make(map[string]models.Character, len(generated))
	for _, name := range requested {
		char, ok := generated[name]
		if !ok {
			for key, candidate := range generated {
				if candidate.Name == name {
					char = candidate
					delete(generated, key)
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		char.NormalizeForCraft(name)
		out[name] = char
	}
	if len(requested) > 0 {
		return out
	}
	for name, char := range generated {
		char.NormalizeForCraft(name)
		out[name] = char
	}
	return out
}

func normalizeGeneratedOrganizations(requested []string, generated map[string]models.Organization) map[string]models.Organization {
	out := make(map[string]models.Organization, len(generated))
	for _, name := range requested {
		org, ok := generated[name]
		if !ok {
			for key, candidate := range generated {
				if candidate.Name == name {
					org = candidate
					delete(generated, key)
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		org.NormalizeForCraft(name)
		out[name] = org
	}
	if len(requested) > 0 {
		return out
	}
	for name, org := range generated {
		org.NormalizeForCraft(name)
		out[name] = org
	}
	return out
}

func normalizeGeneratedLocations(requested []string, generated map[string]models.Location) map[string]models.Location {
	out := make(map[string]models.Location, len(generated))
	for _, name := range requested {
		loc, ok := generated[name]
		if !ok {
			for key, candidate := range generated {
				if candidate.Name == name {
					loc = candidate
					delete(generated, key)
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		loc.NormalizeForCraft(name)
		out[name] = loc
	}
	if len(requested) > 0 {
		return out
	}
	for name, loc := range generated {
		loc.NormalizeForCraft(name)
		out[name] = loc
	}
	return out
}

func normalizeGeneratedItems(requested []string, generated map[string]models.Item) map[string]models.Item {
	out := make(map[string]models.Item, len(generated))
	for _, name := range requested {
		item, ok := generated[name]
		if !ok {
			for key, candidate := range generated {
				if candidate.Name == name {
					item = candidate
					delete(generated, key)
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		item.NormalizeForCraft(name)
		out[name] = item
	}
	if len(requested) > 0 {
		return out
	}
	for name, item := range generated {
		item.NormalizeForCraft(name)
		out[name] = item
	}
	return out
}

func formatStateAnchor(anchor models.StateAnchor) string {
	return joinNonEmpty([]string{
		"cultivation=" + anchor.Cultivation,
		fmt.Sprintf("spirit_stones=%d", anchor.SpiritStones),
		"location=" + anchor.Location,
		"allies=" + strings.Join(anchor.Allies, ", "),
		"injuries=" + strings.Join(anchor.Injuries, ", "),
		"key_items=" + strings.Join(anchor.KeyItems, ", "),
		anchor.Notes,
	}, "; ")
}

func joinNonEmpty(values []string, sep string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "level=0" && value != "count=0" && value != "spirit_stones=0" {
			out = append(out, value)
		}
	}
	return strings.Join(out, sep)
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func coalesceText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
