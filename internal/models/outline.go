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
	ID             string                `json:"id" md:"-"` // ID not shown in markdown
	Title          string                `json:"title" md:"title"`
	Summary        string                `json:"summary" md:"heading"`
	PayoffContract *VolumePayoffContract `json:"payoff_contract,omitempty" md:"payoff_contract" desc:"可选，卷级爽点兑现契约"`
	Chapters       []Chapter             `json:"chapters" md:"chapters"`
}

// VolumePayoffContract defines the reader promise and satisfying payoff for a volume.
type VolumePayoffContract struct {
	VolumeQuestion      string `json:"volume_question,omitempty" md:"volume_question" desc:"本卷驱动读者追看的核心问题"`
	PowerPromise        string `json:"power_promise,omitempty" md:"power_promise" desc:"本卷承诺展示的能力爽点或独特设定用法"`
	MainOpponentMisread string `json:"main_opponent_misread,omitempty" md:"main_opponent_misread" desc:"本卷主要对手对主角、规则或局势的误判"`
	BigWin              string `json:"big_win,omitempty" md:"big_win" desc:"本卷最终要落地的爽点大赢画面"`
	VisibleReward       string `json:"visible_reward,omitempty" md:"visible_reward" desc:"读者能看见的实际收益：资源、地位、地盘、秘密、盟友等"`
	ReputationShift     string `json:"reputation_shift,omitempty" md:"reputation_shift" desc:"赢后外界对主角的看法如何改变"`
	NextBiggerGame      string `json:"next_bigger_game,omitempty" md:"next_bigger_game" desc:"赢后露出的下一层更大局"`
}

func (p *VolumePayoffContract) IsZero() bool {
	if p == nil {
		return true
	}
	return strings.TrimSpace(p.VolumeQuestion) == "" &&
		strings.TrimSpace(p.PowerPromise) == "" &&
		strings.TrimSpace(p.MainOpponentMisread) == "" &&
		strings.TrimSpace(p.BigWin) == "" &&
		strings.TrimSpace(p.VisibleReward) == "" &&
		strings.TrimSpace(p.ReputationShift) == "" &&
		strings.TrimSpace(p.NextBiggerGame) == ""
}

// MergeVolumePayoffContract overlays non-empty patch fields onto base.
func MergeVolumePayoffContract(base, patch *VolumePayoffContract) *VolumePayoffContract {
	if patch == nil {
		return nil
	}
	merged := VolumePayoffContract{}
	if base != nil {
		merged = *base
	}
	if strings.TrimSpace(patch.VolumeQuestion) != "" {
		merged.VolumeQuestion = patch.VolumeQuestion
	}
	if strings.TrimSpace(patch.PowerPromise) != "" {
		merged.PowerPromise = patch.PowerPromise
	}
	if strings.TrimSpace(patch.MainOpponentMisread) != "" {
		merged.MainOpponentMisread = patch.MainOpponentMisread
	}
	if strings.TrimSpace(patch.BigWin) != "" {
		merged.BigWin = patch.BigWin
	}
	if strings.TrimSpace(patch.VisibleReward) != "" {
		merged.VisibleReward = patch.VisibleReward
	}
	if strings.TrimSpace(patch.ReputationShift) != "" {
		merged.ReputationShift = patch.ReputationShift
	}
	if strings.TrimSpace(patch.NextBiggerGame) != "" {
		merged.NextBiggerGame = patch.NextBiggerGame
	}
	if merged.IsZero() {
		return nil
	}
	return &merged
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
	Cultivation  string   `json:"cultivation,omitempty" md:"cultivation" desc:"修炼境界/能力等级，如：炼气三层、S级进化者一阶"`
	SpiritStones int      `json:"spirit_stones,omitempty" md:"spirit_stones" desc:"灵石/资源数量"`
	Allies       []string `json:"allies,omitempty" md:"allies" desc:"当前盟友/同伴"`
	Injuries     []string `json:"injuries,omitempty" md:"injuries" desc:"当前伤势"`
	Location     string   `json:"location,omitempty" md:"location" desc:"章节开始时的位置"`
	KeyItems     []string `json:"key_items,omitempty" md:"key_items" desc:"当前持有的关键物品"`
	Notes        string   `json:"notes,omitempty" md:"notes" desc:"其他值得追踪的状态"`
}

