package rpg

import (
	"encoding/json"
	"math"
	"math/rand"
)

// 技能类型
type SkillType string

const (
	SkillTypeActive    SkillType = "active"    // 主动技能
	SkillTypePassive   SkillType = "passive"   // 被动技能
	SkillTypeReaction  SkillType = "reaction"  // 反应技能（反击等）
	SkillTypeUltimate  SkillType = "ultimate"  // 终极技能
)

// 技能目标类型
type SkillTarget string

const (
	SkillTargetSelf      SkillTarget = "self"       // 自身
	SkillTargetSingle    SkillTarget = "single"     // 单体
	SkillTargetRow       SkillTarget = "row"        // 横排
	SkillTargetColumn    SkillTarget = "column"     // 竖排
	SkillTargetAll       SkillTarget = "all"        // 全体
	SkillTargetArea      SkillTarget = "area"       // 范围
	SkillTargetAlly      SkillTarget = "ally"       // 友方单体
	SkillTargetAllAllies SkillTarget = "all_allies" // 全体友方
)

// 伤害类型
type DamageType string

const (
	DamageTypePhysical DamageType = "physical" // 物理
	DamageTypeMagic    DamageType = "magic"    // 魔法
	DamageTypeTrue     DamageType = "true"     // 真实
	DamageTypeHeal     DamageType = "heal"     // 治疗
)

// 技能消耗
type SkillCost struct {
	HP int `json:"hp,omitempty"` // 生命值消耗
	MP int `json:"mp,omitempty"` // 魔法值消耗
	SP int `json:"sp,omitempty"` // 特殊能量消耗
}

// 技能伤害/治疗计算
type SkillDamage struct {
	Type          DamageType `json:"type"`
	Power         int        `json:"power"`          // 基础威力
	ScalingStat   string     `json:"scaling_stat"`   // 基于哪个属性计算: attack, magic, etc
	ScalingFactor float64    `json:"scaling_factor"` // 属性加成系数
	Element       ElementType `json:"element,omitempty"`
	IsFixed       bool       `json:"is_fixed"`       // 是否固定伤害
}

// 技能效果
type SkillEffect struct {
	Type       EffectType  `json:"type"`
	Chance     float64     `json:"chance"`     // 触发概率
	Duration   int         `json:"duration"`   // 持续回合
	Value      interface{} `json:"value"`      // 效果值
}

// 技能升级信息
type SkillLevelInfo struct {
	Level       int        `json:"level"`
	PowerBonus  int        `json:"power_bonus"`  // 威力加成
	CostReduce  int        `json:"cost_reduce"`  // 消耗减少
	EffectBonus float64    `json:"effect_bonus"` // 效果加成
}

// 技能定义
type Skill struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        SkillType      `json:"type"`
	
	Icon        string         `json:"icon,omitempty"`
	Element     ElementType    `json:"element,omitempty"`
	
	LevelRequired int          `json:"level_required"`   // 学习等级
	ClassRequired []string     `json:"class_required,omitempty"` // 职业限制
	SkillRequired []string     `json:"skill_required,omitempty"` // 前置技能
	
	Cost        SkillCost      `json:"cost"`
	Cooldown    int            `json:"cooldown"`         // 冷却回合
	Target      SkillTarget    `json:"target"`
	Range       int            `json:"range,omitempty"`  // 射程
	
	Damage      *SkillDamage   `json:"damage,omitempty"` // 伤害/治疗信息
	Effects     []SkillEffect  `json:"effects,omitempty"` // 附加效果
	
	MaxLevel    int            `json:"max_level"`        // 最高等级
	LevelInfo   []SkillLevelInfo `json:"level_info,omitempty"` // 每级信息
	
	Animation   string         `json:"animation,omitempty"` // 动画ID
	Sound       string         `json:"sound,omitempty"`     // 音效ID
	
	Tags        []string       `json:"tags,omitempty"`
}

