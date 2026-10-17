// Package dsl provides RPG-DSL parsing and conversion
package dsl

import "fmt"

// DSL represents the root of the DSL AST
type DSL struct {
	Metadata   *Metadata
	World      *World
	Characters *Characters
	Storyline  *Storyline
	Systems    *Systems
}

// Metadata represents story metadata
type Metadata struct {
	Title       string
	Subtitle    string
	Genre       []string
	PowerSystem string
	Tone        string
	DSLVersion  string
	Phase       string `json:"phase,omitempty"` // For merge tracking
}

// World represents the game world
type World struct {
	Locations []Location
	Items     []Item
	Rules     []Rule
}

// Location represents a game location
type Location struct {
	ID                string
	Name              string
	Type              string // indoor, outdoor, dungeon, city
	Description       string
	Appearance        string
	Atmosphere        string
	History           string
	SensoryDetails    map[string][]string // visual, audio, smell
	Inhabitants       []string
	Events            []string
	Secrets           string
	Connections       []Connection
	Properties        map[string]interface{}
	IsPlaceholder     bool   `json:"__placeholder__,omitempty"`
	PlaceholderSource string `json:"__source_phase__,omitempty"`
}

// Connection represents a connection between locations
type Connection struct {
	To        string
	Direction string
	Condition string
}

// Item represents a game item
type Item struct {
	ID          string
	Name        string
	Type        string // material, equipment, consumable, quest
	Rarity      string // common, uncommon, rare, epic, legendary
	Description string
	Effects     map[string]interface{}
	Sources     map[string]interface{}
}

// Rule represents an environmental rule
type Rule struct {
	Name    string
	Trigger string
	Effect  string
}

// Characters represents all characters in the game
type Characters struct {
	Player  *Player
	Enemies []Enemy
	NPCs    []NPC
}

// Player represents the player character
type Player struct {
	ID                string
	Name              string
	Class             string
	Stats             Stats
	Equipment         Equipment
	Skills            []string
	Inventory         map[string]int
	Traits            map[string]Trait
	Description       string
	Age               int
	Gender            string
	Race              string
	Background        string
	Personality       []string
	Motivation        string
	Abilities         []string
	Affiliations      []string
	RoleInStory       string
	Voice             string
	IsPlaceholder     bool   `json:"__placeholder__,omitempty"`
	PlaceholderSource string `json:"__source_phase__,omitempty"`
}

// Stats represents character statistics
type Stats struct {
	STR int `json:"str"`
	AGI int `json:"agi"`
	INT int `json:"int"`
	VIT int `json:"vit"`
	HP  int `json:"hp"`
	MP  int `json:"mp"`
}

// Equipment represents character equipment
type Equipment struct {
	Weapon string
	Armor  string
}

// Trait represents a character trait/ability
type Trait struct {
	Unlocked bool
	Trigger  string
}

// Enemy represents an enemy character
type Enemy struct {
	ID                string
	Name              string
	Description       string
	Type              string
	Template          EnemyTemplate
	Behavior          EnemyBehavior
	Drops             Drops
	SpawnLocations    []string
	SpawnWeight       int
	Appearance        string
	Abilities         []string
	Level             int
	IsPlaceholder     bool   `json:"__placeholder__,omitempty"`
	PlaceholderSource string `json:"__source_phase__,omitempty"`
}

// EnemyTemplate defines enemy stat templates
type EnemyTemplate struct {
	BaseLevel     int
	HPFormula     string
	StatsPerLevel map[string]int
}

// EnemyBehavior defines enemy AI behavior
type EnemyBehavior struct {
	AIType          string
	PreferredTarget string
	Skills          []string
}

// Drops represents enemy drop tables
type Drops struct {
	Fixed  []FixedDrop
	Random []RandomDrop
}

// FixedDrop represents a guaranteed drop
type FixedDrop struct {
	Item string
	Min  int
	Max  int
}

// RandomDrop represents a random drop
type RandomDrop struct {
	Item   string
	Chance float64
	Min    int
	Max    int
}

// NPC represents a non-player character
type NPC struct {
	ID                string
	Name              string
	Role              string
	Description       string
	Age               int
	Gender            string
	Appearance        string
	Background        string
	Personality       []string
	DefaultLocation   string
	Services          NPCServices
	Dialogue          Dialogue
	Affiliations      []string
	IsPlaceholder     bool   `json:"__placeholder__,omitempty"`
	PlaceholderSource string `json:"__source_phase__,omitempty"`
}

// NPCServices represents services an NPC provides
type NPCServices struct {
	Trade *TradeService
	Quest *QuestService
}

// TradeService represents trading service
type TradeService struct {
	BuyPriceModifier  float64
	SellPriceModifier float64
	AcceptsItems      []string
}

// QuestService represents quest service
type QuestService struct {
	Provides []string
}

