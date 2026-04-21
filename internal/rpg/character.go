package rpg

import (
	"encoding/json"
	"fmt"
	"time"
)

// 角色类型
type CharacterType string

const (
	CharacterTypePlayer    CharacterType = "player"    // 玩家角色
	CharacterTypeNPC       CharacterType = "npc"       // NPC
	CharacterTypeEnemy     CharacterType = "enemy"     // 敌人
	CharacterTypeBoss      CharacterType = "boss"      // Boss
	CharacterTypeCompanion CharacterType = "companion" // 同伴
)

// 角色状态
type CharacterState string

const (
	CharacterStateNormal   CharacterState = "normal"   // 正常
	CharacterStateBattle   CharacterState = "battle"   // 战斗中
	CharacterStateDead     CharacterState = "dead"     // 死亡
	CharacterStatePoisoned CharacterState = "poisoned" // 中毒
	CharacterStateStunned  CharacterState = "stunned"  // 眩晕
	CharacterStateSleeping CharacterState = "sleeping" // 睡眠
	CharacterStateSilenced CharacterState = "silenced" // 沉默
)

// 角色关系
type Relationship struct {
	CharacterID string `json:"character_id"`
	Name        string `json:"name"`
	Value       int    `json:"value"` // -100 到 100，负数敌对，正数友好
	Description string `json:"description,omitempty"`
}

// 角色装备槽
type EquipmentSlots struct {
	Weapon     string `json:"weapon,omitempty"`     // 武器ID
	Armor      string `json:"armor,omitempty"`      // 护甲ID
	Helmet     string `json:"helmet,omitempty"`     // 头盔ID
	Accessory1 string `json:"accessory1,omitempty"` // 饰品1
	Accessory2 string `json:"accessory2,omitempty"` // 饰品2
}

// 角色定义（模板）
type CharacterTemplate struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        CharacterType `json:"type"`
	Race        string        `json:"race,omitempty"`
	ClassID     string        `json:"class_id,omitempty"` // 职业ID

	BaseStats   BaseStats   `json:"base_stats"`
	GrowthStats GrowthStats `json:"growth_stats"`

	Element ElementType `json:"element,omitempty"`
	Rarity  Rarity      `json:"rarity"`

	DefaultSkills []string   `json:"default_skills,omitempty"` // 默认技能ID列表
	DropItems     []DropItem `json:"drop_items,omitempty"`     // 掉落物品

	AIBehavior string `json:"ai_behavior,omitempty"` // AI行为模式
	DialogueID string `json:"dialogue_id,omitempty"` // 对话ID
}

// 掉落物品
type DropItem struct {
	ItemID   string  `json:"item_id"`
	Chance   float64 `json:"chance"` // 掉落概率 0-1
	MinCount int     `json:"min_count,omitempty"`
	MaxCount int     `json:"max_count,omitempty"`
}

// 突破阶段定义
type BreakthroughStage struct {
	Name           string   `json:"name"`                      // 阶段名称（如 F级、筑基期、一阶）
	Order          int      `json:"order"`                     // 阶段顺序（数字越大越强）
	StatMultiplier float64  `json:"stat_multiplier"`           // 属性倍率（相对于基础属性）
	Description    string   `json:"description,omitempty"`     // 阶段描述
	UnlockedSkills []string `json:"unlocked_skills,omitempty"` // 解锁的技能
}

// 突破体系定义
type BreakthroughSystem struct {
	Name   string              `json:"name"`   // 体系名称（如 基因进化、修仙、超能力）
	Stages []BreakthroughStage `json:"stages"` // 阶段列表（按 order 排序）
}

