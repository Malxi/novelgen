package logic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"
)

// StateMatrixManager handles StateMatrix calculations and operations
type StateMatrixManager struct {
	projectRoot        string
	setup              *models.StorySetup // cached setup for storyline descriptions
	useRPGDSL          bool
	rpgDeltasLoaded    bool
	rpgDeltasByChapter map[string][]models.RPGStateDelta
	rpgTargetAliases   map[string]string
	rpgPlayerID        string
}

// NewStateMatrixManager creates a new StateMatrixManager
func NewStateMatrixManager(projectRoot string) *StateMatrixManager {
	return &StateMatrixManager{projectRoot: projectRoot, useRPGDSL: true}
}

// SetUseRPGDSL controls whether generated RPG DSL state_delta facts are folded into RPGState.
func (m *StateMatrixManager) SetUseRPGDSL(enabled bool) {
	m.useRPGDSL = enabled
}

// loadSetup loads the story setup from file
func (m *StateMatrixManager) loadSetup() *models.StorySetup {
	if m.setup != nil {
		return m.setup
	}
	if m.projectRoot == "" {
		return nil
	}

	setupPath := filepath.Join(m.projectRoot, "story", "setup", "story_setup.json")
	data, err := os.ReadFile(setupPath)
	if err != nil {
		return nil
	}

	var setup models.StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil
	}

	m.setup = &setup
	return m.setup
}

// CalculateStateMatrix calculates the story state up to the target chapter
func (m *StateMatrixManager) CalculateStateMatrix(outline *models.Outline, targetChapter *models.Chapter) *models.StateMatrix {
	state := &models.StateMatrix{
		Characters:    make(map[string]*models.Character),
		Locations:     make(map[string]*models.Location),
		Items:         make(map[string]*models.Item),
		Relationships: make(map[string]string),
		Goals:         make(map[string][]string),
		Storylines:    make(map[string]*models.StorylineState),
		Premises:      make(map[string]string),
		Gates:         make(map[string]*models.GateState),
		Status:        make(map[string]*models.StatusState),
		Memories:      make(map[string][]*models.MemoryState),
		RPG:           newRPGState(),
	}

	// Load all generated elements
	m.loadElementsIntoState(state)

	// Apply events from all chapters up to and including target
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				// Apply events from this chapter
				for _, event := range ch.Events {
					m.applyEvent(state, event, ch.ID)
				}
				m.applyRPGDSLDeltasForChapter(state, ch.ID)

				// Stop when we reach target chapter (after processing it)
				if ch.ID == targetChapter.ID {
					return state
				}
			}
		}
	}

	return state
}

// CalculateRPGState calculates only the structured RPG state up to the target chapter.
func (m *StateMatrixManager) CalculateRPGState(outline *models.Outline, targetChapter *models.Chapter) *models.RPGState {
	state := m.CalculateStateMatrix(outline, targetChapter)
	if state == nil {
		return nil
	}
	return state.RPG
}

// applyEvent applies a single event to the state matrix
func (m *StateMatrixManager) applyEvent(state *models.StateMatrix, event models.Event, chapterID string) {
	m.applyRPGEvent(state, event, chapterID)

	// Fields now tracked exclusively in RPGState:
	//   relationship → state.RPG.Relationships
	//   goal         → state.RPG.Characters[name].Goals
	//   item         → state.RPG.Resources (ownership) + state.RPG.Characters[name].Inventory
	//   storyline    → state.RPG.Storylines
	// applyRPGEvent() is always called first and handles all of the above.

	switch event.Type {
	case "premise":
		// Character premise/progression update
		if len(event.Characters) > 0 && event.Subject != "" {
			key := event.Characters[0] + "_" + event.Subject
			state.Premises[key] = event.Change
		}
	case "gate":
		// Gate/obstacle introduced, escalated, or overcome
		if event.Subject != "" {
			charName := ""
			if len(event.Characters) > 0 {
				charName = event.Characters[0]
			}

			gateState := &models.GateState{
				Name:       event.Subject,
				Status:     event.Change,
				Characters: charName,
				ChapterID:  chapterID,
				Details:    event.Details,
			}

			// If gate is overcome, remove it from active gates
			if event.Change == "overcome" {
				delete(state.Gates, event.Subject)
			} else {
				// Store or update the gate
				state.Gates[event.Subject] = gateState
			}
		}
	case "status":
		// Character physical/mental status change
		if len(event.Characters) > 0 && event.Subject != "" {
			charName := event.Characters[0]
			statusType := event.Subject

			// Smart status key generation:
			// If subject already contains charName, use subject directly
			// Otherwise, use charName_subject format
			var statusKey string
			if strings.Contains(statusType, charName) {
				statusKey = statusType
			} else {
				statusKey = charName + "_" + statusType
			}

			// If status is resolved/recovered, remove it
			if event.Change == "resolved" || event.Change == "recovered" || event.Change == "healed" {
				delete(state.Status, statusKey)
			} else {
				// Store or update status
				state.Status[statusKey] = &models.StatusState{
					Type:      statusType,
					State:     event.Change,
					ChapterID: chapterID,
					Details:   event.Details,
				}
			}
		}
	case "memory":
		// Character learns/acquires information
		if len(event.Characters) > 0 && event.Subject != "" {
			charName := event.Characters[0]
			memory := &models.MemoryState{
				Info:      event.Subject,
				Category:  event.Change,
				ChapterID: chapterID,
				Details:   event.Details,
			}
			state.Memories[charName] = append(state.Memories[charName], memory)
		}
	}
}