// Dialogue represents NPC dialogue
type Dialogue struct {
	Greeting string
	Options  []DialogueOption
}

// DialogueOption represents a dialogue option
type DialogueOption struct {
	Text   string
	Action string
	Target string
}

// Storyline represents the story structure
type Storyline struct {
	Arcs     []Arc
	Chapters []Chapter
}

// Arc represents a story arc (volume/part)
type Arc struct {
	ID               string
	Name             string
	Position         int
	Chapters         []string
	UnlockCondition  string
	CompletionReward Reward
}

// Reward represents completion rewards
type Reward struct {
	Exp         int
	Title       string
	Description string
	Items       []string
	Unlocks     []string
	Flags       []string
}

// Chapter represents a story chapter
type Chapter struct {
	ID         string
	Title      string
	Arc        string
	Position   int
	Objectives []Objective
	Completion ChapterCompletion
}

// ChapterCompletion represents chapter completion rewards
type ChapterCompletion struct {
	Exp        int
	Items      []string
	Unlocks    []string
	StoryFlags []string
}

// Objective represents a chapter objective
type Objective struct {
	ID    string
	Name  string
	Type  string // sequence, parallel, optional
	Steps []Step
}

// Step represents an objective step
type Step struct {
	Order       int
	Description string
	Trigger     *Trigger
	Event       Event
}

// Trigger represents a step trigger condition
type Trigger struct {
	Type      string
	Condition string
	Location  string
}

// Event represents a game event
type Event struct {
	Type        string // spawn, move, combat, dialogue, acquire
	Trigger     *Trigger
	Require     *Requirement
	OnComplete  *EventResult
	OnFail      *EventResult
	StateDeltas []StateDelta

	// Type-specific fields
	Spawn    *SpawnEvent
	Move     *MoveEvent
	Combat   *CombatEvent
	Dialogue *DialogueEvent
	Acquire  *AcquireEvent
}

// StateDelta represents a structured narrative state transition.
type StateDelta struct {
	Target string
	Kind   string
	Field  string
	From   string
	To     string
	Delta  int
	Unit   string
	Cost   string
	Note   string
}

// Requirement represents event requirements
type Requirement struct {
	Flags []string
	Items []string
	Stats map[string]int
}

// EventResult represents event completion/failure results
type EventResult struct {
	Narration    string
	Exp          int
	Items        []string
	TriggerEvent string
	SetFlag      string
	UnlockStage  string
	Heal         int
	Result       string // for on_defeat: retry, game_over
}

// SpawnEvent represents a spawn event
type SpawnEvent struct {
	Actor    string
	Location string
	State    map[string]interface{}
}

// MoveEvent represents a move event
type MoveEvent struct {
	Actor  string
	From   string
	To     string
	Method string
}

// CombatEvent represents a combat event
type CombatEvent struct {
	Setup     CombatSetup
	Phases    []CombatPhase
	OnVictory *EventResult
	OnDefeat  *EventResult
}

// CombatSetup represents combat setup
type CombatSetup struct {
	Location    string
	Enemies     []EnemySpawn
	Environment map[string]interface{}
}

// EnemySpawn represents an enemy spawn in combat
type EnemySpawn struct {
	ID    string
	Count int
	Level int
	Elite bool
	Boss  bool
}

// CombatPhase represents a combat phase
type CombatPhase struct {
	Name      string
	Trigger   string
	Duration  string
	Modifiers map[string]interface{}
	Narration string
	OnTrigger *EventResult
}

// DialogueEvent represents a dialogue event
type DialogueEvent struct {
	Speaker string
	Text    string
	Choices []DialogueChoice
}

// DialogueChoice represents a dialogue choice
type DialogueChoice struct {
	Text   string
	Action string
}

// AcquireEvent represents an item acquisition event
type AcquireEvent struct {
	Actor    string
	Item     string
	Quantity int
	Source   string
}

// Systems represents game systems
type Systems struct {
	Progression  *Progression
	Breakthrough *Breakthrough
	SkillSystem  *SkillSystem
	Hooks        []Hook
	Triggers     []TriggerDef

	// Additional systems for DSL-RPG
	ProgressionSystems []ProgressionSystem `json:"progression_systems,omitempty"`
	Counters           []CounterSystem     `json:"counters,omitempty"`

	// Custom attribute and power system
	AttributeSystem *AttributeSystem `json:"attribute_system,omitempty"`
	PowerFormula    *PowerFormula    `json:"power_formula,omitempty"`
}

// ProgressionSystem represents a custom progression system
type ProgressionSystem struct {
	ID          string
	Name        string
	Description string
	Levels      []ProgressionLevel
}

// ProgressionLevel represents a level in a progression system
type ProgressionLevel struct {
	Level        int
	Name         string
	Requirements string
	Bonuses      []string
}

