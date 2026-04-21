package dsl

import (
	"fmt"
	"strconv"
	"strings"
)

// EnhancedParser wraps the standard parser with better error reporting
type EnhancedParser struct {
	*Parser
	reporter *ErrorReporter
	errors   *ErrorCollection
}

// NewEnhancedParser creates a new enhanced parser
func NewEnhancedParser(content string) *EnhancedParser {
	return &EnhancedParser{
		Parser:   NewParser(content),
		reporter: NewErrorReporter(content),
		errors:   &ErrorCollection{},
	}
}

// ParseEnhanced parses with enhanced error reporting
func (ep *EnhancedParser) ParseEnhanced() (*DSL, error) {
	content := strings.TrimSpace(ep.content)

	// Try JSON first
	if strings.HasPrefix(content, "{") {
		return ep.parseJSONEnhanced()
	}

	// Parse DSL format with enhanced error handling
	return ep.parseDSLEnhanced()
}

// parseJSONEnhanced parses JSON with enhanced errors
func (ep *EnhancedParser) parseJSONEnhanced() (*DSL, error) {
	dsl, err := ep.parseJSON()
	if err != nil {
		// Wrap with position info if possible
		return nil, &DSLParseError{
			Pos:     Position{Line: 1, Column: 1},
			Message: fmt.Sprintf("JSON parsing failed: %v", err),
			Context: ep.getCurrentContext(),
		}
	}
	return dsl, nil
}

// parseDSLEnhanced parses DSL with enhanced error handling
func (ep *EnhancedParser) parseDSLEnhanced() (*DSL, error) {
	dsl := &DSL{}

	for !ep.eof() {
		ep.skipWhitespace()
		if ep.eof() {
			break
		}

		// Skip comments
		if ep.peek() == '#' {
			ep.skipLine()
			continue
		}

		// Remember position before parsing block
		blockStartPos := Position{Line: ep.line, Column: ep.col}

		// Parse block type
		blockType, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, blockStartPos, "parsing block type")
		}

		var parseErr error
		switch blockType {
		case "metadata":
			dsl.Metadata, parseErr = ep.parseMetadataEnhanced()
		case "world":
			dsl.World, parseErr = ep.parseWorldEnhanced()
		case "characters":
			dsl.Characters, parseErr = ep.parseCharactersEnhanced()
		case "storyline":
			dsl.Storyline, parseErr = ep.parseStorylineEnhanced()
		case "systems":
			dsl.Systems, parseErr = ep.parseSystemsEnhanced()
		default:
			parseErr = &DSLParseError{
				Pos:     blockStartPos,
				Message: fmt.Sprintf("unknown block type: '%s'", blockType),
				Context: ep.getCurrentContext(),
			}
		}

		if parseErr != nil {
			return nil, ep.enhanceError(parseErr, blockStartPos, fmt.Sprintf("parsing '%s' block", blockType))
		}
	}

	return dsl, nil
}

// parseIdentifierEnhanced parses an identifier with enhanced errors
func (ep *EnhancedParser) parseIdentifierEnhanced() (string, error) {
	ep.skipWhitespace()
	start := ep.pos
	startLine := ep.line
	startCol := ep.col

	for !ep.eof() {
		ch := ep.peek()
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			ep.advance()
		} else {
			break
		}
	}

	if start == ep.pos {
		return "", &DSLParseError{
			Pos:     Position{Line: startLine, Column: startCol},
			Message: "expected identifier",
			Context: ep.getCurrentContext(),
		}
	}

	return ep.content[start:ep.pos], nil
}

// parseMetadataEnhanced parses metadata with enhanced errors
func (ep *EnhancedParser) parseMetadataEnhanced() (*Metadata, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	meta := &Metadata{}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing metadata key")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after metadata key '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "title":
			meta.Title = ep.toString(value)
		case "subtitle":
			meta.Subtitle = ep.toString(value)
		case "power_system":
			meta.PowerSystem = ep.toString(value)
		case "tone":
			meta.Tone = ep.toString(value)
		case "dsl_version":
			meta.DSLVersion = ep.toString(value)
		case "genre":
			meta.Genre = ep.toStringSlice(value)
		default:
			// Unknown field - add warning but continue
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing metadata block")
	}

	return meta, nil
}