func newRPGState() *models.RPGState {
	return &models.RPGState{
		Characters:    make(map[string]*models.RPGCharacterState),
		Resources:     make(map[string]*models.RPGResourceState),
		Relationships: make(map[string]*models.RPGRelationState),
		Storylines:    make(map[string]*models.RPGQuestState),
		Systems:       make(map[string]*models.RPGSystemState),
		Flags:         make(map[string]string),
		Timeline:      make([]models.RPGTimelineEntry, 0),
		Deltas:        make([]models.RPGStateDelta, 0),
	}
}

func (m *StateMatrixManager) ensureRPGState(state *models.StateMatrix) *models.RPGState {
	if state.RPG == nil {
		state.RPG = newRPGState()
	}
	return state.RPG
}

func (m *StateMatrixManager) applyRPGEvent(state *models.StateMatrix, event models.Event, chapterID string) {
	rpgState := m.ensureRPGState(state)
	rpgState.CurrentChapter = chapterID

	actor := event.GetActor()
	action := event.GetAction()
	target := event.GetTarget()
	targetType := event.GetTargetType()
	result := firstNonEmpty(event.Result, event.Details, event.Change)
	if actor == "" && len(event.Characters) > 0 {
		actor = event.Characters[0]
	}

	if actor != "" {
		m.ensureRPGCharacter(state, actor).LastChangedAt = chapterID
	}
	if event.Context != "" {
		rpgState.CurrentLocation = event.Context
	}

	rpgState.Timeline = append(rpgState.Timeline, models.RPGTimelineEntry{
		ChapterID: chapterID,
		Actor:     actor,
		Action:    action,
		Target:    target,
		Result:    result,
	})

	switch {
	case targetType == "item" || event.Type == "item":
		m.applyRPGResourceEvent(state, chapterID, actor, action, target, result, event.Change)
	case targetType == "status" || event.Type == "status":
		m.applyRPGStatusEvent(state, chapterID, actor, target, result, event.Change)
	case targetType == "relationship" || event.Type == "relationship":
		m.applyRPGRelationshipEvent(state, chapterID, event, actor, target, result)
	case targetType == "knowledge" || action == models.ActionLearn || action == models.ActionDiscover || event.Type == "memory":
		m.applyRPGKnowledgeEvent(state, chapterID, actor, target, result, event.Change)
	case event.Type == "goal":
		m.applyRPGGoalEvent(state, chapterID, actor, target, result, event.Change)
	case event.Type == "storyline":
		m.applyRPGQuestEvent(state, chapterID, event)
	case event.Type == "premise":
		m.applyRPGSystemEvent(state, chapterID, event, actor)
	}

	switch action {
	case models.ActionMove, models.ActionEnter, models.ActionTeleport:
		if actor != "" && target != "" {
			char := m.ensureRPGCharacter(state, actor)
			from := char.Location
			char.Location = target
			char.LastChangedAt = chapterID
			rpgState.CurrentLocation = target
			m.addRPGDelta(state, chapterID, actor, "location", "location", from, target, 0, "", "", result)
		}
	case models.ActionDefeat:
		if target != "" {
			char := m.ensureRPGCharacter(state, target)
			from := boolState(char.Alive)
			char.Alive = false
			char.LastChangedAt = chapterID
			m.addRPGDelta(state, chapterID, target, "life", "alive", from, "dead", -1, "", "", result)
		}
	case models.ActionRecover:
		if actor != "" {
			m.applyRPGStatusEvent(state, chapterID, actor, "recovery", result, "recovered")
		}
	}
}

