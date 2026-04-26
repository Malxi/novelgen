package models

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Outline represents the complete story outline with 3-level structure (parts → volumes → chapters)
type Outline struct {
	Parts []Part `json:"parts" md:"parts"`
}

// Part represents a major section of the story
type Part struct {
	ID      string   `json:"id" md:"-"` // ID not shown in markdown
	Title   string   `json:"title" md:"title"`
	Summary string   `json:"summary" md:"heading"`
	Volumes []Volume `json:"volumes" md:"volumes"`
}

// Volume represents a subdivision of a part
type Volume struct {
	ID       string    `json:"id" md:"-"` // ID not shown in markdown
	Title    string    `json:"title" md:"title"`
	Summary  string    `json:"summary" md:"heading"`
	Chapters []Chapter `json:"chapters" md:"chapters"`
}

// ChapterTimeline represents timeline information for a chapter
type ChapterTimeline struct {
	Anchor      string `json:"anchor,omitempty" md:"anchor" desc:"相对于故事开始的时间点，如：第3天傍晚、三个月后"`
	StartTime   string `json:"start_time,omitempty" md:"start_time" desc:"章节开始的具体时间描述"`
	EndTime     string `json:"end_time,omitempty" md:"end_time" desc:"章节结束的具体时间描述"`
	Duration    string `json:"duration,omitempty" md:"duration" desc:"章节内经过的时间：当天、持续3天、一瞬间"`
	TimeJump    bool   `json:"time_jump,omitempty" md:"time_jump" desc:"是否是时间跳跃（相对于上一章）"`
	PreviousGap string `json:"previous_gap,omitempty" md:"previous_gap" desc:"与上一章的时间间隔，如：过了7天、次日清晨"`
	Transition  string `json:"transition,omitempty" md:"transition" desc:"时间过渡说明，解释这段时间发生了什么"`
}

// StateAnchor declares the expected protagonist state at chapter start.
// Write agent uses this as a target; validator checks cross-chapter consistency.
type StateAnchor struct {
	Cultivation string   `json:"cultivation,omitempty" md:"cultivation" desc:"修炼境界/能力等级，如：炼气三层、S级进化者一阶"`
	SpiritStones int     `json:"spirit_stones,omitempty" md:"spirit_stones" desc:"灵石/资源数量"`
	Allies      []string `json:"allies,omitempty" md:"allies" desc:"当前盟友/同伴"`
	Injuries    []string `json:"injuries,omitempty" md:"injuries" desc:"当前伤势"`
	Location    string   `json:"location,omitempty" md:"location" desc:"章节开始时的位置"`
	KeyItems    []string `json:"key_items,omitempty" md:"key_items" desc:"当前持有的关键物品"`
	Notes       string   `json:"notes,omitempty" md:"notes" desc:"其他值得追踪的状态"`
}

// Chapter represents a single chapter in the story
type Chapter struct {
	ID          string          `json:"id" md:"-"` // ID not shown in markdown
	Title       string          `json:"title" md:"title"`
	Summary     string          `json:"summary" md:"heading"`       // 格式: 角色 在 什么地方 发生了 什么事
	Characters  []string        `json:"characters" md:"characters"` // 本章出现的角色名列表
	Location    string          `json:"location" md:"location"`     // 事情发生的地点
	Events      []Event         `json:"events" md:"events"`         // 本章发生的事件
	Beats       []string        `json:"beats" md:"beats"`
	OpeningBeat string          `json:"opening_beat,omitempty" desc:"First beat that continues the previous chapter"`
	ClosingBeat string          `json:"closing_beat,omitempty" desc:"Final beat/hook that must lead into next chapter"`
	StateChange string          `json:"state_change,omitempty" desc:"Primary change this chapter causes; must map to Events"`
	Conflict    string          `json:"conflict" md:"conflict"`
	Pacing      string          `json:"pacing" md:"pacing"`
	Timeline    ChapterTimeline `json:"timeline,omitempty" md:"timeline" desc:"章节时间线信息"`
	StateAnchor StateAnchor     `json:"state_anchor,omitempty" md:"state_anchor" desc:"章节开始时的主角状态锚点"`
}