// CounterSystem represents a counter with milestones
type CounterSystem struct {
	Name        string
	Track       string
	Description string
	Milestones  []CounterMilestone
}

// CounterMilestone represents a milestone in a counter
type CounterMilestone struct {
	Value  int
	Reward Reward
}

// Progression represents the leveling system
type Progression struct {
	Type    string
	Formula ProgressionFormula
	LevelUp LevelUpRewards
}

// ProgressionFormula represents progression formulas
type ProgressionFormula struct {
	ExpToNext    string
	ExpFromEnemy string
	ExpFromQuest string
}

// LevelUpRewards represents level up rewards
type LevelUpRewards struct {
	StatPoints  int
	SkillPoints int
	HPRestore   string
	MPRestore   string
}

// Breakthrough represents the breakthrough system
type Breakthrough struct {
	ID     string
	Type   string
	Stages []BreakthroughStage
}

// BreakthroughStage represents a breakthrough stage
type BreakthroughStage struct {
	Name                string
	MaxLevel            int
	AttributeMultiplier float64
	Requirements        BreakthroughRequirements
	UnlockSkills        []string
	SpecialAbility      string
}

// BreakthroughRequirements represents stage requirements
type BreakthroughRequirements struct {
	Level       int
	Quest       string
	Item        string
	Achievement string
}

// SkillSystem represents the skill system
type SkillSystem struct {
	Learning SkillLearning
	Upgrade  SkillUpgrade
}

// SkillLearning represents skill learning rules
type SkillLearning struct {
	Method    string
	MaxSkills int
}

// SkillUpgrade represents skill upgrade rules
type SkillUpgrade struct {
	MaxLevel    int
	CostFormula string
}

// Hook represents a system hook
type Hook struct {
	ID        string
	EventType string // on_kill, on_damage_taken, on_skill_use, on_level_up
	Condition string
	Counters  []Counter
}

// Counter represents a statistics counter
type Counter struct {
	Name       string
	Filter     string
	Max        int
	Milestones []Milestone
}

// Milestone represents a counter milestone
type Milestone struct {
	Value   int
	Reward  Reward
	SetFlag string
}

// TriggerDef represents a trigger definition
type TriggerDef struct {
	ID         string
	Name       string
	Conditions []Condition
	OnTrigger  EventResult
	Once       bool
}

// Condition represents a trigger condition
type Condition struct {
	Type     string // stat, flag, counter, location, time, random
	Stat     string
	Op       string
	Value    interface{}
	Counter  string
	Location string
	Time     string
	Random   float64
}

// Validate performs basic validation on the DSL
func (d *DSL) Validate() error {
	if d == nil {
		return fmt.Errorf("dsl is required")
	}
	if d.Metadata == nil {
		return fmt.Errorf("metadata is required")
	}
	if d.Metadata.Title == "" {
		return fmt.Errorf("metadata.title is required")
	}
	if d.Characters == nil || d.Characters.Player == nil {
		return fmt.Errorf("at least one player character is required")
	}
	if d.Storyline == nil || len(d.Storyline.Chapters) == 0 {
		return fmt.Errorf("at least one chapter is required")
	}
	return nil
}

// GetPlayer returns the player character
func (d *DSL) GetPlayer() *Player {
	if d.Characters != nil {
		return d.Characters.Player
	}
	return nil
}

// GetChapter returns a chapter by ID
func (d *DSL) GetChapter(id string) *Chapter {
	for i := range d.Storyline.Chapters {
		if d.Storyline.Chapters[i].ID == id {
			return &d.Storyline.Chapters[i]
		}
	}
	return nil
}

// GetEnemy returns an enemy by ID
func (d *DSL) GetEnemy(id string) *Enemy {
	for i := range d.Characters.Enemies {
		if d.Characters.Enemies[i].ID == id {
			return &d.Characters.Enemies[i]
		}
	}
	return nil
}

// GetLocation returns a location by ID
func (d *DSL) GetLocation(id string) *Location {
	for i := range d.World.Locations {
		if d.World.Locations[i].ID == id {
			return &d.World.Locations[i]
		}
	}
	return nil
}

// GetNPC returns an NPC by ID
func (d *DSL) GetNPC(id string) *NPC {
	for i := range d.Characters.NPCs {
		if d.Characters.NPCs[i].ID == id {
			return &d.Characters.NPCs[i]
		}
	}
	return nil
}

// CountPlaceholders returns the number of placeholder elements
func (d *DSL) CountPlaceholders() map[string]int {
	counts := map[string]int{
		"player":    0,
		"enemies":   0,
		"npcs":      0,
		"locations": 0,
	}

	if d.Characters != nil {
		if d.Characters.Player != nil && d.Characters.Player.IsPlaceholder {
			counts["player"]++
		}
		for _, e := range d.Characters.Enemies {
			if e.IsPlaceholder {
				counts["enemies"]++
			}
		}
		for _, n := range d.Characters.NPCs {
			if n.IsPlaceholder {
				counts["npcs"]++
			}
		}
	}

	if d.World != nil {
		for _, l := range d.World.Locations {
			if l.IsPlaceholder {
				counts["locations"]++
			}
		}
	}

	return counts
}