func (m *StateMatrixManager) applyRPGResourceEvent(state *models.StateMatrix, chapterID, actor, action, target, result, change string) {
	if target == "" {
		return
	}
	if actor != "" && target == actor {
		return
	}
	resource := m.ensureRPGResource(state, target)
	fromOwner := resource.Owner
	fromQty := resource.Quantity
	if resource.Quantity == 0 {
		resource.Quantity = 1
	}

	switch action {
	case models.ActionAcquire:
		resource.Owner = actor
		resource.Status = "owned"
	case models.ActionLose:
		resource.Owner = ""
		resource.Status = "lost"
	case models.ActionUse:
		resource.Status = "used"
		resource.Quantity--
	default:
		switch change {
		case "get", "acquired", "obtained":
			resource.Owner = actor
			resource.Status = "owned"
		case "lost", "used", "consumed":
			resource.Status = change
			if change == "lost" {
				resource.Owner = ""
			}
			if change == "used" || change == "consumed" {
				resource.Quantity--
			}
		}
	}
	if resource.Quantity < 0 {
		resource.Quantity = 0
	}
	resource.LastChangedAt = chapterID
	resource.Details = result

	if actor != "" {
		char := m.ensureRPGCharacter(state, actor)
		if char.Inventory == nil {
			char.Inventory = make(map[string]int)
		}
		switch resource.Status {
		case "owned":
			char.Inventory[target] = resource.Quantity
		case "lost", "used", "consumed":
			delete(char.Inventory, target)
		}
	}

	m.addRPGDelta(state, chapterID, target, "resource", "owner", fromOwner, resource.Owner, resource.Quantity-fromQty, "count", "", result)
}

func (m *StateMatrixManager) applyRPGStatusEvent(state *models.StateMatrix, chapterID, actor, target, result, change string) {
	if actor == "" {
		return
	}
	if target == "" {
		target = "status"
	}
	char := m.ensureRPGCharacter(state, actor)
	if char.Status == nil {
		char.Status = make(map[string]string)
	}
	from := char.Status[target]
	to := firstNonEmpty(change, result)
	if to == "" {
		to = "changed"
	}
	if to == "resolved" || to == "recovered" || to == "healed" {
		delete(char.Status, target)
	} else {
		char.Status[target] = to
	}
	char.LastChangedAt = chapterID

	kind := "status"
	if looksLikeCultivation(target) || looksLikeCultivation(to) {
		kind = "cultivation"
		char.Realm = to
		if level := extractArabicInt(to); level > 0 {
			char.Level = level
		}
	}
	m.addRPGDelta(state, chapterID, actor, kind, target, from, to, 0, "", "", result)
}

func (m *StateMatrixManager) applyRPGRelationshipEvent(state *models.StateMatrix, chapterID string, event models.Event, actor, target, result string) {
	rpgState := m.ensureRPGState(state)
	from := actor
	to := target
	if len(event.Characters) >= 2 {
		from = event.Characters[0]
		to = event.Characters[1]
	}
	if from == "" || to == "" {
		return
	}
	key := from + "_" + to
	old := ""
	if existing := rpgState.Relationships[key]; existing != nil {
		old = existing.Status
	}
	status := firstNonEmpty(event.Change, result, "changed")
	rpgState.Relationships[key] = &models.RPGRelationState{
		From:          from,
		To:            to,
		Status:        status,
		LastChangedAt: chapterID,
		Details:       result,
	}
	m.addRPGDelta(state, chapterID, key, "relationship", "status", old, status, 0, "", "", result)
}

func (m *StateMatrixManager) applyRPGKnowledgeEvent(state *models.StateMatrix, chapterID, actor, target, result, change string) {
	if actor == "" || target == "" {
		return
	}
	char := m.ensureRPGCharacter(state, actor)
	entry := target
	if result != "" {
		entry = target + ": " + result
	}
	if !containsString(char.Knowledge, entry) {
		char.Knowledge = append(char.Knowledge, entry)
	}
	char.LastChangedAt = chapterID
	m.addRPGDelta(state, chapterID, actor, "knowledge", firstNonEmpty(change, "known"), "", target, 1, "", "", result)
}

