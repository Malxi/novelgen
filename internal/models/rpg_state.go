package models

// RPGState is the structured, simulation-friendly version of StateMatrix.
// It keeps writer-facing continuity data while making numeric/typed state explicit.
type RPGState struct {
	CurrentChapter  string                        `json:"current_chapter,omitempty"`
	CurrentLocation string                        `json:"current_location,omitempty"`
	Characters      map[string]*RPGCharacterState `json:"characters,omitempty"`
	Resources       map[string]*RPGResourceState  `json:"resources,omitempty"`
	Relationships   map[string]*RPGRelationState  `json:"relationships,omitempty"`
	Storylines      map[string]*RPGQuestState     `json:"storylines,omitempty"`
	Systems         map[string]*RPGSystemState    `json:"systems,omitempty"`
	Flags           map[string]string             `json:"flags,omitempty"`
	Timeline        []RPGTimelineEntry            `json:"timeline,omitempty"`
	Deltas          []RPGStateDelta               `json:"deltas,omitempty"`
}

type RPGCharacterState struct {
	ID            string            `json:"id,omitempty"`
	Name          string            `json:"name,omitempty"`
	Role          string            `json:"role,omitempty"`
	Location      string            `json:"location,omitempty"`
	Alive         bool              `json:"alive"`
	Realm         string            `json:"realm,omitempty"`
	Level         int               `json:"level,omitempty"`
	HP            int               `json:"hp,omitempty"`
	MP            int               `json:"mp,omitempty"`
	Status        map[string]string `json:"status,omitempty"`
	Inventory     map[string]int    `json:"inventory,omitempty"`
	Goals         []string          `json:"goals,omitempty"`
	Knowledge     []string          `json:"knowledge,omitempty"`
	LastChangedAt string            `json:"last_changed_at,omitempty"`
}

type RPGResourceState struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Owner         string `json:"owner,omitempty"`
	Quantity      int    `json:"quantity,omitempty"`
	Status        string `json:"status,omitempty"`
	LastChangedAt string `json:"last_changed_at,omitempty"`
	Details       string `json:"details,omitempty"`
}

type RPGRelationState struct {
	From          string `json:"from,omitempty"`
	To            string `json:"to,omitempty"`
	Status        string `json:"status,omitempty"`
	Value         int    `json:"value,omitempty"`
	LastChangedAt string `json:"last_changed_at,omitempty"`
	Details       string `json:"details,omitempty"`
}

type RPGQuestState struct {
	ID              string              `json:"id,omitempty"`
	Name            string              `json:"name,omitempty"`
	Description     string              `json:"description,omitempty"`
	Status          string              `json:"status,omitempty"`
	Progress        string              `json:"progress,omitempty"`
	ProgressHistory []StorylineProgress `json:"progress_history,omitempty"`
}

type RPGSystemState struct {
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name,omitempty"`
	Type    string            `json:"type,omitempty"`
	Level   int               `json:"level,omitempty"`
	Status  string            `json:"status,omitempty"`
	Rules   []string          `json:"rules,omitempty"`
	Values  map[string]string `json:"values,omitempty"`
	Details string            `json:"details,omitempty"`
}

type RPGTimelineEntry struct {
	ChapterID string `json:"chapter_id,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Action    string `json:"action,omitempty"`
	Target    string `json:"target,omitempty"`
	Result    string `json:"result,omitempty"`
}

type RPGStateDelta struct {
	ChapterID string `json:"chapter_id,omitempty"`
	Target    string `json:"target,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Field     string `json:"field,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Delta     int    `json:"delta,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Cost      string `json:"cost,omitempty"`
	Note      string `json:"note,omitempty"`
}
