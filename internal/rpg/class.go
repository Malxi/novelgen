package rpg

import (
	"encoding/json"
)

// 职业类型
type ClassType string

const (
	ClassTypeWarrior    ClassType = "warrior"    // 战士
	ClassTypeMage       ClassType = "mage"       // 法师
	ClassTypeRogue      ClassType = "rogue"      // 盗贼/刺客
	ClassTypeCleric     ClassType = "cleric"     // 牧师/治疗
	ClassTypeRanger     ClassType = "ranger"     // 游侠/弓箭手
	ClassTypePaladin    ClassType = "paladin"    // 圣骑士
	ClassTypeWarlock    ClassType = "warlock"    // 术士
	ClassTypeMonk       ClassType = "monk"       // 武僧
	ClassTypeBard       ClassType = "bard"       // 吟游诗人
	ClassTypeDruid      ClassType = "druid"      // 德鲁伊
	ClassTypeSpecial    ClassType = "special"    // 特殊职业
)

// 职业成长倾向
type GrowthType string

const (
	GrowthTypeBalanced  GrowthType = "balanced"   // 平衡
	GrowthTypePhysical  GrowthType = "physical"   // 物理倾向
	GrowthTypeMagical   GrowthType = "magical"    // 魔法倾向
	GrowthTypeDefensive GrowthType = "defensive"  // 防御倾向
	GrowthTypeSpeed     GrowthType = "speed"      // 速度倾向
)

// 职业属性修正
type ClassStatModifier struct {
	HP         float64 `json:"hp_mod"`         // HP修正系数
	MP         float64 `json:"mp_mod"`         // MP修正系数
	Attack     float64 `json:"attack_mod"`     // 攻击修正
	Defense    float64 `json:"defense_mod"`    // 防御修正
	Magic      float64 `json:"magic_mod"`      // 魔法修正
	Resistance float64 `json:"resistance_mod"` // 魔防修正
	Speed      float64 `json:"speed_mod"`      // 速度修正
	Luck       float64 `json:"luck_mod"`       // 幸运修正
}

// 职业技能树节点
type SkillTreeNode struct {
	SkillID      string   `json:"skill_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	LevelRequired int     `json:"level_required"`
	SkillPoints  int      `json:"skill_points"`      // 需要技能点
	ParentSkills []string `json:"parent_skills,omitempty"` // 前置技能
	MaxRank      int      `json:"max_rank"`          // 最大等级
}

// 职业进阶路线
type ClassAdvancement struct {
	ClassID          string   `json:"class_id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	LevelRequired    int      `json:"level_required"`
	QuestRequired    string   `json:"quest_required,omitempty"` // 需要完成的任务
	ItemRequired     string   `json:"item_required,omitempty"`  // 需要的物品
	BonusStats       BaseStats `json:"bonus_stats,omitempty"`   // 进阶奖励属性
}

// 职业定义
type Class struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        ClassType      `json:"type"`
	
	Icon        string         `json:"icon,omitempty"`
	
	// 基础属性倾向
	BaseStats   BaseStats      `json:"base_stats"`
	GrowthType  GrowthType     `json:"growth_type"`
	
	// 属性成长修正
	StatModifier ClassStatModifier `json:"stat_modifier"`
	
	// 成长率（每级属性成长）
	GrowthStats GrowthStats    `json:"growth_stats"`
	
	// 技能树
	SkillTree   []SkillTreeNode `json:"skill_tree"`
	
	// 默认技能
	DefaultSkills []string     `json:"default_skills"`
	
	// 可装备武器类型
	WeaponTypes []string       `json:"weapon_types"`
	
	// 可装备护甲类型
	ArmorTypes  []string       `json:"armor_types"`
	
	// 职业特性/被动
	Traits      []ClassTrait   `json:"traits"`
	
	// 进阶路线
	Advancements []ClassAdvancement `json:"advancements,omitempty"`
	
	// 限制
	RaceAllowed  []string      `json:"race_allowed,omitempty"`   // 允许种族
	GenderAllowed []string     `json:"gender_allowed,omitempty"` // 允许性别
	
	// 起始装备
	StartingEquipment []string `json:"starting_equipment,omitempty"`
	StartingItems     []string `json:"starting_items,omitempty"`
	
	// 特殊资源
	ResourceType string        `json:"resource_type,omitempty"` // 资源类型: mp, rage, energy, etc
	ResourceMax  int           `json:"resource_max,omitempty"`  // 资源最大值
}

// 职业特性
type ClassTrait struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Level       int    `json:"level"`       // 获得等级
	Effect      Effect `json:"effect"`      // 效果
}

// 职业实例（角色当前职业状态）
type ClassInstance struct {
	ClassID       string            `json:"class_id"`
	Level         int               `json:"level"`
	Exp           int               `json:"exp"`
	ExpToNext     int               `json:"exp_to_next"`
	SkillPoints   int               `json:"skill_points"`    // 可用技能点
	SkillRanks    map[string]int    `json:"skill_ranks"`     // 技能等级
	UnlockedSkills []string         `json:"unlocked_skills"` // 已解锁技能
}

// 职业管理器
type ClassManager struct {
	classes map[string]*Class
}

func NewClassManager() *ClassManager {
	return &ClassManager{
		classes: make(map[string]*Class),
	}
}

func (cm *ClassManager) AddClass(class *Class) {
	cm.classes[class.ID] = class
}

func (cm *ClassManager) GetClass(id string) *Class {
	return cm.classes[id]
}

func (cm *ClassManager) GetAllClasses() []*Class {
	result := make([]*Class, 0, len(cm.classes))
	for _, class := range cm.classes {
		result = append(result, class)
	}
	return result
}