func (m *StateMatrixManager) applyRPGGoalEvent(state *models.StateMatrix, chapterID, actor, target, result, change string) {
	if actor == "" {
		return
	}
	char := m.ensureRPGCharacter(state, actor)
	goal := firstNonEmpty(result, change, target)
	if goal != "" && !containsString(char.Goals, goal) {
		char.Goals = append(char.Goals, goal)
	}
	char.LastChangedAt = chapterID
	m.addRPGDelta(state, chapterID, actor, "goal", "current", "", goal, 0, "", "", result)
}

func (m *StateMatrixManager) applyRPGQuestEvent(state *models.StateMatrix, chapterID string, event models.Event) {
	rpgState := m.ensureRPGState(state)
	if event.Subject == "" {
		return
	}
	quest := rpgState.Storylines[event.Subject]
	if quest == nil {
		quest = &models.RPGQuestState{ID: event.Subject, Name: event.Subject}
		// Load description from story setup if available
		if setup := m.loadSetup(); setup != nil {
			for _, sl := range setup.Storylines {
				if sl.Name == event.Subject {
					quest.Name = sl.Name
					quest.Description = sl.Description
					break
				}
			}
		}
		rpgState.Storylines[event.Subject] = quest
	}
	quest.Status = event.Change
	quest.Progress = event.Details
	quest.ProgressHistory = append(quest.ProgressHistory, models.StorylineProgress{
		ChapterID: chapterID,
		Status:    event.Change,
		Details:   event.Details,
	})
	m.addRPGDelta(state, chapterID, event.Subject, "storyline", "status", "", event.Change, 0, "", "", event.Details)
}

func (m *StateMatrixManager) applyRPGSystemEvent(state *models.StateMatrix, chapterID string, event models.Event, actor string) {
	rpgState := m.ensureRPGState(state)
	if event.Subject == "" {
		return
	}
	key := actor + "_" + event.Subject
	if actor == "" {
		key = event.Subject
	}
	system := rpgState.Systems[key]
	if system == nil {
		system = &models.RPGSystemState{ID: key, Name: event.Subject, Type: "premise", Values: make(map[string]string)}
		rpgState.Systems[key] = system
	}
	system.Status = event.Change
	system.Details = event.Details
	system.Values["owner"] = actor
	m.addRPGDelta(state, chapterID, key, "system", event.Subject, "", event.Change, 0, "", "", event.Details)

	if actor != "" && looksLikeCultivationStateChange(event.Subject+" "+event.Change+" "+event.Details) {
		char := m.ensureRPGCharacter(state, actor)
		from := char.Realm
		char.Realm = firstNonEmpty(event.Change, event.Details)
		if level := extractArabicInt(char.Realm); level > 0 {
			char.Level = level
		}
		char.LastChangedAt = chapterID
		m.addRPGDelta(state, chapterID, actor, "cultivation", "realm", from, char.Realm, 0, "", "", event.Details)
	}
}

func (m *StateMatrixManager) ensureRPGCharacter(state *models.StateMatrix, name string) *models.RPGCharacterState {
	rpgState := m.ensureRPGState(state)
	char := rpgState.Characters[name]
	if char == nil {
		char = &models.RPGCharacterState{
			ID:        name,
			Name:      name,
			Alive:     true,
			Status:    make(map[string]string),
			Inventory: make(map[string]int),
		}
		if modelChar := state.Characters[name]; modelChar != nil {
			char.Role = modelChar.RoleInStory
		}
		rpgState.Characters[name] = char
	}
	return char
}

func (m *StateMatrixManager) ensureRPGResource(state *models.StateMatrix, name string) *models.RPGResourceState {
	rpgState := m.ensureRPGState(state)
	resource := rpgState.Resources[name]
	if resource == nil {
		resource = &models.RPGResourceState{ID: name, Name: name}
		rpgState.Resources[name] = resource
	}
	return resource
}

func (m *StateMatrixManager) addRPGDelta(state *models.StateMatrix, chapterID, target, kind, field, from, to string, delta int, unit, cost, note string) {
	rpgState := m.ensureRPGState(state)
	rpgState.Deltas = append(rpgState.Deltas, models.RPGStateDelta{
		ChapterID: chapterID,
		Target:    target,
		Kind:      kind,
		Field:     field,
		From:      from,
		To:        to,
		Delta:     delta,
		Unit:      unit,
		Cost:      cost,
		Note:      note,
	})
}