// parseWorldEnhanced parses world block with enhanced errors
func (ep *EnhancedParser) parseWorldEnhanced() (*World, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	world := &World{
		Locations: make([]Location, 0),
		Items:     make([]Item, 0),
		Rules:     make([]Rule, 0),
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		blockPos := Position{Line: ep.line, Column: ep.col}
		blockType, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, blockPos, "parsing world block type")
		}

		switch blockType {
		case "location":
			loc, err := ep.parseLocationEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing location")
			}
			world.Locations = append(world.Locations, *loc)
		case "item":
			item, err := ep.parseItemEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing item")
			}
			world.Items = append(world.Items, *item)
		case "rule":
			rule, err := ep.parseRuleEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing rule")
			}
			world.Rules = append(world.Rules, *rule)
		default:
			return nil, &DSLParseError{
				Pos:     blockPos,
				Message: fmt.Sprintf("unknown world block type: '%s'", blockType),
				Context: ep.getCurrentContext(),
			}
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing world block")
	}

	return world, nil
}

// parseLocationEnhanced parses location with enhanced errors
func (ep *EnhancedParser) parseLocationEnhanced() (*Location, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing location name")
	}

	loc := &Location{
		Name:        name,
		Connections: make([]Connection, 0),
		Properties:  make(map[string]interface{}),
	}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing location property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "id":
			loc.ID = ep.toString(value)
		case "type":
			loc.Type = ep.toString(value)
		case "description":
			loc.Description = ep.toString(value)
		default:
			loc.Properties[key] = value
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing location block")
	}

	return loc, nil
}

// parseItemEnhanced parses item with enhanced errors
func (ep *EnhancedParser) parseItemEnhanced() (*Item, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing item name")
	}

	item := &Item{Name: name}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing item property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "id":
			item.ID = ep.toString(value)
		case "type":
			item.Type = ep.toString(value)
		case "rarity":
			item.Rarity = ep.toString(value)
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing item block")
	}

	return item, nil
}

// parseRuleEnhanced parses rule with enhanced errors
func (ep *EnhancedParser) parseRuleEnhanced() (*Rule, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing rule name")
	}

	rule := &Rule{Name: name}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing rule property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "trigger":
			rule.Trigger = ep.toString(value)
		case "effect":
			rule.Effect = ep.toString(value)
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing rule block")
	}

	return rule, nil
}

// parseCharactersEnhanced parses characters with enhanced errors
func (ep *EnhancedParser) parseCharactersEnhanced() (*Characters, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	chars := &Characters{
		Enemies: make([]Enemy, 0),
		NPCs:    make([]NPC, 0),
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		blockPos := Position{Line: ep.line, Column: ep.col}
		blockType, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, blockPos, "parsing character block type")
		}

		switch blockType {
		case "player":
			player, err := ep.parsePlayerEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing player")
			}
			chars.Player = player
		case "enemy":
			enemy, err := ep.parseEnemyEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing enemy")
			}
			chars.Enemies = append(chars.Enemies, *enemy)
		case "npc":
			npc, err := ep.parseNPCEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing NPC")
			}
			chars.NPCs = append(chars.NPCs, *npc)
		default:
			return nil, &DSLParseError{
				Pos:     blockPos,
				Message: fmt.Sprintf("unknown character block type: '%s'", blockType),
				Context: ep.getCurrentContext(),
			}
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing characters block")
	}

	return chars, nil
}

// parsePlayerEnhanced parses player with enhanced errors
func (ep *EnhancedParser) parsePlayerEnhanced() (*Player, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing player name")
	}

	player := &Player{
		Name:      name,
		Stats:     Stats{},
		Skills:    make([]string, 0),
		Inventory: make(map[string]int),
		Traits:    make(map[string]Trait),
	}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing player property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "id":
			player.ID = ep.toString(value)
		case "class":
			player.Class = ep.toString(value)
		case "skills":
			player.Skills = ep.toStringSlice(value)
		default:
			if statVal, ok := ep.toInt(value); ok {
				switch key {
				case "str":
					player.Stats.STR = statVal
				case "agi":
					player.Stats.AGI = statVal
				case "int":
					player.Stats.INT = statVal
				case "vit":
					player.Stats.VIT = statVal
				case "hp":
					player.Stats.HP = statVal
				case "mp":
					player.Stats.MP = statVal
				}
			}
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing player block")
	}

	return player, nil
}