// Event represents a story event that changes state
type Event struct {
	// === 旧字段（向后兼容）===
	Type       string   `json:"type" md:"type" desc:"Event type: relationship (角色关系变化), goal (角色目标更新), item (角色物品更新), premise (角色体系更新), storyline (故事线更新), gate (章节障碍/代价记录), status (角色状态变化 - subject应为状态类型如修为/伤势/身份)"`
	Characters []string `json:"characters" md:"characters" desc:"涉及的角色列表"`
	Subject    string   `json:"subject" md:"subject" desc:"目标角色/物品/体系/故事线/状态类型。对于status类型，subject应为状态类型（如修为、伤势、身份），而非角色名"`
	Change     string   `json:"change" md:"change" desc:"变化描述 (started, progressed, completed, get, lost, resolved等)"`
	Details    string   `json:"details,omitempty" md:"details" desc:"额外详情，用于 storyline 进度描述等"`

	// === 新字段（推荐，语义更清晰）===
	// Actor 动作执行者（主语）
	Actor string `json:"actor,omitempty" md:"actor" desc:"执行动作的角色（主语），如：林跃"`
	// Action 动作类型（谓语）
	Action string `json:"action,omitempty" md:"action" desc:"动作类型（谓语）：acquire(获得)/use(使用)/lose(失去)/move(移动)/combat(战斗)/learn(学习)/discover(发现)/transform(转变)/meet(遇见)/establish(建立)"`
	// Target 动作目标（宾语）
	Target string `json:"target,omitempty" md:"target" desc:"动作的目标对象（宾语），如：生存刀、星芒机甲"`
	// TargetType 目标类型
	TargetType string `json:"target_type,omitempty" md:"target_type" desc:"目标类型：item(物品)/character(角色)/location(地点)/skill(技能)/status(状态)/relationship(关系)/knowledge(知识)"`
	// Context 上下文（可选）
	Context string `json:"context,omitempty" md:"context" desc:"动作发生的上下文，如：低温设施、废土地表"`
	// Result 结果描述（自然语言，可选）
	Result string `json:"result,omitempty" md:"result" desc:"动作结果的描述，用于 AI 理解"`
}

// Action type constants for the new Event structure
const (
	// 物品相关
	ActionAcquire = "acquire" // 获得物品
	ActionUse     = "use"     // 使用物品
	ActionLose    = "lose"    // 失去物品
	ActionCraft   = "craft"   // 制造/合成物品

	// 移动相关
	ActionMove     = "move"     // 移动位置
	ActionEnter    = "enter"    // 进入地点
	ActionLeave    = "leave"    // 离开地点
	ActionTeleport = "teleport" // 传送/跃迁

	// 战斗相关
	ActionCombat = "combat" // 进入战斗
	ActionDefeat = "defeat" // 击败目标
	ActionEscape = "escape" // 逃离战斗
	ActionDefend = "defend" // 防御

	// 能力相关
	ActionLearn   = "learn"   // 学习技能/知识
	ActionAwaken  = "awaken"  // 觉醒能力
	ActionUpgrade = "upgrade" // 升级能力
	ActionMaster  = "master"  // 掌握能力

	// 发现相关
	ActionDiscover = "discover" // 发现/探索
	ActionReveal   = "reveal"   // 揭示/揭露

	// 关系相关
	ActionMeet      = "meet"      // 遇见角色
	ActionBefriend  = "befriend"  // 建立友谊
	ActionBetray    = "betray"    // 背叛
	ActionReconcile = "reconcile" // 和解

	// 状态相关
	ActionTransform = "transform" // 转变/变身
	ActionRecover   = "recover"   // 恢复
	ActionAfflict   = "afflict"   // 受到负面状态

	// 目标相关
	ActionSet      = "set"      // 设定目标
	ActionProgress = "progress" // 推进目标
	ActionAchieve  = "achieve"  // 达成目标
	ActionAbandon  = "abandon"  // 放弃目标
)