func (m *StateMatrixManager) applyRPGDSLDeltasForChapter(state *models.StateMatrix, chapterID string) {
	if state == nil || !m.useRPGDSL {
		return
	}
	for _, delta := range m.loadRPGDSLDeltas()[chapterID] {
		m.applyRPGStateDelta(state, delta)
	}
}

func (m *StateMatrixManager) loadRPGDSLDeltas() map[string][]models.RPGStateDelta {
	if m.rpgDeltasLoaded {
		return m.rpgDeltasByChapter
	}
	m.rpgDeltasLoaded = true
	m.rpgDeltasByChapter = make(map[string][]models.RPGStateDelta)
	m.rpgTargetAliases = make(map[string]string)
	if strings.TrimSpace(m.projectRoot) == "" {
		return m.rpgDeltasByChapter
	}

	for _, path := range m.rpgDSLPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsed, err := rpgdsl.NewParser(string(data)).Parse()
		if err != nil || parsed == nil {
			continue
		}
		m.addRPGAliasesFromDSL(parsed)
		if parsed.Storyline == nil {
			continue
		}
		for _, chapter := range parsed.Storyline.Chapters {
			for _, objective := range chapter.Objectives {
				for _, step := range objective.Steps {
					for _, delta := range step.Event.StateDeltas {
						converted := convertRPGDSLStateDelta(chapter.ID, delta)
						m.rpgDeltasByChapter[chapter.ID] = append(m.rpgDeltasByChapter[chapter.ID], converted)
					}
				}
			}
		}
	}
	return m.rpgDeltasByChapter
}

func (m *StateMatrixManager) rpgDSLPaths() []string {
	rpgDir := filepath.Join(m.projectRoot, "story", "rpg")
	candidates := []string{
		filepath.Join(rpgDir, "01_outline.rpg"),
		filepath.Join(rpgDir, "02_craft.rpg"),
		filepath.Join(rpgDir, "03_systems.rpg"),
		filepath.Join(rpgDir, "04_chapters.rpg"),
	}
	paths := make([]string, 0, len(candidates))
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func (m *StateMatrixManager) addRPGAliasesFromDSL(parsed *rpgdsl.DSL) {
	if parsed == nil || parsed.Characters == nil {
		return
	}
	if parsed.Characters.Player != nil {
		m.rpgPlayerID = strings.TrimSpace(parsed.Characters.Player.ID)
		m.addRPGTargetAlias(parsed.Characters.Player.ID, parsed.Characters.Player.Name)
	}
	for _, npc := range parsed.Characters.NPCs {
		m.addRPGTargetAlias(npc.ID, npc.Name)
	}
	for _, enemy := range parsed.Characters.Enemies {
		m.addRPGTargetAlias(enemy.ID, enemy.Name)
	}
}

func (m *StateMatrixManager) addRPGTargetAlias(id, name string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return
	}
	if id == name {
		return
	}
	m.rpgTargetAliases[id] = name
}

func convertRPGDSLStateDelta(chapterID string, delta rpgdsl.StateDelta) models.RPGStateDelta {
	return models.RPGStateDelta{
		ChapterID: chapterID,
		Target:    delta.Target,
		Kind:      delta.Kind,
		Field:     delta.Field,
		From:      delta.From,
		To:        delta.To,
		Delta:     delta.Delta,
		Unit:      delta.Unit,
		Cost:      delta.Cost,
		Note:      delta.Note,
	}
}