// parseEnemyEnhanced parses enemy with enhanced errors
func (ep *EnhancedParser) parseEnemyEnhanced() (*Enemy, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing enemy name")
	}

	enemy := &Enemy{
		Name:           name,
		Template:       EnemyTemplate{StatsPerLevel: make(map[string]int)},
		SpawnLocations: make([]string, 0),
	}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing enemy property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "id":
			enemy.ID = ep.toString(value)
		case "type":
			enemy.Type = ep.toString(value)
		default:
			if statVal, ok := ep.toInt(value); ok {
				switch key {
				case "str":
					enemy.Template.StatsPerLevel["str"] = statVal
				case "agi":
					enemy.Template.StatsPerLevel["agi"] = statVal
				case "hp":
					enemy.Template.HPFormula = fmt.Sprintf("%d", statVal)
				}
			}
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing enemy block")
	}

	return enemy, nil
}

// parseNPCEnhanced parses NPC with enhanced errors
func (ep *EnhancedParser) parseNPCEnhanced() (*NPC, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing NPC name")
	}

	npc := &NPC{Name: name}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing NPC property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "id":
			npc.ID = ep.toString(value)
		case "role":
			npc.Role = ep.toString(value)
		case "dialogue":
			// Parse dialogue as object or simple greeting
			if greeting, ok := value.(string); ok {
				npc.Dialogue.Greeting = greeting
			}
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing NPC block")
	}

	return npc, nil
}

// parseStorylineEnhanced parses storyline with enhanced errors
func (ep *EnhancedParser) parseStorylineEnhanced() (*Storyline, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	story := &Storyline{
		Arcs:     make([]Arc, 0),
		Chapters: make([]Chapter, 0),
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		blockPos := Position{Line: ep.line, Column: ep.col}
		blockType, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, blockPos, "parsing storyline block type")
		}

		switch blockType {
		case "chapter":
			chapter, err := ep.parseChapterEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, blockPos, "parsing chapter")
			}
			story.Chapters = append(story.Chapters, *chapter)
		case "arc":
			ep.skipBlock()
		default:
			ep.skipBlock()
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing storyline block")
	}

	return story, nil
}

// parseChapterEnhanced parses chapter with enhanced errors
func (ep *EnhancedParser) parseChapterEnhanced() (*Chapter, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	title, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing chapter title")
	}

	chapter := &Chapter{
		Title:      title,
		Objectives: make([]Objective, 0),
	}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing chapter property")
		}

		switch key {
		case "id":
			if err := ep.expectCharEnhanced('='); err != nil {
				return nil, ep.enhanceError(err, keyPos, "after 'id'")
			}
			val, err := ep.parseValueEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, keyPos, "parsing chapter id")
			}
			chapter.ID = ep.toString(val)
		case "objective":
			obj, err := ep.parseObjectiveEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, keyPos, "parsing objective")
			}
			chapter.Objectives = append(chapter.Objectives, *obj)
		default:
			if err := ep.expectCharEnhanced('='); err != nil {
				return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
			}
			ep.parseValueEnhanced()
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing chapter block")
	}

	return chapter, nil
}

// parseObjectiveEnhanced parses objective with enhanced errors
func (ep *EnhancedParser) parseObjectiveEnhanced() (*Objective, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	name, err := ep.parseStringEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing objective name")
	}

	obj := &Objective{
		Name:  name,
		Steps: make([]Step, 0),
	}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing objective property")
		}

		switch key {
		case "step":
			step, err := ep.parseStepEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, keyPos, "parsing step")
			}
			obj.Steps = append(obj.Steps, *step)
		default:
			if err := ep.expectCharEnhanced('='); err != nil {
				return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
			}
			ep.parseValueEnhanced()
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing objective block")
	}

	return obj, nil
}