// Target type constants
const (
	TargetTypeItem         = "item"         // 物品
	TargetTypeCharacter    = "character"    // 角色
	TargetTypeLocation     = "location"     // 地点
	TargetTypeSkill        = "skill"        // 技能
	TargetTypeStatus       = "status"       // 状态
	TargetTypeRelationship = "relationship" // 关系
	TargetTypeKnowledge    = "knowledge"    // 知识
	TargetTypeGoal         = "goal"         // 目标
	TargetTypePremise      = "premise"      // 能力/体系
	TargetTypeStoryline    = "storyline"    // 故事线
)

// Event type constants
const (
	EventTypeRelationship = "relationship" // (relationship, characterA, characterB, change) 角色关系变化
	EventTypeGoal         = "goal"         // (goal, character, change) 角色目标更新
	EventTypeItem         = "item"         // (item, character, get/lost) 角色物品更新
	EventTypePremise      = "premise"      // (premise, character, change) 角色体系更新
	EventTypeStoryline    = "storyline"    // (storyline, change) 故事线更新
	EventTypeGate         = "gate"         // (gate, character, change) 章节障碍/代价记录
	EventTypeStatus       = "status"       // (status, character, statusType, change) 角色状态变化，subject 应为状态类型（如"修为"、"伤势"、"身份"）
)

// Save writes the outline to a file
func (o *Outline) Save(path string) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadOutline reads the outline from a file
func LoadOutline(path string) (*Outline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var outline Outline
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, err
	}
	return &outline, nil
}

// GetChapterByID finds a chapter by its ID
func (o *Outline) GetChapterByID(id string) *Chapter {
	for _, part := range o.Parts {
		for _, volume := range part.Volumes {
			for i := range volume.Chapters {
				if volume.Chapters[i].ID == id {
					return &volume.Chapters[i]
				}
			}
		}
	}
	return nil
}

// GetVolumeByID finds a volume by its ID
func (o *Outline) GetVolumeByID(id string) *Volume {
	for _, part := range o.Parts {
		for i := range part.Volumes {
			if part.Volumes[i].ID == id {
				return &part.Volumes[i]
			}
		}
	}
	return nil
}

// GetPartByID finds a part by its ID
func (o *Outline) GetPartByID(id string) *Part {
	for i := range o.Parts {
		if o.Parts[i].ID == id {
			return &o.Parts[i]
		}
	}
	return nil
}

// ToMarkdown converts the outline to markdown format using reflection
func (o *Outline) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Story Outline\n\n")

	for _, part := range o.Parts {
		sb.WriteString(part.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts part to markdown
func (p *Part) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", p.Title))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", p.Summary))

	for _, volume := range p.Volumes {
		sb.WriteString(volume.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts volume to markdown
func (v *Volume) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", v.Title))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", v.Summary))

	for _, chapter := range v.Chapters {
		sb.WriteString(chapter.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts chapter to markdown
func (c *Chapter) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#### %s\n\n", c.Title))

	// Summary
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", c.Summary))

	// Characters
	if len(c.Characters) > 0 {
		sb.WriteString(fmt.Sprintf("**Characters:** %s\n\n", strings.Join(c.Characters, ", ")))
	}

	// Location
	if c.Location != "" {
		sb.WriteString(fmt.Sprintf("**Location:** %s\n\n", c.Location))
	}

	// Events
	if len(c.Events) > 0 {
		sb.WriteString("**Events:**\n")
		for _, event := range c.Events {
			sb.WriteString(event.ToMarkdown())
		}
		sb.WriteString("\n")
	}

	// Beats
	if len(c.Beats) > 0 {
		sb.WriteString("**Beats:**\n")
		for i, beat := range c.Beats {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, beat))
		}
		sb.WriteString("\n")
	}

	// Continuity anchors
	if c.OpeningBeat != "" {
		sb.WriteString(fmt.Sprintf("**Opening Beat:** %s\n\n", c.OpeningBeat))
	}
	if c.ClosingBeat != "" {
		sb.WriteString(fmt.Sprintf("**Closing Beat:** %s\n\n", c.ClosingBeat))
	}
	if c.StateChange != "" {
		sb.WriteString(fmt.Sprintf("**State Change:** %s\n\n", c.StateChange))
	}

	// Conflict
	if c.Conflict != "" {
		sb.WriteString(fmt.Sprintf("**Conflict:** %s\n\n", c.Conflict))
	}

	// Pacing
	if c.Pacing != "" {
		sb.WriteString(fmt.Sprintf("**Pacing:** %s\n\n", c.Pacing))
	}

	sb.WriteString("---\n\n")

	return sb.String()
}