func (m *StateMatrixManager) applyRPGStateDelta(state *models.StateMatrix, delta models.RPGStateDelta) {
	rpgState := m.ensureRPGState(state)
	rpgState.CurrentChapter = delta.ChapterID
	rpgState.Deltas = append(rpgState.Deltas, delta)

	kind := strings.ToLower(strings.TrimSpace(delta.Kind))
	field := firstNonEmpty(delta.Field, kind, "state")
	to := firstNonEmpty(delta.To, delta.Note)
	context := strings.Join([]string{delta.Target, delta.Field, delta.From, delta.To, delta.Note}, " ")

	switch kind {
	case "death":
		target := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		char := m.ensureRPGCharacter(state, target)
		char.Alive = false
		char.LastChangedAt = delta.ChapterID
		char.Status["life"] = "dead"
	case "revive":
		target := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		char := m.ensureRPGCharacter(state, target)
		char.Alive = true
		char.LastChangedAt = delta.ChapterID
		delete(char.Status, "life")
	case "cultivation":
		target := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		char := m.ensureRPGCharacter(state, target)
		from := char.Realm
		if to != "" {
			char.Realm = to
		}
		if level := extractArabicInt(firstNonEmpty(delta.To, delta.From, delta.Note)); level > 0 {
			char.Level = level
		}
		char.LastChangedAt = delta.ChapterID
		if char.Status == nil {
			char.Status = make(map[string]string)
		}
		char.Status[field] = firstNonEmpty(to, from)
	case "lifespan":
		target := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		char := m.ensureRPGCharacter(state, target)
		if char.Status == nil {
			char.Status = make(map[string]string)
		}
		value := to
		if value == "" && delta.Delta != 0 {
			value = strconv.Itoa(delta.Delta)
			if delta.Unit != "" {
				value += delta.Unit
			}
		}
		char.Status["lifespan"] = value
		char.LastChangedAt = delta.ChapterID
	case "injury", "status":
		target := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		char := m.ensureRPGCharacter(state, target)
		if to == "recovered" || to == "resolved" || to == "healed" {
			delete(char.Status, field)
		} else {
			char.Status[field] = to
		}
		char.LastChangedAt = delta.ChapterID
	case "resource":
		owner := m.resolveRPGDeltaCharacterTarget(state, delta.Target, context)
		resourceName := firstNonEmpty(delta.Field, delta.Target)
		if resourceName == "" {
			return
		}
		if owner != "" && owner == resourceName {
			return
		}
		if resourceName == delta.Target && owner != delta.Target && delta.Field == "" {
			resourceName = delta.Target
		}
		resource := m.ensureRPGResource(state, resourceName)
		resource.LastChangedAt = delta.ChapterID
		resource.Details = delta.Note
		if owner != "" && owner != "unknown" && owner != resourceName {
			resource.Owner = owner
		}
		if delta.Delta != 0 {
			resource.Quantity += delta.Delta
		}
		if resource.Quantity < 0 {
			resource.Quantity = 0
		}
		if to != "" {
			resource.Status = to
		}
		if resource.Status == "" && resource.Owner != "" {
			resource.Status = "owned"
		}
		if owner != "" && owner != "unknown" && owner != resourceName {
			char := m.ensureRPGCharacter(state, owner)
			if char.Inventory == nil {
				char.Inventory = make(map[string]int)
			}
			char.Inventory[resourceName] = resource.Quantity
			char.LastChangedAt = delta.ChapterID
		}
	case "relationship":
		target := firstNonEmpty(delta.Target, delta.Field)
		if target == "" {
			return
		}
		old := rpgState.Relationships[target]
		rel := &models.RPGRelationState{
			Status:        to,
			LastChangedAt: delta.ChapterID,
			Details:       delta.Note,
		}
		if old != nil {
			rel.From = old.From
			rel.To = old.To
		}
		rpgState.Relationships[target] = rel
	case "time", "transition":
		key := kind
		if delta.Field != "" {
			key += "." + delta.Field
		}
		rpgState.Flags[key] = firstNonEmpty(to, strconv.Itoa(delta.Delta), delta.Note)
	default:
		if delta.Target == "" {
			return
		}
		rpgState.Flags[delta.Target+"."+field] = to
	}
}

func (m *StateMatrixManager) resolveRPGDeltaCharacterTarget(state *models.StateMatrix, target, context string) string {
	if target != "" {
		if alias := m.rpgTargetAliases[target]; alias != "" {
			return alias
		}
		if target == m.rpgPlayerID {
			if protagonist := findProtagonistName(state); protagonist != "" {
				return protagonist
			}
		}
		if state.RPG != nil && state.RPG.Characters[target] != nil {
			return target
		}
	}
	for name := range state.Characters {
		if strings.Contains(context, name) {
			return name
		}
	}
	if target != "" {
		return target
	}
	return "unknown"
}

