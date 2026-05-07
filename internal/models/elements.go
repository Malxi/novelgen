package models

// Character represents a detailed character profile
// Note: Dynamic fields like relationships, goals, character_arc are managed by StateMatrix
type Character struct {
	Name         string   `json:"name" desc:"Character name"`
	Aliases      []string `json:"aliases,omitempty" desc:"Alternative names or titles (array of strings)"`
	Age          string   `json:"age,omitempty" desc:"Character age or age range"`
	Gender       string   `json:"gender,omitempty" desc:"Character gender"`
	Race         string   `json:"race,omitempty" desc:"Character race/species"`
	Appearance   string   `json:"appearance" desc:"Physical appearance description"`
	Personality  []string `json:"personality" desc:"Personality traits (array of strings)"`
	Background   string   `json:"background" desc:"Character backstory"`
	Motivation   string   `json:"motivation" desc:"Character's core motivation"`
	Skills       []string `json:"skills,omitempty" desc:"Skills and abilities (array of strings)"`
	Abilities    []string `json:"abilities,omitempty" desc:"Special powers or abilities (array of strings)"`
	Affiliations []string `json:"affiliations,omitempty" desc:"Organizations the character belongs to (array of strings)"`
	RoleInStory  string   `json:"role_in_story" desc:"Character's role in the story (protagonist/antagonist/supporting/etc)"`
	Voice        string   `json:"voice,omitempty" desc:"Speaking style and mannerisms"`
	Notes        string   `json:"notes,omitempty" desc:"Additional notes for writers (string, NOT array)"`

	// Optional RPG/DSL metadata. These fields describe static simulation defaults;
	// dynamic progress is still owned by outline events, recaps, and StateMatrix.
	RPGRole      string             `json:"rpg_role,omitempty" desc:"RPG role: player/npc/ally/enemy/boss/mentor/vendor/quest_giver"`
	CombatRole   string             `json:"combat_role,omitempty" desc:"Combat function such as striker/tank/support/controller/noncombat"`
	PowerLevel   int                `json:"power_level,omitempty" desc:"Approximate starting power level for simulation"`
	RPGStats     *CraftRPGStats     `json:"rpg_stats,omitempty" desc:"Optional RPG stat defaults for DSL simulation"`
	DSLTags      []string           `json:"dsl_tags,omitempty" desc:"Stable tags used by RPG DSL conversion"`
	StateEffects []CraftStateEffect `json:"state_effects,omitempty" desc:"Static state effects this character implies when introduced"`
}

// Location represents a detailed location description
type Location struct {
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Description        string          `json:"description"`
	Appearance         string          `json:"appearance"`
	Atmosphere         string          `json:"atmosphere"`
	SensoryDetails     *SensoryDetails `json:"sensory_details,omitempty"`
	Significance       string          `json:"significance"`
	History            string          `json:"history,omitempty"`
	Inhabitants        []string        `json:"inhabitants,omitempty"`
	ConnectedLocations []string        `json:"connected_locations,omitempty"`
	Events             []string        `json:"events,omitempty"`
	Secrets            string          `json:"secrets,omitempty"`
	Notes              string          `json:"notes,omitempty"`

	RPGMapType    string             `json:"rpg_map_type,omitempty" desc:"DSL map type: city/dungeon/base/region/battlefield/indoor/outdoor"`
	DangerLevel   int                `json:"danger_level,omitempty" desc:"Approximate encounter danger level for simulation"`
	EncounterTags []string           `json:"encounter_tags,omitempty" desc:"Encounter tags available in this location"`
	ResourceTags  []string           `json:"resource_tags,omitempty" desc:"Resource tags available in this location"`
	DSLTags       []string           `json:"dsl_tags,omitempty" desc:"Stable tags used by RPG DSL conversion"`
	StateEffects  []CraftStateEffect `json:"state_effects,omitempty" desc:"Static state effects this location implies when entered"`
}

// SensoryDetails contains sensory information about a location
type SensoryDetails struct {
	Sights   []string `json:"sights,omitempty"`
	Sounds   []string `json:"sounds,omitempty"`
	Smells   []string `json:"smells,omitempty"`
	Textures []string `json:"textures,omitempty"`
}

