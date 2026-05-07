package models

import (
	"encoding/json"
	"os"
	"strings"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string               `json:"project_name" prompt:"小说名称" desc:"2-6个字，有吸引力，不超过60个字符"`
	Genres         []string             `json:"genres" prompt:"类型" desc:"2-4个具体小说类型"`
	Premise        string               `json:"premise" prompt:"核心设定" desc:"核心设定，2-4句话，不要列表"`
	Theme          string               `json:"theme" prompt:"主题" desc:"主题，清晰的陈述，不要单个词"`
	Rules          []string             `json:"rules" prompt:"规则" desc:"小说设定的规则"`
	TargetAudience string               `json:"target_audience" prompt:"目标读者" desc:"包含年龄段和读者类型"`
	Tone           string               `json:"tone" prompt:"基调" desc:"小说基调，一句话，2-4个形容词，逗号分隔"`
	Tense          string               `json:"tense" prompt:"时态" desc:"过去时或现在时"`
	POVStyle       string               `json:"pov_style" prompt:"视角" desc:"第一人称、第三人称有限视角或第三人称全知视角"`
	WritingStyle   WritingStyle         `json:"writing_style,omitempty" prompt:"写作风格" desc:"可选，写作风格要求与参考片段；write 阶段只作为风格参照，不作为剧情事实"`
	Storylines     []Storyline          `json:"storylines,omitempty" prompt:"故事线" desc:"故事线"`
	Premises       []Premise            `json:"premises,omitempty" prompt:"设定体系" desc:"设定升级体系（含阵营定义和主角能力体系）"`
	WorldTimeline  []WorldTimelineEntry `json:"world_timeline,omitempty" prompt:"世界时间线" desc:"关键历史事件时间线"`
	WorldResources []WorldResource      `json:"world_resources,omitempty" prompt:"核心资源" desc:"世界中的核心资源定义"`
}

// WritingStyle captures optional prose-level style instructions for write agents.
type WritingStyle struct {
	Name             string   `json:"name,omitempty" prompt:"写作风格名称" desc:"可选，风格名称或参照对象，例如：冷峻克制、轻喜剧网文、参考文的节奏"`
	Description      string   `json:"description,omitempty" prompt:"写作风格描述" desc:"可选，2-6句话，说明叙述声音、句式节奏、描写密度、对白风格"`
	Principles       []string `json:"principles,omitempty" prompt:"写作原则" desc:"可选，3-8条可执行写作原则"`
	Avoid            []string `json:"avoid,omitempty" prompt:"避免事项" desc:"可选，风格禁忌或不要模仿的内容"`
	ReferenceExcerpt string   `json:"reference_excerpt,omitempty" prompt:"参考文章片段" desc:"可选，作为文风参照的短文片段；只模仿风格，不继承情节、设定或人物"`
}

// IsZero reports whether the style carries any user-visible instruction.
func (s WritingStyle) IsZero() bool {
	if strings.TrimSpace(s.Name) != "" ||
		strings.TrimSpace(s.Description) != "" ||
		strings.TrimSpace(s.ReferenceExcerpt) != "" {
		return false
	}
	return len(nonEmptyStrings(s.Principles)) == 0 && len(nonEmptyStrings(s.Avoid)) == 0
}

// CompactReference returns a copy with ReferenceExcerpt trimmed to maxRunes.
func (s WritingStyle) CompactReference(maxRunes int) WritingStyle {
	s.Name = strings.TrimSpace(s.Name)
	s.Description = strings.TrimSpace(s.Description)
	s.Principles = nonEmptyStrings(s.Principles)
	s.Avoid = nonEmptyStrings(s.Avoid)
	if maxRunes <= 0 {
		s.ReferenceExcerpt = ""
		return s
	}
	runes := []rune(strings.TrimSpace(s.ReferenceExcerpt))
	if len(runes) > maxRunes {
		s.ReferenceExcerpt = string(runes[:maxRunes]) + "..."
	} else {
		s.ReferenceExcerpt = string(runes)
	}
	return s
}