// ToMarkdown converts event to markdown
func (e *Event) ToMarkdown() string {
	var sb strings.Builder

	// 优先使用新格式（如果 Actor 和 Action 都有值）
	if e.Actor != "" && e.Action != "" {
		sb.WriteString(fmt.Sprintf("- **%s** → %s", e.Actor, e.Action))

		if e.Target != "" {
			sb.WriteString(fmt.Sprintf(" → %s", e.Target))
			if e.TargetType != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", e.TargetType))
			}
		}

		if e.Context != "" {
			sb.WriteString(fmt.Sprintf(" @ %s", e.Context))
		}

		if e.Result != "" {
			sb.WriteString(fmt.Sprintf(" | %s", e.Result))
		}
	} else {
		// 使用旧格式
		sb.WriteString(fmt.Sprintf("- **%s**", e.Type))

		if len(e.Characters) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(e.Characters, ", ")))
		}

		if e.Subject != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", e.Subject))
		}

		if e.Change != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Change))
		}

		if e.Details != "" {
			sb.WriteString(fmt.Sprintf(" - %s", e.Details))
		}
	}

	sb.WriteString("\n")

	return sb.String()
}

// HasNewFormat checks if this event uses the new format (Actor + Action)
func (e *Event) HasNewFormat() bool {
	return e.Actor != "" && e.Action != ""
}

// GetActor returns the actor (prefer new format, fallback to old)
func (e *Event) GetActor() string {
	if e.Actor != "" {
		return e.Actor
	}
	if len(e.Characters) > 0 {
		return e.Characters[0]
	}
	return ""
}

// GetAction returns the action (prefer new format, fallback to old)
func (e *Event) GetAction() string {
	if e.Action != "" {
		return e.Action
	}
	// 从旧格式推断
	return inferActionFromOldFormat(e.Type, e.Change)
}

// GetTarget returns the target (prefer new format, fallback to old)
func (e *Event) GetTarget() string {
	if e.Target != "" {
		return e.Target
	}
	return e.Subject
}

// GetTargetType returns the target type (prefer new format, fallback to old)
func (e *Event) GetTargetType() string {
	if e.TargetType != "" {
		return e.TargetType
	}
	// 从旧格式推断
	return inferTargetTypeFromOldFormat(e.Type)
}

// inferActionFromOldFormat infers action from old event format
func inferActionFromOldFormat(eventType, change string) string {
	switch change {
	case "acquired", "get", "获得":
		return ActionAcquire
	case "lost", "失去":
		return ActionLose
	case "used", "使用":
		return ActionUse
	case "discovered", "发现":
		return ActionDiscover
	case "awakened", "awaken", "觉醒":
		return ActionAwaken
	case "upgraded", "升级":
		return ActionUpgrade
	case "completed", "完成":
		return ActionAchieve
	case "started", "开始":
		return ActionSet
	case "progressed", "推进":
		return ActionProgress
	default:
		// 根据 eventType 推断
		switch eventType {
		case EventTypeItem:
			return ActionAcquire
		case EventTypeStatus:
			return ActionTransform
		case EventTypePremise:
			return ActionAwaken
		default:
			return ActionDiscover
		}
	}
}

// inferTargetTypeFromOldFormat infers target type from old event format
func inferTargetTypeFromOldFormat(eventType string) string {
	switch eventType {
	case EventTypeItem:
		return TargetTypeItem
	case EventTypeStatus:
		return TargetTypeStatus
	case EventTypeRelationship:
		return TargetTypeRelationship
	case EventTypeGoal:
		return TargetTypeGoal
	case EventTypePremise:
		return TargetTypePremise
	case EventTypeStoryline:
		return TargetTypeStoryline
	default:
		return ""
	}
}
