package dsl

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Parser parses DSL content into AST
// For MVP, we support both DSL format and JSON format
type Parser struct {
	content string
	pos     int
	line    int
	col     int
}

// NewParser creates a new parser
func NewParser(content string) *Parser {
	return &Parser{
		content: content,
		pos:     0,
		line:    1,
		col:     1,
	}
}

// Parse parses the content into a DSL AST
func (p *Parser) Parse() (*DSL, error) {
	content := strings.TrimSpace(p.content)

	// Try JSON first
	if strings.HasPrefix(content, "{") {
		return p.parseJSON()
	}

	// Parse DSL format
	return p.parseDSL()
}

// parseJSON parses JSON format
func (p *Parser) parseJSON() (*DSL, error) {
	var dsl DSL
	if err := json.Unmarshal([]byte(p.content), &dsl); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &dsl, nil
}

// parseDSL parses DSL format (simplified HCL-like)
func (p *Parser) parseDSL() (*DSL, error) {
	dsl := &DSL{}

	for !p.eof() {
		p.skipWhitespace()
		if p.eof() {
			break
		}

		// Skip comments
		if p.peek() == '#' {
			p.skipLine()
			continue
		}

		// Parse block
		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, fmt.Errorf("line %d: expected block type: %w", p.line, err)
		}

		switch blockType {
		case "metadata":
			dsl.Metadata, err = p.parseMetadata()
		case "world":
			dsl.World, err = p.parseWorld()
		case "characters":
			dsl.Characters, err = p.parseCharacters()
		case "storyline":
			dsl.Storyline, err = p.parseStoryline()
		case "systems":
			dsl.Systems, err = p.parseSystems()
		default:
			return nil, fmt.Errorf("line %d: unknown block type: %s", p.line, blockType)
		}

		if err != nil {
			return nil, fmt.Errorf("line %d: error parsing %s: %w", p.line, blockType, err)
		}
	}

	return dsl, nil
}

// parseMetadata parses metadata block
func (p *Parser) parseMetadata() (*Metadata, error) {
	meta := &Metadata{}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "title":
			meta.Title = p.toString(value)
		case "subtitle":
			meta.Subtitle = p.toString(value)
		case "power_system":
			meta.PowerSystem = p.toString(value)
		case "tone":
			meta.Tone = p.toString(value)
		case "dsl_version":
			meta.DSLVersion = p.toString(value)
		case "genre":
			meta.Genre = p.toStringSlice(value)
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return meta, nil
}