func findProtagonistName(state *models.StateMatrix) string {
	if state == nil {
		return ""
	}
	names := make([]string, 0, len(state.Characters))
	for name := range state.Characters {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		char := state.Characters[name]
		if char != nil && strings.Contains(strings.ToLower(char.RoleInStory), "protagonist") {
			return name
		}
	}
	for _, name := range names {
		char := state.Characters[name]
		if char != nil && strings.Contains(char.RoleInStory, "主角") && !strings.Contains(char.RoleInStory, "反派") {
			return name
		}
	}
	return ""
}

// applyStorylineEventWithDescription is deprecated — storyline state is now tracked
// via RPGQuestState in state.RPG.Storylines (populated by applyRPGQuestEvent).
// Kept as a no-op for compatibility; will be removed in a future cleanup.
func (m *StateMatrixManager) applyStorylineEventWithDescription(state *models.StateMatrix, event models.Event, chapterID string) {
}

// loadElementsIntoState loads generated elements into state matrix
func (m *StateMatrixManager) loadElementsIntoState(state *models.StateMatrix) {
	if m.projectRoot == "" {
		return
	}

	// Load characters
	charPath := filepath.Join(m.projectRoot, "story", "craft", "characters.json")
	if data, err := os.ReadFile(charPath); err == nil {
		var chars map[string]*models.Character
		if err := json.Unmarshal(data, &chars); err == nil {
			for name, char := range chars {
				state.Characters[name] = char
				rpgChar := m.ensureRPGCharacter(state, name)
				rpgChar.ID = name
				rpgChar.Name = char.Name
				rpgChar.Role = char.RoleInStory
				rpgChar.Alive = true
				// Also add aliases to the map for lookup
				for _, alias := range char.Aliases {
					if alias != "" && alias != name {
						state.Characters[alias] = char
					}
				}
			}
		}
	}

	// Load locations
	locPath := filepath.Join(m.projectRoot, "story", "craft", "locations.json")
	if data, err := os.ReadFile(locPath); err == nil {
		var locs map[string]*models.Location
		if err := json.Unmarshal(data, &locs); err == nil {
			for name, loc := range locs {
				state.Locations[name] = loc
			}
		}
	}

	// Load items
	// Note: Items from craft are loaded without owners. Ownership is tracked via Events.
	itemPath := filepath.Join(m.projectRoot, "story", "craft", "items.json")
	if data, err := os.ReadFile(itemPath); err == nil {
		var items map[string]*models.Item
		if err := json.Unmarshal(data, &items); err == nil {
			for name, item := range items {
				// Clear owner from craft definition - ownership is determined by Events
				item.Owner = ""
				state.Items[name] = item
				resource := m.ensureRPGResource(state, name)
				resource.Name = item.Name
				resource.Owner = ""
				resource.Status = "available"
				resource.Details = item.Description
			}
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func boolState(alive bool) string {
	if alive {
		return "alive"
	}
	return "dead"
}

func looksLikeCultivation(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "修为") ||
		strings.Contains(value, "境界") ||
		strings.Contains(value, "练气") ||
		strings.Contains(value, "炼气") ||
		strings.Contains(value, "筑基") ||
		strings.Contains(value, "金丹") ||
		strings.Contains(value, "level") ||
		strings.Contains(value, "realm") ||
		strings.Contains(value, "cultivation")
}

func looksLikeCultivationStateChange(value string) bool {
	if !looksLikeCultivation(value) {
		return false
	}
	for _, blocker := range []string{"规则", "代价", "折损", "寿命", "寿元", "复活能力", "附加规则"} {
		if strings.Contains(value, blocker) {
			return false
		}
	}
	return true
}

func extractArabicInt(value string) int {
	var b strings.Builder
	last := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			last, _ = strconv.Atoi(b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		last, _ = strconv.Atoi(b.String())
	}
	if last == 0 {
		return extractChineseLevel(value)
	}
	return last
}

func extractChineseLevel(value string) int {
	bestIndex := -1
	bestValue := 0
	for _, token := range []struct {
		Text  string
		Value int
	}{
		{"十", 10},
		{"九", 9},
		{"八", 8},
		{"七", 7},
		{"六", 6},
		{"五", 5},
		{"四", 4},
		{"三", 3},
		{"二", 2},
		{"两", 2},
		{"一", 1},
	} {
		if idx := strings.LastIndex(value, token.Text); idx > bestIndex {
			bestIndex = idx
			bestValue = token.Value
		}
	}
	return bestValue
}