// 技能实例（角色已学习的技能）
type SkillInstance struct {
	SkillID   string `json:"skill_id"`
	Level     int    `json:"level"`
	CurrentCD int    `json:"current_cd"` // 当前冷却
}

// 技能管理器
type SkillManager struct {
	skills map[string]*Skill
}

func NewSkillManager() *SkillManager {
	return &SkillManager{
		skills: make(map[string]*Skill),
	}
}

func (sm *SkillManager) AddSkill(skill *Skill) {
	sm.skills[skill.ID] = skill
}

func (sm *SkillManager) GetSkill(id string) *Skill {
	return sm.skills[id]
}

func (sm *SkillManager) GetAllSkills() []*Skill {
	result := make([]*Skill, 0, len(sm.skills))
	for _, skill := range sm.skills {
		result = append(result, skill)
	}
	return result
}

func (sm *SkillManager) GetSkillsByType(skillType SkillType) []*Skill {
	result := make([]*Skill, 0)
	for _, skill := range sm.skills {
		if skill.Type == skillType {
			result = append(result, skill)
		}
	}
	return result
}

func (sm *SkillManager) GetSkillsByElement(element ElementType) []*Skill {
	result := make([]*Skill, 0)
	for _, skill := range sm.skills {
		if skill.Element == element {
			result = append(result, skill)
		}
	}
	return result
}

// 检查角色是否可以使用技能
func (sm *SkillManager) CanUseSkill(skillID string, caster *Character) (bool, string) {
	skill := sm.skills[skillID]
	if skill == nil {
		return false, "技能不存在"
	}
	
	// 检查等级
	if caster.Level < skill.LevelRequired {
		return false, "等级不足"
	}
	
	// 检查职业
	if len(skill.ClassRequired) > 0 {
		hasClass := false
		for _, classID := range skill.ClassRequired {
			if caster.ClassID == classID {
				hasClass = true
				break
			}
		}
		if !hasClass {
			return false, "职业不符"
		}
	}
	
	// 检查消耗
	if caster.CurrentStats.MP < skill.Cost.MP {
		return false, "魔法值不足"
	}
	if caster.CurrentStats.HP <= skill.Cost.HP {
		return false, "生命值不足"
	}
	
	return true, ""
}

// 使用技能
func (sm *SkillManager) UseSkill(skillID string, caster *Character, targets []*Character) *SkillResult {
	skill := sm.skills[skillID]
	if skill == nil {
		return nil
	}
	
	// 扣除消耗
	caster.CurrentStats.MP -= skill.Cost.MP
	caster.CurrentStats.HP -= skill.Cost.HP
	
	result := &SkillResult{
		SkillID:    skillID,
		CasterID:   caster.ID,
		TargetIDs:  make([]string, 0),
		Damage:     make(map[string]int),
		Effects:    make([]Effect, 0),
		IsCritical: false,
		IsMiss:     false,
	}
	
	// 计算命中率
	hitChance := sm.calculateHitChance(caster, targets[0])
	if rand.Float64() > hitChance {
		result.IsMiss = true
		return result
	}
	
	// 计算伤害/治疗
	if skill.Damage != nil {
		for _, target := range targets {
			damage := sm.calculateDamage(skill, caster, target)
			result.TargetIDs = append(result.TargetIDs, target.ID)
			result.Damage[target.ID] = damage
			
			// 应用伤害
			if skill.Damage.Type == DamageTypeHeal {
				target.CurrentStats.HP = min(target.CurrentStats.HP+damage, target.BaseStats.HP)
			} else {
				target.CurrentStats.HP -= damage
				if target.CurrentStats.HP <= 0 {
					target.CurrentStats.HP = 0
					target.State = CharacterStateDead
				}
			}
		}
	}
	
	// 应用附加效果
	for _, effect := range skill.Effects {
		if rand.Float64() <= effect.Chance {
			for _, target := range targets {
				result.Effects = append(result.Effects, Effect{
					Type:     effect.Type,
					TargetID: target.ID,
					Value:    effect.Value,
					Duration: effect.Duration,
				})
			}
		}
	}
	
	return result
}