// parseWorld parses world block
func (p *Parser) parseWorld() (*World, error) {
	world := &World{
		Locations: make([]Location, 0),
		Items:     make([]Item, 0),
		Rules:     make([]Rule, 0),
	}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch blockType {
		case "location":
			loc, err := p.parseLocation()
			if err != nil {
				return nil, err
			}
			world.Locations = append(world.Locations, *loc)
		case "item":
			item, err := p.parseItem()
			if err != nil {
				return nil, err
			}
			world.Items = append(world.Items, *item)
		case "rule":
			rule, err := p.parseRule()
			if err != nil {
				return nil, err
			}
			world.Rules = append(world.Rules, *rule)
		default:
			// Skip unknown block
			p.skipBlock()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return world, nil
}

// parseLocation parses a location block
func (p *Parser) parseLocation() (*Location, error) {
	loc := &Location{
		Connections: make([]Connection, 0),
		Properties:  make(map[string]interface{}),
	}

	// Parse location name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	loc.Name = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			loc.ID = p.toString(value)
		case "type":
			loc.Type = p.toString(value)
		case "description":
			loc.Description = p.toString(value)
		default:
			// Store other properties
			loc.Properties[key] = value
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return loc, nil
}

// parseCharacters parses characters block
func (p *Parser) parseCharacters() (*Characters, error) {
	chars := &Characters{
		Enemies: make([]Enemy, 0),
		NPCs:    make([]NPC, 0),
	}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch blockType {
		case "player":
			player, err := p.parsePlayer()
			if err != nil {
				return nil, err
			}
			chars.Player = player
		case "enemy":
			enemy, err := p.parseEnemy()
			if err != nil {
				return nil, err
			}
			chars.Enemies = append(chars.Enemies, *enemy)
		case "npc":
			npc, err := p.parseNPC()
			if err != nil {
				return nil, err
			}
			chars.NPCs = append(chars.NPCs, *npc)
		default:
			p.skipBlock()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return chars, nil
}

// parsePlayer parses a player block
func (p *Parser) parsePlayer() (*Player, error) {
	player := &Player{
		Stats:     Stats{},
		Skills:    make([]string, 0),
		Inventory: make(map[string]int),
		Traits:    make(map[string]Trait),
	}

	// Parse player name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	player.Name = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			player.ID = p.toString(value)
		case "class":
			player.Class = p.toString(value)
		case "skills":
			player.Skills = p.toStringSlice(value)
		default:
			// Try to parse as stats
			if statVal, ok := p.toInt(value); ok {
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

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return player, nil
}

// parseEnemy parses an enemy block
func (p *Parser) parseEnemy() (*Enemy, error) {
	enemy := &Enemy{
		Template:       EnemyTemplate{StatsPerLevel: make(map[string]int)},
		SpawnLocations: make([]string, 0),
	}

	// Parse enemy name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	enemy.Name = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			enemy.ID = p.toString(value)
		case "type":
			enemy.Type = p.toString(value)
		default:
			// Parse stats
			if statVal, ok := p.toInt(value); ok {
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

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return enemy, nil
}

// parseStoryline parses storyline block
func (p *Parser) parseStoryline() (*Storyline, error) {
	story := &Storyline{
		Arcs:     make([]Arc, 0),
		Chapters: make([]Chapter, 0),
	}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch blockType {
		case "chapter":
			chapter, err := p.parseChapter()
			if err != nil {
				return nil, err
			}
			story.Chapters = append(story.Chapters, *chapter)
		case "arc":
			// Skip arcs for MVP
			p.skipBlock()
		default:
			p.skipBlock()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return story, nil
}

// parseChapter parses a chapter block
func (p *Parser) parseChapter() (*Chapter, error) {
	chapter := &Chapter{
		Objectives: make([]Objective, 0),
	}

	// Parse chapter title
	title, err := p.parseString()
	if err != nil {
		return nil, err
	}
	chapter.Title = title

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			chapter.ID = p.toString(val)
		case "objective":
			obj, err := p.parseObjective()
			if err != nil {
				return nil, err
			}
			chapter.Objectives = append(chapter.Objectives, *obj)
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue() // Skip unknown values
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return chapter, nil
}

// parseObjective parses an objective block
func (p *Parser) parseObjective() (*Objective, error) {
	obj := &Objective{
		Steps: make([]Step, 0),
	}

	// Parse objective name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	obj.Name = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "step":
			step, err := p.parseStep()
			if err != nil {
				return nil, err
			}
			obj.Steps = append(obj.Steps, *step)
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return obj, nil
}

// parseStep parses a step block
func (p *Parser) parseStep() (*Step, error) {
	step := &Step{}

	// Parse step number
	numStr, err := p.parseNumber()
	if err != nil {
		return nil, err
	}
	step.Order, _ = strconv.Atoi(numStr)

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "description":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			step.Description = p.toString(val)
		case "event":
			event, err := p.parseEvent()
			if err != nil {
				return nil, err
			}
			step.Event = *event
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return step, nil
}

// parseEvent parses an event block
func (p *Parser) parseEvent() (*Event, error) {
	event := &Event{}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "type":
			event.Type = p.toString(value)
		case "actor":
			if event.Spawn == nil {
				event.Spawn = &SpawnEvent{}
			}
			event.Spawn.Actor = p.toString(value)
		case "location":
			if event.Spawn == nil {
				event.Spawn = &SpawnEvent{}
			}
			event.Spawn.Location = p.toString(value)
		case "to":
			if event.Move == nil {
				event.Move = &MoveEvent{}
			}
			event.Move.To = p.toString(value)
		case "enemies":
			if event.Combat == nil {
				event.Combat = &CombatEvent{}
			}
			event.Combat.Setup.Enemies = p.parseEnemySpawnList(value)
		default:
			// Handle nested blocks like on_complete
			if key == "on_complete" {
				result, err := p.parseEventResult()
				if err != nil {
					return nil, err
				}
				event.OnComplete = result
			}
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return event, nil
}

// parseEventResult parses an event result block
func (p *Parser) parseEventResult() (*EventResult, error) {
	result := &EventResult{}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		switch key {
		case "narration":
			result.Narration = p.toString(value)
		case "exp":
			if exp, ok := p.toInt(value); ok {
				result.Exp = exp
			}
		case "result":
			result.Result = p.toString(value)
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return result, nil
}

// parseSystems parses systems block (simplified for MVP)
func (p *Parser) parseSystems() (*Systems, error) {
	systems := &Systems{}

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespaceAndComments()
		if p.peekChar('}') {
			break
		}

		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch blockType {
		case "attribute_system":
			systems.AttributeSystem, err = p.parseAttributeSystem()
		case "power_formula":
			systems.PowerFormula, err = p.parsePowerFormula()
		case "progression":
			// Skip for now
			p.skipBlock()
		case "counter":
			// Skip for now
			p.skipBlock()
		default:
			// Skip unknown blocks
			p.skipBlock()
		}

		if err != nil {
			return nil, fmt.Errorf("line %d: error parsing %s: %w", p.line, blockType, err)
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return systems, nil
}

// Helper methods

func (p *Parser) eof() bool {
	return p.pos >= len(p.content)
}

func (p *Parser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.content[p.pos]
}

func (p *Parser) peekChar(c byte) bool {
	p.skipWhitespace()
	return p.peek() == c
}

func (p *Parser) advance() byte {
	if p.eof() {
		return 0
	}
	ch := p.content[p.pos]
	p.pos++
	if ch == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return ch
}

func (p *Parser) skipWhitespace() {
	for !p.eof() {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.advance()
		} else {
			break
		}
	}
}

func (p *Parser) skipWhitespaceAndComments() {
	for !p.eof() {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.advance()
		} else if ch == '#' {
			// Skip comment line
			p.skipLine()
		} else {
			break
		}
	}
}

func (p *Parser) skipLine() {
	for !p.eof() && p.peek() != '\n' {
		p.advance()
	}
	if !p.eof() {
		p.advance()
	}
}

func (p *Parser) skipBlock() {
	depth := 1
	for !p.eof() && depth > 0 {
		ch := p.advance()
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
		}
	}
}

func (p *Parser) expectChar(c byte) error {
	p.skipWhitespace()
	if p.peek() != c {
		return fmt.Errorf("expected '%c', got '%c'", c, p.peek())
	}
	p.advance()
	return nil
}

func (p *Parser) parseIdentifier() (string, error) {
	p.skipWhitespace()
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			p.advance()
		} else {
			break
		}
	}
	if start == p.pos {
		return "", fmt.Errorf("expected identifier")
	}
	return p.content[start:p.pos], nil
}

func (p *Parser) parseString() (string, error) {
	p.skipWhitespace()
	if p.peek() != '"' {
		return "", fmt.Errorf("expected string")
	}
	p.advance() // skip opening quote
	start := p.pos
	for !p.eof() && p.peek() != '"' {
		p.advance()
	}
	if p.eof() {
		return "", fmt.Errorf("unterminated string")
	}
	str := p.content[start:p.pos]
	p.advance() // skip closing quote
	return str, nil
}

func (p *Parser) parseNumber() (string, error) {
	p.skipWhitespace()
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if (ch >= '0' && ch <= '9') || ch == '.' {
			p.advance()
		} else {
			break
		}
	}
	if start == p.pos {
		return "", fmt.Errorf("expected number")
	}
	return p.content[start:p.pos], nil
}

func (p *Parser) parseValue() (interface{}, error) {
	p.skipWhitespace()
	ch := p.peek()

	switch ch {
	case '"':
		return p.parseString()
	case '[':
		return p.parseArray()
	case '{':
		return p.parseObject()
	default:
		// Try number
		if (ch >= '0' && ch <= '9') || ch == '-' {
			return p.parseNumber()
		}
		// Try boolean
		ident, _ := p.parseIdentifier()
		if ident == "true" {
			return true, nil
		}
		if ident == "false" {
			return false, nil
		}
		return ident, nil
	}
}

func (p *Parser) parseArray() ([]interface{}, error) {
	if err := p.expectChar('['); err != nil {
		return nil, err
	}

	arr := make([]interface{}, 0)
	for !p.peekChar(']') {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance()
		}
	}

	if err := p.expectChar(']'); err != nil {
		return nil, err
	}

	return arr, nil
}