func nonEmptyStrings(items []string) []string {
	var result []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name           string        `json:"name" prompt:"Name" desc:"故事线名称"`
	Description    string        `json:"description" prompt:"Description" desc:"故事线描述，2-4句话"`
	Type           string        `json:"type" prompt:"Type" desc:"故事线类型"`                   // main, subplot, character_arc, etc.
	Importance     int           `json:"importance" prompt:"Importance" desc:"故事线重要性，1-10"` // 1-10, 10 being most important
	Scope          string        `json:"scope,omitempty" prompt:"Scope" desc:"可选，高层作用域：opening、volume、book、series；不要写具体章节"`
	PayoffStyle    string        `json:"payoff_style,omitempty" prompt:"Payoff Style" desc:"可选，兑现方式：immediate、staged_reveal、slow_burn、final_turn 等高层方式"`
	SetupRole      string        `json:"setup_role,omitempty" prompt:"Setup Role" desc:"可选，这条线在故事机器中的高层作用：生存钩子、长期悬念、势力压力、成长驱动等"`
	Desire         string        `json:"desire,omitempty" prompt:"Desire" desc:"这条故事线中角色或势力最想得到什么"`
	Opposition     string        `json:"opposition,omitempty" prompt:"Opposition" desc:"阻止这条故事线推进的人、规则、困境或代价"`
	Stakes         string        `json:"stakes,omitempty" prompt:"Stakes" desc:"如果失败会失去什么，或成功会改变什么"`
	Turn           string        `json:"turn,omitempty" prompt:"Turn" desc:"这条线最有戏剧张力的反转、误判或关系变化"`
	Payoff         string        `json:"payoff,omitempty" prompt:"Payoff" desc:"这条线承诺给读者的情绪或信息回收"`
	OpenQuestion   string        `json:"open_question,omitempty" prompt:"Open Question" desc:"驱动读者继续看的未解问题"`
	PressurePoints []string      `json:"pressure_points,omitempty" prompt:"Pressure Points" desc:"可选的2-4个推进压力点，不必写成固定阶段"`
	AppealEngine   *AppealEngine `json:"appeal_engine,omitempty" prompt:"Appeal Engine" desc:"可选，爽点引擎：能力爽点、表面限制、破解方式、赢法展示、升级钩子、敌人误判、收益类型"`
}

// AppealEngine describes how a setting or arc creates satisfying power-fantasy payoffs.
// It frames limits as exploitable surfaces rather than grim mandatory costs.
type AppealEngine struct {
	Appeal          string `json:"appeal,omitempty" prompt:"Appeal" desc:"核心能力爽点或读者期待"`
	SurfaceLimit    string `json:"surface_limit,omitempty" prompt:"Surface Limit" desc:"表面限制、冷却、盲区、条件或规则边界，避免能力无限膨胀"`
	Exploit         string `json:"exploit,omitempty" prompt:"Exploit" desc:"主角如何利用规则、时机、信息差或敌人假设漂亮破局"`
	SignatureWin    string `json:"signature_win,omitempty" prompt:"Signature Win" desc:"这个设定能制造的具体赢法画面"`
	UpgradePath     string `json:"upgrade_path,omitempty" prompt:"Upgrade Path" desc:"后续如何升级爽点且不破坏规则"`
	OpponentMisread string `json:"opponent_misread,omitempty" prompt:"Opponent Misread" desc:"敌人通常会误判这个设定的地方"`
	RewardType      string `json:"reward_type,omitempty" prompt:"Reward Type" desc:"赢后收益类型：资源、地位、秘密、盟友、地盘、名声、自由等"`
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name         string             `json:"name" prompt:"Name" desc:"设定体系名称"`
	Description  string             `json:"description" prompt:"Description" desc:"设定体系描述，2-4句话"`
	Category     string             `json:"category" prompt:"Category" desc:"设定体系类型"`         // 机甲, 基因, 飞船, 魔法, etc.
	Progression  []ProgressionStage `json:"progression" prompt:"Progression" desc:"设定体系升级体系"` // 升级体系
	AppealEngine *AppealEngine      `json:"appeal_engine,omitempty" prompt:"Appeal Engine" desc:"可选，该设定体系的爽点引擎"`
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level" prompt:"Level" desc:"设定体系升级体系等级"`
	Name         string `json:"name" prompt:"Name" desc:"设定体系升级体系名称"`
	Description  string `json:"description" prompt:"Description" desc:"设定体系升级体系描述"`
	Requirements string `json:"requirements,omitempty" prompt:"Requirements" desc:"设定体系升级体系要求"`
}

// WorldTimelineEntry represents a key historical event.
type WorldTimelineEntry struct {
	Year           string `json:"year" prompt:"时间" desc:"时间标识，如：公元2247年、星元79年"`
	Event          string `json:"event" prompt:"事件" desc:"事件简述"`
	Impact         string `json:"impact,omitempty" prompt:"影响" desc:"对当前世界的影响"`
	RelatedMystery string `json:"related_mystery,omitempty" prompt:"关联伏笔" desc:"关联的谜题ID，如 myst_timeline_gap"`
}

// WorldResource defines a core resource in the world.
type WorldResource struct {
	Name        string `json:"name" prompt:"名称" desc:"资源名称，如：氚晶体、基因进化药剂"`
	Category    string `json:"category" prompt:"类型" desc:"能源/消耗品/材料/货币"`
	Scarcity    string `json:"scarcity" prompt:"稀有度" desc:"常见/稀有/独一无二"`
	Description string `json:"description" prompt:"描述" desc:"功能与来源简述"`
}

// Save writes the story setup to a file
func (s *StorySetup) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStorySetup reads the story setup from a file
func LoadStorySetup(path string) (*StorySetup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var setup StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil, err
	}
	return &setup, nil
}