// 预定义的突破体系
var DefaultBreakthroughSystems = map[string]*BreakthroughSystem{
	"gene_evolution": {
		Name: "基因进化",
		Stages: []BreakthroughStage{
			{Name: "凡人", Order: 0, StatMultiplier: 1.0, Description: "未进化的普通人类"},
			{Name: "F级", Order: 1, StatMultiplier: 3.0, Description: "初代基因进化者，身体素质提升3倍"},
			{Name: "E级", Order: 2, StatMultiplier: 5.0, Description: "中级基因进化者，获得特殊天赋"},
			{Name: "D级", Order: 3, StatMultiplier: 8.0, Description: "高级基因进化者，可感知能量波动"},
			{Name: "C级", Order: 4, StatMultiplier: 12.0, Description: "精英基因进化者，可操控元素"},
			{Name: "B级", Order: 5, StatMultiplier: 18.0, Description: "大师级基因进化者，可短暂飞行"},
			{Name: "A级", Order: 6, StatMultiplier: 25.0, Description: "宗师级基因进化者，可改变局部环境"},
			{Name: "S级", Order: 7, StatMultiplier: 50.0, Description: "终极基因进化者，接近神明"},
		},
	},
	"cultivation": {
		Name: "修仙",
		Stages: []BreakthroughStage{
			{Name: "凡人", Order: 0, StatMultiplier: 1.0, Description: "未修炼的普通人"},
			{Name: "炼气期", Order: 1, StatMultiplier: 2.0, Description: "引气入体，初窥门径"},
			{Name: "筑基期", Order: 2, StatMultiplier: 4.0, Description: "筑就道基，脱胎换骨"},
			{Name: "金丹期", Order: 3, StatMultiplier: 8.0, Description: "凝结金丹，寿元大增"},
			{Name: "元婴期", Order: 4, StatMultiplier: 15.0, Description: "孕育元婴，可夺舍重生"},
			{Name: "化神期", Order: 5, StatMultiplier: 25.0, Description: "化神入境，可撕裂虚空"},
			{Name: "炼虚期", Order: 6, StatMultiplier: 40.0, Description: "炼虚合道，可操控法则"},
			{Name: "大乘期", Order: 7, StatMultiplier: 60.0, Description: "大乘圆满，可飞升仙界"},
			{Name: "渡劫期", Order: 8, StatMultiplier: 100.0, Description: "渡劫成仙，超脱凡俗"},
		},
	},
	"superpower": {
		Name: "超能力",
		Stages: []BreakthroughStage{
			{Name: "普通人", Order: 0, StatMultiplier: 1.0, Description: "未觉醒的普通人"},
			{Name: "E级", Order: 1, StatMultiplier: 2.5, Description: "初步觉醒，能力微弱"},
			{Name: "D级", Order: 2, StatMultiplier: 5.0, Description: "能力稳定，可日常使用"},
			{Name: "C级", Order: 3, StatMultiplier: 10.0, Description: "能力强化，可战斗应用"},
			{Name: "B级", Order: 4, StatMultiplier: 18.0, Description: "能力精通，可改变战局"},
			{Name: "A级", Order: 5, StatMultiplier: 30.0, Description: "能力大师，可一人敌一国"},
			{Name: "S级", Order: 6, StatMultiplier: 50.0, Description: "能力巅峰，接近神明"},
			{Name: "SSS级", Order: 7, StatMultiplier: 100.0, Description: "超越极限，可改写现实"},
		},
	},
	"martial_arts": {
		Name: "武道",
		Stages: []BreakthroughStage{
			{Name: "普通人", Order: 0, StatMultiplier: 1.0, Description: "未习武的普通人"},
			{Name: "淬体境", Order: 1, StatMultiplier: 2.0, Description: "淬炼肉身，力达百斤"},
			{Name: "开脉境", Order: 2, StatMultiplier: 4.0, Description: "开启经脉，内力流转"},
			{Name: "凝气境", Order: 3, StatMultiplier: 8.0, Description: "凝聚真气，可外放伤人"},
			{Name: "先天境", Order: 4, StatMultiplier: 15.0, Description: "先天真气，可御空而行"},
			{Name: "宗师境", Order: 5, StatMultiplier: 25.0, Description: "一代宗师，可开宗立派"},
			{Name: "大宗师", Order: 6, StatMultiplier: 40.0, Description: "武道巅峰，可一人破军"},
			{Name: "武神境", Order: 7, StatMultiplier: 80.0, Description: "武道之神，可破碎虚空"},
		},
	},
}