// 计算命中率
func (sm *SkillManager) calculateHitChance(caster, target *Character) float64 {
	baseChance := 0.95
	levelDiff := caster.Level - target.Level
	
	// 等级差影响
	if levelDiff > 0 {
		baseChance += float64(levelDiff) * 0.02
	} else {
		baseChance += float64(levelDiff) * 0.03
	}
	
	// 速度差影响
	speedDiff := caster.CurrentStats.Speed - target.CurrentStats.Speed
	baseChance += float64(speedDiff) * 0.001
	
	// 幸运影响
	luckBonus := float64(caster.CurrentStats.Luck) * 0.002
	baseChance += luckBonus
	
	return math.Max(0.1, math.Min(1.0, baseChance))
}

// 计算伤害
func (sm *SkillManager) calculateDamage(skill *Skill, caster, target *Character) int {
	if skill.Damage == nil {
		return 0
	}
	
	damage := skill.Damage.Power
	
	// 属性加成
	var statValue int
	switch skill.Damage.ScalingStat {
	case "attack":
		statValue = caster.CurrentStats.Attack
	case "magic":
		statValue = caster.CurrentStats.Magic
	case "speed":
		statValue = caster.CurrentStats.Speed
	default:
		statValue = caster.CurrentStats.Attack
	}
	
	damage += int(float64(statValue) * skill.Damage.ScalingFactor)
	
	// 防御减免
	if skill.Damage.Type == DamageTypePhysical {
		reduction := float64(target.CurrentStats.Defense) / (float64(target.CurrentStats.Defense) + 100)
		damage = int(float64(damage) * (1 - reduction))
	} else if skill.Damage.Type == DamageTypeMagic {
		reduction := float64(target.CurrentStats.Resistance) / (float64(target.CurrentStats.Resistance) + 100)
		damage = int(float64(damage) * (1 - reduction))
	}
	
	// 元素克制（简化版）
	elementBonus := sm.getElementBonus(skill.Element, target.Element)
	damage = int(float64(damage) * elementBonus)
	
	// 随机波动 (90% - 110%)
	variance := 0.9 + rand.Float64()*0.2
	damage = int(float64(damage) * variance)
	
	// 暴击判定
	critChance := 0.05 + float64(caster.CurrentStats.Luck)*0.002
	if rand.Float64() <= critChance {
		damage = int(float64(damage) * 1.5)
	}
	
	return max(1, damage)
}

// 元素克制关系
func (sm *SkillManager) getElementBonus(skillElement, targetElement ElementType) float64 {
	advantages := map[ElementType][]ElementType{
		ElementFire:    {ElementIce, ElementWind},
		ElementWater:   {ElementFire, ElementEarth},
		ElementWind:    {ElementEarth, ElementThunder},
		ElementEarth:   {ElementWater, ElementThunder},
		ElementThunder: {ElementWater, ElementWind},
		ElementIce:     {ElementWind, ElementEarth},
		ElementLight:   {ElementDark},
		ElementDark:    {ElementLight},
	}
	
	if skillElement == ElementNone || targetElement == ElementNone {
		return 1.0
	}
	
	if advs, ok := advantages[skillElement]; ok {
		for _, adv := range advs {
			if adv == targetElement {
				return 1.5 // 克制
			}
		}
	}
	
	// 被克制
	if advs, ok := advantages[targetElement]; ok {
		for _, adv := range advs {
			if adv == skillElement {
				return 0.75 // 被克制
			}
		}
	}
	
	return 1.0
}

// 技能结果
type SkillResult struct {
	SkillID    string         `json:"skill_id"`
	CasterID   string         `json:"caster_id"`
	TargetIDs  []string       `json:"target_ids"`
	Damage     map[string]int `json:"damage"`
	Effects    []Effect       `json:"effects"`
	IsCritical bool           `json:"is_critical"`
	IsMiss     bool           `json:"is_miss"`
}

// 序列化
func (s *Skill) ToJSON() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