// Item represents a detailed item description
type Item struct {
	Name         string     `json:"name" desc:"Item name"`
	Type         string     `json:"type" desc:"Item type/category"`
	Description  string     `json:"description" desc:"Item description"`
	Appearance   string     `json:"appearance" desc:"Physical appearance"`
	Function     string     `json:"function" desc:"What the item does"`
	Origin       string     `json:"origin,omitempty" desc:"Where the item comes from"`
	History      string     `json:"history,omitempty" desc:"Item's history/background"`
	Powers       []string   `json:"powers,omitempty" desc:"Special powers (array of strings)"`
	Limitations  []string   `json:"limitations,omitempty" desc:"Limitations or drawbacks (array of strings)"`
	Owner        string     `json:"owner,omitempty" desc:"Current owner"`
	Significance string     `json:"significance" desc:"Story significance"`
	RelatedItems []string   `json:"related_items,omitempty" desc:"Related items (array of strings)"`
	Secrets      StringList `json:"secrets,omitempty" desc:"Secrets about the item (array of strings)"`
	Notes        string     `json:"notes,omitempty" desc:"Additional notes (string, NOT array)"`

	RPGItemType      string             `json:"rpg_item_type,omitempty" desc:"DSL item type: weapon/armor/consumable/artifact/document/resource/key/material"`
	Rarity           string             `json:"rarity,omitempty" desc:"RPG rarity: common/uncommon/rare/epic/legendary/unique"`
	PowerLevel       int                `json:"power_level,omitempty" desc:"Approximate item power level for simulation"`
	QuantityTracking bool               `json:"quantity_tracking,omitempty" desc:"Whether the item should be tracked as a quantity/resource"`
	DSLTags          []string           `json:"dsl_tags,omitempty" desc:"Stable tags used by RPG DSL conversion"`
	StateEffects     []CraftStateEffect `json:"state_effects,omitempty" desc:"State effects caused when the item is acquired or used"`
}

// CraftRPGStats contains optional static stat defaults for RPG/DSL conversion.
type CraftRPGStats struct {
	STR   int `json:"str,omitempty" desc:"Strength"`
	AGI   int `json:"agi,omitempty" desc:"Agility"`
	INT   int `json:"int,omitempty" desc:"Intelligence"`
	VIT   int `json:"vit,omitempty" desc:"Vitality"`
	HP    int `json:"hp,omitempty" desc:"Hit points"`
	MP    int `json:"mp,omitempty" desc:"Mind/energy/mana points"`
	Level int `json:"level,omitempty" desc:"Starting level"`
}

// CraftStateEffect is a compact, DSL-compatible state delta emitted by craft.
type CraftStateEffect struct {
	Target string `json:"target,omitempty" desc:"State target, e.g. protagonist, item id, faction"`
	Kind   string `json:"kind,omitempty" desc:"State kind, e.g. item, resource, cultivation, relationship"`
	Field  string `json:"field,omitempty" desc:"State field affected"`
	From   string `json:"from,omitempty" desc:"Previous value if known"`
	To     string `json:"to,omitempty" desc:"New value or state"`
	Delta  int    `json:"delta,omitempty" desc:"Numeric delta when applicable"`
	Unit   string `json:"unit,omitempty" desc:"Unit for delta/value"`
	Cost   string `json:"cost,omitempty" desc:"Cost or limitation"`
	Note   string `json:"note,omitempty" desc:"Short explanation"`
}

// NormalizeForCraft clamps optional simulation metadata while preserving old JSON.
func (c *Character) NormalizeForCraft(name string) {
	if c.Name == "" {
		c.Name = name
	}
	c.PowerLevel = clampNonNegative(c.PowerLevel)
	normalizeRPGStats(c.RPGStats)
	c.DSLTags = compactStringList(c.DSLTags)
	c.StateEffects = compactStateEffects(c.StateEffects)
}

// NormalizeForCraft clamps optional simulation metadata while preserving old JSON.
func (l *Location) NormalizeForCraft(name string) {
	if l.Name == "" {
		l.Name = name
	}
	l.DangerLevel = clampNonNegative(l.DangerLevel)
	l.EncounterTags = compactStringList(l.EncounterTags)
	l.ResourceTags = compactStringList(l.ResourceTags)
	l.DSLTags = compactStringList(l.DSLTags)
	l.StateEffects = compactStateEffects(l.StateEffects)
}

// NormalizeForCraft clamps optional simulation metadata while preserving old JSON.
func (i *Item) NormalizeForCraft(name string) {
	if i.Name == "" {
		i.Name = name
	}
	i.PowerLevel = clampNonNegative(i.PowerLevel)
	i.DSLTags = compactStringList(i.DSLTags)
	i.StateEffects = compactStateEffects(i.StateEffects)
}