// parseStepEnhanced parses step with enhanced errors
func (ep *EnhancedParser) parseStepEnhanced() (*Step, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	numStr, err := ep.parseNumberEnhanced()
	if err != nil {
		return nil, ep.enhanceError(err, startPos, "parsing step number")
	}

	order, _ := strconv.Atoi(numStr)
	step := &Step{Order: order}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing step property")
		}

		switch key {
		case "description":
			if err := ep.expectCharEnhanced('='); err != nil {
				return nil, ep.enhanceError(err, keyPos, "after 'description'")
			}
			val, err := ep.parseValueEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, keyPos, "parsing description")
			}
			step.Description = ep.toString(val)
		case "event":
			event, err := ep.parseEventEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, keyPos, "parsing event")
			}
			step.Event = *event
		default:
			if err := ep.expectCharEnhanced('='); err != nil {
				return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
			}
			ep.parseValueEnhanced()
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing step block")
	}

	return step, nil
}

// parseEventEnhanced parses event with enhanced errors
func (ep *EnhancedParser) parseEventEnhanced() (*Event, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	event := &Event{}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing event property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "type":
			event.Type = ep.toString(value)
		case "actor":
			if event.Spawn == nil {
				event.Spawn = &SpawnEvent{}
			}
			event.Spawn.Actor = ep.toString(value)
		case "location":
			if event.Spawn == nil {
				event.Spawn = &SpawnEvent{}
			}
			event.Spawn.Location = ep.toString(value)
		case "to":
			if event.Move == nil {
				event.Move = &MoveEvent{}
			}
			event.Move.To = ep.toString(value)
		case "enemies":
			if event.Combat == nil {
				event.Combat = &CombatEvent{}
			}
			event.Combat.Setup.Enemies = ep.parseEnemySpawnList(value)
		case "on_complete":
			result, err := ep.parseEventResultEnhanced()
			if err != nil {
				return nil, ep.enhanceError(err, valuePos, "parsing on_complete")
			}
			event.OnComplete = result
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing event block")
	}

	return event, nil
}

// parseEventResultEnhanced parses event result with enhanced errors
func (ep *EnhancedParser) parseEventResultEnhanced() (*EventResult, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	result := &EventResult{}

	for !ep.peekChar('}') {
		ep.skipWhitespace()
		if ep.peekChar('}') {
			break
		}

		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing event result property")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		value, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		switch key {
		case "narration":
			result.Narration = ep.toString(value)
		case "exp":
			if exp, ok := ep.toInt(value); ok {
				result.Exp = exp
			}
		case "result":
			result.Result = ep.toString(value)
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing event result block")
	}

	return result, nil
}

// parseSystemsEnhanced parses systems with enhanced errors
func (ep *EnhancedParser) parseSystemsEnhanced() (*Systems, error) {
	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	systems := &Systems{}
	ep.skipBlock()

	return systems, nil
}

// Helper methods for enhanced parsing

func (ep *EnhancedParser) expectCharEnhanced(c byte) error {
	ep.skipWhitespace()
	if ep.peek() != c {
		return &DSLParseError{
			Pos:     Position{Line: ep.line, Column: ep.col},
			Message: fmt.Sprintf("expected '%c', found '%c'", c, ep.peek()),
			Context: ep.getCurrentContext(),
		}
	}
	ep.advance()
	return nil
}

func (ep *EnhancedParser) parseStringEnhanced() (string, error) {
	ep.skipWhitespace()
	startLine := ep.line
	startCol := ep.col

	if ep.peek() != '"' {
		return "", &DSLParseError{
			Pos:     Position{Line: startLine, Column: startCol},
			Message: "expected string (starting with \")",
			Context: ep.getCurrentContext(),
		}
	}

	ep.advance() // skip opening quote
	start := ep.pos

	for !ep.eof() && ep.peek() != '"' {
		if ep.peek() == '\n' {
			return "", &DSLParseError{
				Pos:     Position{Line: ep.line, Column: ep.col},
				Message: "unterminated string (newline in string)",
				Context: ep.getCurrentContext(),
			}
		}
		ep.advance()
	}

	if ep.eof() {
		return "", &DSLParseError{
			Pos:     Position{Line: startLine, Column: startCol},
			Message: "unterminated string (reached end of file)",
			Context: ep.getCurrentContext(),
		}
	}

	str := ep.content[start:ep.pos]
	ep.advance() // skip closing quote
	return str, nil
}