// OutlineEnemy declares an enemy that appears in this chapter.
// Write agent uses this as a reference for combat scenes.
type OutlineEnemy struct {
	Name    string `json:"name" md:"name" desc:"敌人名称，如：虫族工蜂、低阶妖兽"`
	Faction string `json:"faction,omitempty" md:"faction" desc:"所属阵营，如：zerg、ai_mech"`
	Tier    string `json:"tier,omitempty" md:"tier" desc:"该阵营内的等级标识，如：drone、soldier、elite、queen"`
	Count   int    `json:"count" md:"count" desc:"出现数量"`
	Level   int    `json:"level,omitempty" md:"level" desc:"敌人等级"`
	IsBoss  bool   `json:"is_boss,omitempty" md:"is_boss" desc:"是否是boss级敌人"`
	BossID  string `json:"boss_id,omitempty" md:"boss_id" desc:"boss唯一ID，跨章追踪用，如 boss_queen"`
	Status  string `json:"status,omitempty" md:"status" desc:"new(首次出场)/engaged(战斗中)/defeated(击败)/escaped(逃脱)"`
	Context string `json:"context,omitempty" md:"context" desc:"出现场景/上下文，如：矿道伏击、据点围攻"`
}

// ResourceLedgerEntry declares expected resource changes in this chapter.
// Validator checks cross-chapter arithmetic.
type ResourceLedgerEntry struct {
	Item   string `json:"item" md:"item" desc:"资源名称，如：灵石、氚晶体"`
	Start  int    `json:"start" md:"start" desc:"本章开始时的数量"`
	Delta  int    `json:"delta" md:"delta" desc:"变化量，正数=获得，负数=消耗/失去"`
	End    int    `json:"end" md:"end" desc:"本章结束时的数量"`
	Reason string `json:"reason" md:"reason" desc:"变化原因，如：挖矿获得、购买消耗"`
}

// OutlineScene represents a scene within a chapter.
// Write agent uses scene specs for focused writing.
type OutlineScene struct {
	Order      int      `json:"order" desc:"场景序号，从1开始"`
	POV        string   `json:"pov" md:"pov" desc:"视角角色"`
	Goal       string   `json:"goal" md:"goal" desc:"场景目标：这个场景要推进什么"`
	Location   string   `json:"location" md:"location" desc:"场景地点（可以和章级地点不同）"`
	Characters []string `json:"characters" md:"characters" desc:"本场景出现的角色"`
	Words      int      `json:"words,omitempty" md:"words" desc:"建议字数"`
	Tone       string   `json:"tone,omitempty" md:"tone" desc:"情绪基调：紧张、轻松、悲伤、燃"`
	Beats      []string `json:"beats,omitempty" md:"beats" desc:"本场景的1-2个plot beats"`
}

// MysteryPlanted declares a new clue or mystery introduced in this chapter.
type MysteryPlanted struct {
	ID      string `json:"id" md:"id" desc:"谜题唯一ID，如 myst_why_woken"`
	Clue    string `json:"clue" md:"clue" desc:"本章揭示的线索，如：星核日志显示三天前有人远程激活了休眠舱"`
	Horizon string `json:"horizon,omitempty" md:"horizon" desc:"可选：this_volume/next_volume/book/series，用于区分本卷回收还是长线伏笔"`
	Status  string `json:"status,omitempty" md:"status" desc:"可选：open/deferred，长线伏笔可标 deferred"`
}

// MysteryResolved declares a previously planted mystery resolved in this chapter.
type MysteryResolved struct {
	ID         string `json:"id" md:"id" desc:"被解决的谜题ID"`
	Resolution string `json:"resolution" md:"resolution" desc:"解答，如：解密主控记录确认激活指令来自联邦秘密计划"`
}

// ChapterMysteries tracks planted and resolved mysteries.
type ChapterMysteries struct {
	Planted  []MysteryPlanted  `json:"planted,omitempty" md:"planted" desc:"本章新埋下的线索/谜题"`
	Resolved []MysteryResolved `json:"resolved,omitempty" md:"resolved" desc:"本章回收的伏笔"`
}