// HasPlaceholders returns true if any element is a placeholder
func (d *DSL) HasPlaceholders() bool {
	counts := d.CountPlaceholders()
	for _, count := range counts {
		if count > 0 {
			return true
		}
	}
	return false
}

// GetPlaceholderList returns a list of all placeholder elements
func (d *DSL) GetPlaceholderList() []PlaceholderInfo {
	var placeholders []PlaceholderInfo

	if d.Characters != nil {
		if d.Characters.Player != nil && d.Characters.Player.IsPlaceholder {
			placeholders = append(placeholders, PlaceholderInfo{
				Type: "player",
				ID:   d.Characters.Player.ID,
				Name: d.Characters.Player.Name,
			})
		}
		for _, e := range d.Characters.Enemies {
			if e.IsPlaceholder {
				placeholders = append(placeholders, PlaceholderInfo{
					Type: "enemy",
					ID:   e.ID,
					Name: e.Name,
				})
			}
		}
		for _, n := range d.Characters.NPCs {
			if n.IsPlaceholder {
				placeholders = append(placeholders, PlaceholderInfo{
					Type: "npc",
					ID:   n.ID,
					Name: n.Name,
				})
			}
		}
	}

	if d.World != nil {
		for _, l := range d.World.Locations {
			if l.IsPlaceholder {
				placeholders = append(placeholders, PlaceholderInfo{
					Type: "location",
					ID:   l.ID,
					Name: l.Name,
				})
			}
		}
	}

	return placeholders
}

// ============================================
// Custom Attribute and Power System
// ============================================

// AttributeSystem defines custom attributes for the story
type AttributeSystem struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Attributes  []AttributeDef `json:"attributes"`
}

// AttributeDef defines a single custom attribute
type AttributeDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "resource" (灵气/法力), "stat" (属性), "special" (特殊)
	BaseValue   int    `json:"base_value"`
	MinValue    int    `json:"min_value,omitempty"`
	MaxValue    int    `json:"max_value,omitempty"`
	IsResource  bool   `json:"is_resource"` // 是否是消耗资源（如法力）
}

// PowerFormula defines how combat power is calculated
type PowerFormula struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Formula     string   `json:"formula"`    // 计算公式，如 "str*2 + agi*1 + spirit*3"
	BasePower   int      `json:"base_power"` // 基础战力
	Factors     []Factor `json:"factors"`    // 各属性系数
}

// Factor represents a single factor in power calculation
type Factor struct {
	Attribute string  `json:"attribute"` // 属性ID
	Name      string  `json:"name"`      // 属性名称
	Weight    float64 `json:"weight"`    // 权重系数
}

// ExtendedProgressionLevel extends ProgressionLevel with attribute bonuses
type ExtendedProgressionLevel struct {
	ProgressionLevel
	AttributeBonuses map[string]int `json:"attribute_bonuses,omitempty"` // 属性奖励: {"灵气": 10, "力量": 2}
	PowerBonus       int            `json:"power_bonus,omitempty"`       // 战力直接奖励
}

// GetAttributeValue returns the value of a custom attribute from player
func (d *DSL) GetAttributeValue(attrID string) int {
	if d.Systems == nil || d.Systems.AttributeSystem == nil {
		return 0
	}
	// 默认返回基础值，实际应从player的自定义属性中获取
	for _, attr := range d.Systems.AttributeSystem.Attributes {
		if attr.ID == attrID {
			return attr.BaseValue
		}
	}
	return 0
}

// CalculatePower calculates power using custom formula
func (d *DSL) CalculatePower(stats Stats, customAttrs map[string]int) int {
	if d.Systems == nil || d.Systems.PowerFormula == nil {
		// 使用默认计算
		return stats.STR*2 + stats.AGI*1 + stats.INT*2 + stats.VIT*1 + stats.HP/10
	}

	formula := d.Systems.PowerFormula
	power := formula.BasePower

	// 根据factors计算
	for _, factor := range formula.Factors {
		value := 0
		switch factor.Attribute {
		case "str":
			value = stats.STR
		case "agi":
			value = stats.AGI
		case "int":
			value = stats.INT
		case "vit":
			value = stats.VIT
		case "hp":
			value = stats.HP
		case "mp":
			value = stats.MP
		default:
			// 自定义属性
			if customAttrs != nil {
				value = customAttrs[factor.Attribute]
			}
		}
		power += int(float64(value) * factor.Weight)
	}

	return power
}