func normalizeRPGStats(stats *CraftRPGStats) {
	if stats == nil {
		return
	}
	stats.STR = clampNonNegative(stats.STR)
	stats.AGI = clampNonNegative(stats.AGI)
	stats.INT = clampNonNegative(stats.INT)
	stats.VIT = clampNonNegative(stats.VIT)
	stats.HP = clampNonNegative(stats.HP)
	stats.MP = clampNonNegative(stats.MP)
	stats.Level = clampNonNegative(stats.Level)
}

func clampNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func compactStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func compactStateEffects(effects []CraftStateEffect) []CraftStateEffect {
	if len(effects) == 0 {
		return nil
	}
	out := make([]CraftStateEffect, 0, len(effects))
	for _, effect := range effects {
		if effect.Target == "" && effect.Kind == "" && effect.Field == "" && effect.To == "" && effect.Delta == 0 {
			continue
		}
		out = append(out, effect)
	}
	return out
}

// Organization represents a faction, guild, or organization in the story
type Organization struct {
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	Description  string             `json:"description"`
	Founding     string             `json:"founding,omitempty"`
	Headquarters string             `json:"headquarters,omitempty"`
	Leadership   string             `json:"leadership,omitempty"`
	Members      []string           `json:"members,omitempty"`
	Goals        []string           `json:"goals"`
	Ideology     string             `json:"ideology,omitempty"`
	Resources    []string           `json:"resources,omitempty"`
	Allies       []string           `json:"allies,omitempty"`
	Enemies      []string           `json:"enemies,omitempty"`
	Reputation   string             `json:"reputation,omitempty"`
	Structure    string             `json:"structure,omitempty"`
	Significance string             `json:"significance"`
	Secrets      string             `json:"secrets,omitempty"`
	Notes        string             `json:"notes,omitempty"`
	DSLTags      []string           `json:"dsl_tags,omitempty" desc:"Stable tags used by RPG DSL conversion"`
	StateEffects []CraftStateEffect `json:"state_effects,omitempty" desc:"Static state effects this organization implies when introduced"`
}

// NormalizeForCraft clamps optional metadata while preserving old JSON.
func (o *Organization) NormalizeForCraft(name string) {
	if o.Name == "" {
		o.Name = name
	}
	o.Members = compactStringList(o.Members)
	o.Goals = compactStringList(o.Goals)
	o.Resources = compactStringList(o.Resources)
	o.Allies = compactStringList(o.Allies)
	o.Enemies = compactStringList(o.Enemies)
	o.DSLTags = compactStringList(o.DSLTags)
	o.StateEffects = compactStateEffects(o.StateEffects)
}

// Race represents a species or race in the story world
type Race struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Appearance   string   `json:"appearance"`
	Traits       []string `json:"traits"`
	Abilities    []string `json:"abilities,omitempty"`
	Weaknesses   []string `json:"weaknesses,omitempty"`
	Lifespan     string   `json:"lifespan,omitempty"`
	Culture      string   `json:"culture,omitempty"`
	Society      string   `json:"society,omitempty"`
	Habitat      string   `json:"habitat,omitempty"`
	Diet         string   `json:"diet,omitempty"`
	Reproduction string   `json:"reproduction,omitempty"`
	Language     string   `json:"language,omitempty"`
	Relations    []string `json:"relations,omitempty"`
	History      string   `json:"history,omitempty"`
	Significance string   `json:"significance"`
	Notes        string   `json:"notes,omitempty"`
}

// AbilitySystem represents a magic system, cultivation system, or skill system
type AbilitySystem struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Source        string   `json:"source,omitempty"`
	Mechanics     string   `json:"mechanics"`
	Levels        []string `json:"levels,omitempty"`
	Requirements  []string `json:"requirements,omitempty"`
	Limitations   []string `json:"limitations,omitempty"`
	Costs         []string `json:"costs,omitempty"`
	Applications  []string `json:"applications,omitempty"`
	Practitioners []string `json:"practitioners,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
	RelatedItems  []string `json:"related_items,omitempty"`
	Significance  string   `json:"significance"`
	Notes         string   `json:"notes,omitempty"`
}

// WorldLore represents world-building elements like history, culture, or rules
type WorldLore struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Description     string   `json:"description"`
	Content         string   `json:"content"`
	Origin          string   `json:"origin,omitempty"`
	Significance    string   `json:"significance"`
	RelatedElements []string `json:"related_elements,omitempty"`
	Notes           string   `json:"notes,omitempty"`
}