func (p *Parser) parseObject() (map[string]interface{}, error) {
	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	obj := make(map[string]interface{})
	for !p.peekChar('}') {
		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expectChar('='); err != nil {
			return nil, err
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}

		obj[key] = val

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return obj, nil
}

// Type conversion helpers

func (p *Parser) toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (p *Parser) toStringSlice(v interface{}) []string {
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, len(arr))
		for i, item := range arr {
			result[i] = p.toString(item)
		}
		return result
	}
	return nil
}

func (p *Parser) toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i, true
		}
	}
	return 0, false
}

func (p *Parser) parseEnemySpawnList(v interface{}) []EnemySpawn {
	// Simplified for MVP
	return make([]EnemySpawn, 0)
}

// parseNPC parses an NPC block
func (p *Parser) parseNPC() (*NPC, error) {
	npc := &NPC{
		Personality:  make([]string, 0),
		Affiliations: make([]string, 0),
	}

	// Parse NPC name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	npc.Name = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.ID = p.toString(val)
		case "role":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Role = p.toString(val)
		case "description":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Description = p.toString(val)
		case "age":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if age, ok := p.toInt(val); ok {
				npc.Age = age
			}
		case "gender":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Gender = p.toString(val)
		case "appearance":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Appearance = p.toString(val)
		case "background":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Background = p.toString(val)
		case "personality":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Personality = p.toStringSlice(val)
		case "default_location":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.DefaultLocation = p.toString(val)
		case "affiliations":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			npc.Affiliations = p.toStringSlice(val)
		default:
			// Skip unknown fields
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return npc, nil
}