// 角色实例（运行时）
type Character struct {
	ID          string        `json:"id"`
	TemplateID  string        `json:"template_id,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Type        CharacterType `json:"type"`
	Race        string        `json:"race,omitempty"`
	ClassID     string        `json:"class_id,omitempty"`

	Level     int `json:"level"`
	Exp       int `json:"exp"`
	ExpToNext int `json:"exp_to_next"`

	// 突破体系
	BreakthroughSystem       string    `json:"breakthrough_system,omitempty"` // 体系名称（如 gene_evolution）
	BreakthroughStage        string    `json:"breakthrough_stage,omitempty"`  // 当前阶段名称（如 F级）
	BreakthroughOrder        int       `json:"breakthrough_order"`            // 当前阶段顺序
	BaseStatsForBreakthrough BaseStats `json:"base_stats_for_breakthrough"`   // 突破前的基础属性（用于计算倍率）

	BaseStats    BaseStats   `json:"base_stats"`
	CurrentStats BaseStats   `json:"current_stats"` // 当前属性（包含装备加成）
	GrowthStats  GrowthStats `json:"growth_stats"`

	Element ElementType    `json:"element,omitempty"`
	State   CharacterState `json:"state"`

	Equipment EquipmentSlots  `json:"equipment"`
	Skills    []string        `json:"skills"` // 已学习技能ID
	Items     []InventoryItem `json:"items"`  // 背包

	Relationships []Relationship `json:"relationships,omitempty"`

	Flags    map[string]interface{} `json:"flags,omitempty"` // 自定义标志
	Position Position               `json:"position,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// 背包物品
type InventoryItem struct {
	ItemID   string `json:"item_id"`
	Count    int    `json:"count"`
	Equipped bool   `json:"equipped,omitempty"`
}

// 位置
type Position struct {
	MapID     string  `json:"map_id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Direction string  `json:"direction,omitempty"` // 朝向
}

// 战力计算
type BattlePower struct {
	Total      int `json:"total"`
	LevelPower int `json:"level_power"`
	StatsPower int `json:"stats_power"`
	EquipPower int `json:"equip_power"`
	SkillPower int `json:"skill_power"`
}

// 创建新角色
func NewCharacter(template *CharacterTemplate, name string) *Character {
	char := &Character{
		ID:            string(NewID("char")),
		TemplateID:    template.ID,
		Name:          name,
		Description:   template.Description,
		Type:          template.Type,
		Race:          template.Race,
		ClassID:       template.ClassID,
		Level:         1,
		Exp:           0,
		ExpToNext:     100,
		BaseStats:     template.BaseStats,
		CurrentStats:  template.BaseStats,
		GrowthStats:   template.GrowthStats,
		Element:       template.Element,
		State:         CharacterStateNormal,
		Skills:        template.DefaultSkills,
		Items:         make([]InventoryItem, 0),
		Relationships: make([]Relationship, 0),
		Flags:         make(map[string]interface{}),
		CreatedAt:     fmt.Sprintf("%d", time.Now().Unix()),
		UpdatedAt:     fmt.Sprintf("%d", time.Now().Unix()),
	}
	return char
}

// 计算当前属性（包含装备加成）
func (c *Character) CalculateCurrentStats(equipmentMgr *EquipmentManager) {
	c.CurrentStats = c.BaseStats

	// 添加装备加成
	equips := []string{c.Equipment.Weapon, c.Equipment.Armor, c.Equipment.Helmet,
		c.Equipment.Accessory1, c.Equipment.Accessory2}

	for _, equipID := range equips {
		if equipID == "" || equipmentMgr == nil {
			continue
		}
		equip := equipmentMgr.GetEquipment(equipID)
		if equip != nil {
			c.CurrentStats.HP += equip.Stats.HP
			c.CurrentStats.MP += equip.Stats.MP
			c.CurrentStats.Attack += equip.Stats.Attack
			c.CurrentStats.Defense += equip.Stats.Defense
			c.CurrentStats.Magic += equip.Stats.Magic
			c.CurrentStats.Resistance += equip.Stats.Resistance
			c.CurrentStats.Speed += equip.Stats.Speed
			c.CurrentStats.Luck += equip.Stats.Luck
		}
	}
}

// 升级
func (c *Character) LevelUp() {
	c.Level++
	c.Exp -= c.ExpToNext
	c.ExpToNext = int(float64(c.ExpToNext)*1.2) + 50

	// 应用成长率
	c.BaseStats.HP += int(c.GrowthStats.HP)
	c.BaseStats.MP += int(c.GrowthStats.MP)
	c.BaseStats.Attack += int(c.GrowthStats.Attack)
	c.BaseStats.Defense += int(c.GrowthStats.Defense)
	c.BaseStats.Magic += int(c.GrowthStats.Magic)
	c.BaseStats.Resistance += int(c.GrowthStats.Resistance)
	c.BaseStats.Speed += int(c.GrowthStats.Speed)
	c.BaseStats.Luck += int(c.GrowthStats.Luck)

	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
}

// 获得经验
func (c *Character) GainExp(amount int) (bool, map[string]int) {
	c.Exp += amount
	leveledUp := false
	statChanges := make(map[string]int)

	for c.Exp >= c.ExpToNext {
		// 记录升级前的属性
		oldHP := c.BaseStats.HP
		oldMP := c.BaseStats.MP
		oldAtk := c.BaseStats.Attack
		oldDef := c.BaseStats.Defense
		oldMag := c.BaseStats.Magic
		oldRes := c.BaseStats.Resistance
		oldSpd := c.BaseStats.Speed
		oldLuck := c.BaseStats.Luck

		c.LevelUp()
		leveledUp = true

		// 记录属性变化
		if leveledUp {
			statChanges["hp"] = c.BaseStats.HP - oldHP
			statChanges["mp"] = c.BaseStats.MP - oldMP
			statChanges["attack"] = c.BaseStats.Attack - oldAtk
			statChanges["defense"] = c.BaseStats.Defense - oldDef
			statChanges["magic"] = c.BaseStats.Magic - oldMag
			statChanges["resistance"] = c.BaseStats.Resistance - oldRes
			statChanges["speed"] = c.BaseStats.Speed - oldSpd
			statChanges["luck"] = c.BaseStats.Luck - oldLuck
		}
	}

	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	return leveledUp, statChanges
}

// 突破到下一阶段
func (c *Character) Breakthrough(systemName, stageName string) (bool, map[string]int, error) {
	// 获取突破体系
	system, ok := DefaultBreakthroughSystems[systemName]
	if !ok {
		return false, nil, fmt.Errorf("突破体系不存在: %s", systemName)
	}

	// 查找目标阶段
	var targetStage *BreakthroughStage
	for i := range system.Stages {
		if system.Stages[i].Name == stageName {
			targetStage = &system.Stages[i]
			break
		}
	}

	if targetStage == nil {
		return false, nil, fmt.Errorf("阶段不存在: %s", stageName)
	}

	// 检查是否是更高阶段
	if targetStage.Order <= c.BreakthroughOrder {
		return false, nil, fmt.Errorf("已经是 %s 或更高阶段", c.BreakthroughStage)
	}

	// 保存突破前的基础属性（如果还没有保存）
	if c.BaseStatsForBreakthrough.HP == 0 {
		c.BaseStatsForBreakthrough = c.BaseStats
	}

	// 记录突破前的属性
	oldStats := c.BaseStats

	// 应用突破倍率（基于原始基础属性）
	multiplier := targetStage.StatMultiplier
	c.BaseStats.HP = int(float64(c.BaseStatsForBreakthrough.HP) * multiplier)
	c.BaseStats.MP = int(float64(c.BaseStatsForBreakthrough.MP) * multiplier)
	c.BaseStats.Attack = int(float64(c.BaseStatsForBreakthrough.Attack) * multiplier)
	c.BaseStats.Defense = int(float64(c.BaseStatsForBreakthrough.Defense) * multiplier)
	c.BaseStats.Magic = int(float64(c.BaseStatsForBreakthrough.Magic) * multiplier)
	c.BaseStats.Resistance = int(float64(c.BaseStatsForBreakthrough.Resistance) * multiplier)
	c.BaseStats.Speed = int(float64(c.BaseStatsForBreakthrough.Speed) * multiplier)
	c.BaseStats.Luck = int(float64(c.BaseStatsForBreakthrough.Luck) * multiplier)

	// 更新突破信息
	c.BreakthroughSystem = systemName
	c.BreakthroughStage = stageName
	c.BreakthroughOrder = targetStage.Order

	// 记录属性变化
	statChanges := map[string]int{
		"hp":         c.BaseStats.HP - oldStats.HP,
		"mp":         c.BaseStats.MP - oldStats.MP,
		"attack":     c.BaseStats.Attack - oldStats.Attack,
		"defense":    c.BaseStats.Defense - oldStats.Defense,
		"magic":      c.BaseStats.Magic - oldStats.Magic,
		"resistance": c.BaseStats.Resistance - oldStats.Resistance,
		"speed":      c.BaseStats.Speed - oldStats.Speed,
		"luck":       c.BaseStats.Luck - oldStats.Luck,
	}

	// 解锁新技能
	if len(targetStage.UnlockedSkills) > 0 {
		for _, skillID := range targetStage.UnlockedSkills {
			c.LearnSkill(skillID)
		}
	}

	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	return true, statChanges, nil
}

// 获取当前突破阶段信息
func (c *Character) GetBreakthroughInfo() (string, string, float64) {
	if c.BreakthroughSystem == "" {
		return "", "未设定", 1.0
	}
	system, ok := DefaultBreakthroughSystems[c.BreakthroughSystem]
	if !ok {
		return c.BreakthroughSystem, c.BreakthroughStage, 1.0
	}

	stageName := c.BreakthroughStage
	multiplier := 1.0
	for _, stage := range system.Stages {
		if stage.Name == stageName {
			multiplier = stage.StatMultiplier
			break
		}
	}

	return system.Name, stageName, multiplier
}

// 学习技能
func (c *Character) LearnSkill(skillID string) bool {
	for _, id := range c.Skills {
		if id == skillID {
			return false // 已经学会
		}
	}
	c.Skills = append(c.Skills, skillID)
	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	return true
}

// 计算战力
func (c *Character) CalculateBattlePower() BattlePower {
	bp := BattlePower{}

	// 等级战力
	bp.LevelPower = c.Level * 100

	// 属性战力
	bp.StatsPower = c.BaseStats.HP/10 + c.BaseStats.MP/5 +
		c.BaseStats.Attack*2 + c.BaseStats.Defense*2 +
		c.BaseStats.Magic*2 + c.BaseStats.Resistance*2 +
		c.BaseStats.Speed + c.BaseStats.Luck

	// 装备战力（简化计算）
	bp.EquipPower = (c.CurrentStats.HP-c.BaseStats.HP)/10 +
		(c.CurrentStats.Attack-c.BaseStats.Attack)*2

	// 技能战力
	bp.SkillPower = len(c.Skills) * 50

	bp.Total = bp.LevelPower + bp.StatsPower + bp.EquipPower + bp.SkillPower

	return bp
}

// 添加物品到背包
func (c *Character) AddItem(itemID string, count int) {
	for i, item := range c.Items {
		if item.ItemID == itemID {
			c.Items[i].Count += count
			return
		}
	}
	c.Items = append(c.Items, InventoryItem{ItemID: itemID, Count: count})
}

// 移除物品
func (c *Character) RemoveItem(itemID string, count int) bool {
	for i, item := range c.Items {
		if item.ItemID == itemID {
			if item.Count >= count {
				c.Items[i].Count -= count
				if c.Items[i].Count == 0 {
					c.Items = append(c.Items[:i], c.Items[i+1:]...)
				}
				return true
			}
			return false
		}
	}
	return false
}

// 设置关系
func (c *Character) SetRelationship(charID, name string, value int, desc string) {
	for i, rel := range c.Relationships {
		if rel.CharacterID == charID {
			c.Relationships[i].Value = value
			c.Relationships[i].Description = desc
			return
		}
	}
	c.Relationships = append(c.Relationships, Relationship{
		CharacterID: charID,
		Name:        name,
		Value:       value,
		Description: desc,
	})
}

// 获取关系值
func (c *Character) GetRelationship(charID string) int {
	for _, rel := range c.Relationships {
		if rel.CharacterID == charID {
			return rel.Value
		}
	}
	return 0
}

// 装备物品
func (c *Character) Equip(itemID string, slot string) bool {
	switch slot {
	case "weapon":
		c.Equipment.Weapon = itemID
	case "armor":
		c.Equipment.Armor = itemID
	case "helmet":
		c.Equipment.Helmet = itemID
	case "accessory1":
		c.Equipment.Accessory1 = itemID
	case "accessory2":
		c.Equipment.Accessory2 = itemID
	default:
		return false
	}
	return true
}

// 序列化
func (c *Character) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

func (ct *CharacterTemplate) ToJSON() string {
	data, _ := json.MarshalIndent(ct, "", "  ")
	return string(data)
}

// 角色管理器
type CharacterManager struct {
	templates map[string]*CharacterTemplate
	instances map[string]*Character
}

func NewCharacterManager() *CharacterManager {
	return &CharacterManager{
		templates: make(map[string]*CharacterTemplate),
		instances: make(map[string]*Character),
	}
}

func (cm *CharacterManager) AddTemplate(template *CharacterTemplate) {
	cm.templates[template.ID] = template
}

func (cm *CharacterManager) GetTemplate(id string) *CharacterTemplate {
	return cm.templates[id]
}

func (cm *CharacterManager) CreateCharacter(templateID, name string) *Character {
	template := cm.templates[templateID]
	if template == nil {
		return nil
	}
	char := NewCharacter(template, name)
	cm.instances[char.ID] = char
	return char
}

func (cm *CharacterManager) GetCharacter(id string) *Character {
	return cm.instances[id]
}

func (cm *CharacterManager) AddCharacterInstance(char *Character) {
	cm.instances[char.ID] = char
}

func (cm *CharacterManager) GetAllCharacters() []*Character {
	result := make([]*Character, 0, len(cm.instances))
	for _, char := range cm.instances {
		result = append(result, char)
	}
	return result
}

// ExportToMap 导出为map
func (cm *CharacterManager) ExportToMap() map[string]interface{} {
	return map[string]interface{}{
		"templates": cm.templates,
		"instances": cm.instances,
	}
}