// StorylineAdvance is an optional, lightweight note about how a chapter moves a setup storyline.
// It is intentionally soft: use it for meaningful pressure, choice, reveal, or consequence,
// not for every minor bit of plot bookkeeping.
type StorylineAdvance struct {
	StorylineName string `json:"storyline_name" md:"storyline_name" desc:"对应 setup.storylines.name，可用自然名称"`
	Stage         string `json:"stage,omitempty" md:"stage" desc:"可选：hook, pressure, reveal, reversal, payoff 等自然阶段"`
	Change        string `json:"change" md:"change" desc:"本章让这条线发生了什么实质变化"`
	Consequence   string `json:"consequence,omitempty" md:"consequence" desc:"这个推进造成的新局面、代价或选择"`
	Pressure      string `json:"pressure,omitempty" md:"pressure" desc:"可选：本章给这条线增加的压力或风险"`
}

// ChapterPayoff defines the concrete satisfying beat a chapter should deliver or set up.
type ChapterPayoff struct {
	Desire       string `json:"desire,omitempty" md:"desire" desc:"本章主角想要得到、证明、避开或推进什么"`
	Pressure     string `json:"pressure,omitempty" md:"pressure" desc:"谁或什么卡住主角，形成即时阻力"`
	CleverMove   string `json:"clever_move,omitempty" md:"clever_move" desc:"主角利用本书独特设定、信息差或对手误判做出的破局动作"`
	PayoffMoment string `json:"payoff_moment,omitempty" md:"payoff_moment" desc:"本章要写出来的具体爽点画面"`
	Reward       string `json:"reward,omitempty" md:"reward" desc:"赢后的实际收益、状态变化或推进"`
	SocialProof  string `json:"social_proof,omitempty" md:"social_proof" desc:"旁人震惊、敌人错愕、地位变化或关系确认"`
	Hook         string `json:"hook,omitempty" md:"hook" desc:"本章结尾露出的下一层更大局或新问题"`
}

func (p *ChapterPayoff) IsZero() bool {
	if p == nil {
		return true
	}
	return strings.TrimSpace(p.Desire) == "" &&
		strings.TrimSpace(p.Pressure) == "" &&
		strings.TrimSpace(p.CleverMove) == "" &&
		strings.TrimSpace(p.PayoffMoment) == "" &&
		strings.TrimSpace(p.Reward) == "" &&
		strings.TrimSpace(p.SocialProof) == "" &&
		strings.TrimSpace(p.Hook) == ""
}