// Placeholder methods for unimplemented parsers
func (p *Parser) parseItem() (*Item, error) { return &Item{}, nil }
func (p *Parser) parseRule() (*Rule, error) { return &Rule{}, nil }

// parseAttributeSystem parses the attribute_system block
func (p *Parser) parseAttributeSystem() (*AttributeSystem, error) {
	sys := &AttributeSystem{
		Attributes: make([]AttributeDef, 0),
	}

	// Parse system name/ID
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	sys.Name = name
	sys.ID = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		blockType, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch blockType {
		case "id":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			sys.ID = p.toString(val)
		case "description":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			sys.Description = p.toString(val)
		case "attribute":
			attr, err := p.parseAttributeDef()
			if err != nil {
				return nil, err
			}
			sys.Attributes = append(sys.Attributes, *attr)
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return sys, nil
}

// parseAttributeDef parses a single attribute definition
func (p *Parser) parseAttributeDef() (*AttributeDef, error) {
	attr := &AttributeDef{}

	// Parse attribute name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	attr.Name = name
	attr.ID = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			attr.ID = p.toString(val)
		case "description":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			attr.Description = p.toString(val)
		case "type":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			attr.Type = p.toString(val)
		case "base_value":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if v, ok := p.toInt(val); ok {
				attr.BaseValue = v
			}
		case "min_value":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if v, ok := p.toInt(val); ok {
				attr.MinValue = v
			}
		case "max_value":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if v, ok := p.toInt(val); ok {
				attr.MaxValue = v
			}
		case "is_resource":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if v, ok := val.(bool); ok {
				attr.IsResource = v
			}
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return attr, nil
}

// parsePowerFormula parses the power_formula block
func (p *Parser) parsePowerFormula() (*PowerFormula, error) {
	formula := &PowerFormula{
		Factors: make([]Factor, 0),
	}

	// Parse formula name/ID
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	formula.Name = name
	formula.ID = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "id":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			formula.ID = p.toString(val)
		case "description":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			formula.Description = p.toString(val)
		case "base_power":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if v, ok := p.toInt(val); ok {
				formula.BasePower = v
			}
		case "factor":
			factor, err := p.parseFactor()
			if err != nil {
				return nil, err
			}
			formula.Factors = append(formula.Factors, *factor)
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return formula, nil
}

// parseFactor parses a single factor in power formula
func (p *Parser) parseFactor() (*Factor, error) {
	factor := &Factor{}

	// Parse factor name
	name, err := p.parseString()
	if err != nil {
		return nil, err
	}
	factor.Name = name
	factor.Attribute = name

	if err := p.expectChar('{'); err != nil {
		return nil, err
	}

	for !p.peekChar('}') {
		p.skipWhitespace()
		if p.peekChar('}') {
			break
		}

		key, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		switch key {
		case "attribute":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			factor.Attribute = p.toString(val)
		case "weight":
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			val, _ := p.parseValue()
			if numStr, ok := val.(string); ok {
				if w, err := strconv.ParseFloat(numStr, 64); err == nil {
					factor.Weight = w
				}
			}
		default:
			if err := p.expectChar('='); err != nil {
				return nil, err
			}
			p.parseValue()
		}
	}

	if err := p.expectChar('}'); err != nil {
		return nil, err
	}

	return factor, nil
}