func (cm *ClassManager) GetClassesByType(classType ClassType) []*Class {
	result := make([]*Class, 0)
	for _, class := range cm.classes {
		if class.Type == classType {
			result = append(result, class)
		}
	}
	return result
}

// 创建职业实例
func (cm *ClassManager) CreateClassInstance(classID string) *ClassInstance {
	class := cm.classes[classID]
	if class == nil {
		return nil
	}
	
	return &ClassInstance{
		ClassID:        classID,
		Level:          1,
		Exp:            0,
		ExpToNext:      100,
		SkillPoints:    0,
		SkillRanks:     make(map[string]int),
		UnlockedSkills: class.DefaultSkills,
	}
}

// 计算职业加成后的属性
func (cm *ClassManager) ApplyClassStats(classID string, baseStats BaseStats, level int) BaseStats {
	class := cm.classes[classID]
	if class == nil {
		return baseStats
	}
	
	stats := baseStats
	
	// 应用基础属性
	stats.HP += class.BaseStats.HP
	stats.MP += class.BaseStats.MP
	stats.Attack += class.BaseStats.Attack
	stats.Defense += class.BaseStats.Defense
	stats.Magic += class.BaseStats.Magic
	stats.Resistance += class.BaseStats.Resistance
	stats.Speed += class.BaseStats.Speed
	stats.Luck += class.BaseStats.Luck
	
	// 应用等级成长
	for i := 1; i < level; i++ {
		stats.HP += int(class.GrowthStats.HP * class.StatModifier.HP)
		stats.MP += int(class.GrowthStats.MP * class.StatModifier.MP)
		stats.Attack += int(class.GrowthStats.Attack * class.StatModifier.Attack)
		stats.Defense += int(class.GrowthStats.Defense * class.StatModifier.Defense)
		stats.Magic += int(class.GrowthStats.Magic * class.StatModifier.Magic)
		stats.Resistance += int(class.GrowthStats.Resistance * class.StatModifier.Resistance)
		stats.Speed += int(class.GrowthStats.Speed * class.StatModifier.Speed)
		stats.Luck += int(class.GrowthStats.Luck * class.StatModifier.Luck)
	}
	
	return stats
}

// 检查是否可以学习技能
func (cm *ClassManager) CanLearnSkill(classID, skillID string, instance *ClassInstance) (bool, string) {
	class := cm.classes[classID]
	if class == nil {
		return false, "职业不存在"
	}
	
	// 查找技能树节点
	var node *SkillTreeNode
	for i := range class.SkillTree {
		if class.SkillTree[i].SkillID == skillID {
			node = &class.SkillTree[i]
			break
		}
	}
	
	if node == nil {
		return false, "该职业无法学习此技能"
	}
	
	// 检查等级
	if instance.Level < node.LevelRequired {
		return false, "等级不足"
	}
	
	// 检查技能点
	currentRank := instance.SkillRanks[skillID]
	if currentRank >= node.MaxRank {
		return false, "技能已满级"
	}
	
	if instance.SkillPoints < node.SkillPoints {
		return false, "技能点不足"
	}
	
	// 检查前置技能
	for _, parentID := range node.ParentSkills {
		if instance.SkillRanks[parentID] == 0 {
			return false, "前置技能未学习"
		}
	}
	
	return true, ""
}

// 学习技能
func (cm *ClassManager) LearnSkill(classID, skillID string, instance *ClassInstance) bool {
	canLearn, _ := cm.CanLearnSkill(classID, skillID, instance)
	if !canLearn {
		return false
	}
	
	class := cm.classes[classID]
	var node *SkillTreeNode
	for i := range class.SkillTree {
		if class.SkillTree[i].SkillID == skillID {
			node = &class.SkillTree[i]
			break
		}
	}
	
	instance.SkillPoints -= node.SkillPoints
	instance.SkillRanks[skillID]++
	
	// 添加到已解锁技能列表
	if instance.SkillRanks[skillID] == 1 {
		instance.UnlockedSkills = append(instance.UnlockedSkills, skillID)
	}
	
	return true
}

// 职业升级
func (cm *ClassManager) LevelUpClass(instance *ClassInstance) bool {
	class := cm.classes[instance.ClassID]
	if class == nil {
		return false
	}
	
	instance.Level++
	instance.Exp -= instance.ExpToNext
	instance.ExpToNext = int(float64(instance.ExpToNext) * 1.3)
	
	// 获得技能点
	instance.SkillPoints += 1
	if instance.Level%5 == 0 {
		instance.SkillPoints += 1 // 每5级额外获得1点
	}
	
	// 解锁新特性
	for _, trait := range class.Traits {
		if trait.Level == instance.Level {
			// 解锁特性
		}
	}
	
	return true
}

// 检查是否可以进阶
func (cm *ClassManager) CanAdvance(instance *ClassInstance, advancementID string) (bool, string) {
	class := cm.classes[instance.ClassID]
	if class == nil {
		return false, "职业不存在"
	}
	
	var advancement *ClassAdvancement
	for i := range class.Advancements {
		if class.Advancements[i].ClassID == advancementID {
			advancement = &class.Advancements[i]
			break
		}
	}
	
	if advancement == nil {
		return false, "进阶路线不存在"
	}
	
	if instance.Level < advancement.LevelRequired {
		return false, "等级不足"
	}
	
	return true, ""
}

// 序列化
func (c *Class) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

func (ci *ClassInstance) ToJSON() string {
	data, _ := json.MarshalIndent(ci, "", "  ")
	return string(data)
}

// ExportToMap 导出为map
func (cm *ClassManager) ExportToMap() map[string]interface{} {
	return map[string]interface{}{
		"classes": cm.classes,
	}
}