// Chapter represents a single chapter in the story
type Chapter struct {
	ID                string                `json:"id" md:"-"` // ID not shown in markdown
	Title             string                `json:"title" md:"title"`
	Summary           string                `json:"summary" md:"heading"`       // 格式: 角色 在 什么地方 发生了 什么事
	Characters        []string              `json:"characters" md:"characters"` // 本章出现的角色名列表
	Location          string                `json:"location" md:"location"`     // 事情发生的地点
	Events            []Event               `json:"events" md:"events"`         // 本章发生的事件
	LegacyBeats       []string              `json:"beats,omitempty" md:"-"`     // 旧版兼容：迁移为 Scenes[].Beats
	OpeningBeat       string                `json:"opening_beat,omitempty" md:"-"`
	ClosingBeat       string                `json:"closing_beat,omitempty" md:"-"`
	StateChange       string                `json:"state_change,omitempty" desc:"Primary change this chapter causes; must map to Events"`
	Conflict          string                `json:"conflict" md:"conflict"`
	Pacing            string                `json:"pacing" md:"pacing"`
	Timeline          ChapterTimeline       `json:"timeline,omitempty" md:"timeline" desc:"章节时间线信息"`
	StateAnchor       StateAnchor           `json:"state_anchor,omitempty" md:"state_anchor" desc:"章节开始时的主角状态锚点"`
	Enemies           []OutlineEnemy        `json:"enemies,omitempty" md:"enemies" desc:"本章出现的敌人清单"`
	ResourceLedger    []ResourceLedgerEntry `json:"resource_ledger,omitempty" md:"resource_ledger" desc:"本章资源变化账本"`
	Scenes            []OutlineScene        `json:"scenes,omitempty" md:"scenes" desc:"章节内场景拆分"`
	Mysteries         ChapterMysteries      `json:"mysteries,omitempty" md:"mysteries" desc:"本章埋设和回收的谜题/伏笔"`
	StorylineAdvances []StorylineAdvance    `json:"storyline_advances,omitempty" md:"storyline_advances" desc:"可选：本章推进了哪些故事线，以及产生了什么后果"`
	ChapterPayoff     *ChapterPayoff        `json:"chapter_payoff,omitempty" md:"chapter_payoff" desc:"可选，本章爽点兑现设计"`
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
	NormalizeOutline(o)
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

	if !v.PayoffContract.IsZero() {
		sb.WriteString("**Payoff Contract:**\n")
		writeMarkdownLine(&sb, "Volume Question", v.PayoffContract.VolumeQuestion)
		writeMarkdownLine(&sb, "Power Promise", v.PayoffContract.PowerPromise)
		writeMarkdownLine(&sb, "Main Opponent Misread", v.PayoffContract.MainOpponentMisread)
		writeMarkdownLine(&sb, "Big Win", v.PayoffContract.BigWin)
		writeMarkdownLine(&sb, "Visible Reward", v.PayoffContract.VisibleReward)
		writeMarkdownLine(&sb, "Reputation Shift", v.PayoffContract.ReputationShift)
		writeMarkdownLine(&sb, "Next Bigger Game", v.PayoffContract.NextBiggerGame)
		sb.WriteString("\n")
	}

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

	// Beats (collected from scenes)
	beats := c.GetBeats()
	if len(beats) > 0 {
		sb.WriteString("**Beats:**\n")
		for i, beat := range beats {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, beat))
		}
		sb.WriteString("\n")
	}

	// Continuity anchors
	if len(beats) > 0 {
		sb.WriteString(fmt.Sprintf("**Opening Beat:** %s\n\n", beats[0]))
	}
	if len(beats) > 1 {
		sb.WriteString(fmt.Sprintf("**Closing Beat:** %s\n\n", beats[len(beats)-1]))
	}
	if c.StateChange != "" {
		sb.WriteString(fmt.Sprintf("**State Change:** %s\n\n", c.StateChange))
	}

	if !c.ChapterPayoff.IsZero() {
		sb.WriteString("**Chapter Payoff:**\n")
		writeMarkdownLine(&sb, "Desire", c.ChapterPayoff.Desire)
		writeMarkdownLine(&sb, "Pressure", c.ChapterPayoff.Pressure)
		writeMarkdownLine(&sb, "Clever Move", c.ChapterPayoff.CleverMove)
		writeMarkdownLine(&sb, "Payoff Moment", c.ChapterPayoff.PayoffMoment)
		writeMarkdownLine(&sb, "Reward", c.ChapterPayoff.Reward)
		writeMarkdownLine(&sb, "Social Proof", c.ChapterPayoff.SocialProof)
		writeMarkdownLine(&sb, "Hook", c.ChapterPayoff.Hook)
		sb.WriteString("\n")
	}

	if len(c.StorylineAdvances) > 0 {
		sb.WriteString("**Storyline Advances:**\n")
		for _, advance := range c.StorylineAdvances {
			sb.WriteString(fmt.Sprintf("- **%s**", advance.StorylineName))
			if advance.Stage != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", advance.Stage))
			}
			if advance.Change != "" {
				sb.WriteString(fmt.Sprintf(": %s", advance.Change))
			}
			if advance.Consequence != "" {
				sb.WriteString(fmt.Sprintf(" | consequence: %s", advance.Consequence))
			}
			if advance.Pressure != "" {
				sb.WriteString(fmt.Sprintf(" | pressure: %s", advance.Pressure))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
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

func writeMarkdownLine(sb *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("- **%s:** %s\n", label, value))
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

// GetBeats collects beats from all scenes. If scenes have beats, returns those.
// Falls back to legacy chapter-level beats for old outline files.
func (c *Chapter) GetBeats() []string {
	var beats []string
	for _, sc := range c.Scenes {
		beats = append(beats, sc.Beats...)
	}
	if len(beats) == 0 {
		beats = append(beats, c.LegacyBeats...)
	}
	return beats
}