func (ep *EnhancedParser) parseNumberEnhanced() (string, error) {
	ep.skipWhitespace()
	startLine := ep.line
	startCol := ep.col
	start := ep.pos

	for !ep.eof() {
		ch := ep.peek()
		if (ch >= '0' && ch <= '9') || ch == '.' {
			ep.advance()
		} else {
			break
		}
	}

	if start == ep.pos {
		return "", &DSLParseError{
			Pos:     Position{Line: startLine, Column: startCol},
			Message: "expected number",
			Context: ep.getCurrentContext(),
		}
	}

	return ep.content[start:ep.pos], nil
}

func (ep *EnhancedParser) parseValueEnhanced() (interface{}, error) {
	ep.skipWhitespace()
	ch := ep.peek()

	switch ch {
	case '"':
		return ep.parseStringEnhanced()
	case '[':
		return ep.parseArrayEnhanced()
	case '{':
		return ep.parseObjectEnhanced()
	default:
		if (ch >= '0' && ch <= '9') || ch == '-' {
			return ep.parseNumberEnhanced()
		}
		ident, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, err
		}
		if ident == "true" {
			return true, nil
		}
		if ident == "false" {
			return false, nil
		}
		return ident, nil
	}
}

func (ep *EnhancedParser) parseArrayEnhanced() ([]interface{}, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('['); err != nil {
		return nil, err
	}

	arr := make([]interface{}, 0)
	for !ep.peekChar(']') {
		valPos := Position{Line: ep.line, Column: ep.col}
		val, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valPos, "parsing array element")
		}
		arr = append(arr, val)

		ep.skipWhitespace()
		if ep.peek() == ',' {
			ep.advance()
		}
	}

	if err := ep.expectCharEnhanced(']'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing array")
	}

	return arr, nil
}

func (ep *EnhancedParser) parseObjectEnhanced() (map[string]interface{}, error) {
	startPos := Position{Line: ep.line, Column: ep.col}

	if err := ep.expectCharEnhanced('{'); err != nil {
		return nil, err
	}

	obj := make(map[string]interface{})
	for !ep.peekChar('}') {
		keyPos := Position{Line: ep.line, Column: ep.col}
		key, err := ep.parseIdentifierEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, keyPos, "parsing object key")
		}

		if err := ep.expectCharEnhanced('='); err != nil {
			return nil, ep.enhanceError(err, keyPos, fmt.Sprintf("after key '%s'", key))
		}

		valuePos := Position{Line: ep.line, Column: ep.col}
		val, err := ep.parseValueEnhanced()
		if err != nil {
			return nil, ep.enhanceError(err, valuePos, fmt.Sprintf("parsing value for '%s'", key))
		}

		obj[key] = val

		ep.skipWhitespace()
		if ep.peek() == ',' {
			ep.advance()
		}
	}

	if err := ep.expectCharEnhanced('}'); err != nil {
		return nil, ep.enhanceError(err, startPos, "closing object")
	}

	return obj, nil
}

// enhanceError wraps an error with additional context
func (ep *EnhancedParser) enhanceError(err error, pos Position, context string) error {
	if err == nil {
		return nil
	}

	// If it's already a DSLParseError, just add context
	if parseErr, ok := err.(*DSLParseError); ok {
		if parseErr.Context == "" {
			parseErr.Context = context
		}
		return parseErr
	}

	// Wrap as DSLParseError
	return &DSLParseError{
		Pos:     pos,
		Message: err.Error(),
		Context: context,
	}
}

// getCurrentContext returns the current parsing context
func (ep *EnhancedParser) getCurrentContext() string {
	start := ep.pos - 30
	if start < 0 {
		start = 0
	}
	end := ep.pos + 30
	if end > len(ep.content) {
		end = len(ep.content)
	}

	context := ep.content[start:end]
	context = strings.ReplaceAll(context, "\n", "\\n")
	return fmt.Sprintf("...%s...", context)
}

// FormatError formats an error with source context
func (ep *EnhancedParser) FormatError(err error) string {
	return ep.reporter.Report(err)
}
