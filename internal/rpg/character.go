package rpg

import (
	"encoding/json"
	"fmt"
	"time"
)

// 角色类型
type CharacterType string

const (
	CharacterTypePlayer   CharacterType = "player"   // 玩家角色
	CharacterTypeNPC      CharacterType = "npc"      // NPC
	CharacterTypeEnemy    CharacterType = "enemy"    // 敌人
	CharacterTypeBoss     CharacterType = "boss"     // Boss
	CharacterTypeCompanion CharacterType = "companion" // 同伴
)

// 角色状态
type CharacterState string

const (
	CharacterStateNormal    CharacterState = "normal"     // 正常
	CharacterStateBattle    CharacterState = "battle"     // 战斗中
	CharacterStateDead      CharacterState = "dead"       // 死亡
	CharacterStatePoisoned  CharacterState = "poisoned"   // 中毒
	CharacterStateStunned   CharacterState = "stunned"    // 眩晕
	CharacterStateSleeping  CharacterState = "sleeping"   // 睡眠
	CharacterStateSilenced  CharacterState = "silenced"   // 沉默
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
	Weapon   string `json:"weapon,omitempty"`   // 武器ID
	Armor    string `json:"armor,omitempty"`    // 护甲ID
	Helmet   string `json:"helmet,omitempty"`   // 头盔ID
	Accessory1 string `json:"accessory1,omitempty"` // 饰品1
	Accessory2 string `json:"accessory2,omitempty"` // 饰品2
}

// 角色定义（模板）
type CharacterTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        CharacterType  `json:"type"`
	Race        string         `json:"race,omitempty"`
	ClassID     string         `json:"class_id,omitempty"` // 职业ID
	
	BaseStats   BaseStats      `json:"base_stats"`
	GrowthStats GrowthStats    `json:"growth_stats"`
	
	Element     ElementType    `json:"element,omitempty"`
	Rarity      Rarity         `json:"rarity"`
	
	DefaultSkills []string     `json:"default_skills,omitempty"` // 默认技能ID列表
	DropItems   []DropItem     `json:"drop_items,omitempty"`     // 掉落物品
	
	AIBehavior  string         `json:"ai_behavior,omitempty"`    // AI行为模式
	DialogueID  string         `json:"dialogue_id,omitempty"`    // 对话ID
}

// 掉落物品
type DropItem struct {
	ItemID   string  `json:"item_id"`
	Chance   float64 `json:"chance"`   // 掉落概率 0-1
	MinCount int     `json:"min_count,omitempty"`
	MaxCount int     `json:"max_count,omitempty"`
}

// 角色实例（运行时）
type Character struct {
	ID          string         `json:"id"`
	TemplateID  string         `json:"template_id,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        CharacterType  `json:"type"`
	Race        string         `json:"race,omitempty"`
	ClassID     string         `json:"class_id,omitempty"`
	
	Level       int            `json:"level"`
	Exp         int            `json:"exp"`
	ExpToNext   int            `json:"exp_to_next"`
	
	BaseStats   BaseStats      `json:"base_stats"`
	CurrentStats BaseStats     `json:"current_stats"` // 当前属性（包含装备加成）
	GrowthStats GrowthStats    `json:"growth_stats"`
	
	Element     ElementType    `json:"element,omitempty"`
	State       CharacterState `json:"state"`
	
	Equipment   EquipmentSlots `json:"equipment"`
	Skills      []string       `json:"skills"`        // 已学习技能ID
	Items       []InventoryItem `json:"items"`        // 背包
	
	Relationships []Relationship `json:"relationships,omitempty"`
	
	Flags       map[string]interface{} `json:"flags,omitempty"` // 自定义标志
	Position    Position       `json:"position,omitempty"`
	
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
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
	Total        int `json:"total"`
	LevelPower   int `json:"level_power"`
	StatsPower   int `json:"stats_power"`
	EquipPower   int `json:"equip_power"`
	SkillPower   int `json:"skill_power"`
}

// 创建新角色
func NewCharacter(template *CharacterTemplate, name string) *Character {
	char := &Character{
		ID:           string(NewID("char")),
		TemplateID:   template.ID,
		Name:         name,
		Description:  template.Description,
		Type:         template.Type,
		Race:         template.Race,
		ClassID:      template.ClassID,
		Level:        1,
		Exp:          0,
		ExpToNext:    100,
		BaseStats:    template.BaseStats,
		CurrentStats: template.BaseStats,
		GrowthStats:  template.GrowthStats,
		Element:      template.Element,
		State:        CharacterStateNormal,
		Skills:       template.DefaultSkills,
		Items:        make([]InventoryItem, 0),
		Relationships: make([]Relationship, 0),
		Flags:        make(map[string]interface{}),
		CreatedAt:    fmt.Sprintf("%d", time.Now().Unix()),
		UpdatedAt:    fmt.Sprintf("%d", time.Now().Unix()),
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
	c.ExpToNext = int(float64(c.ExpToNext) * 1.2) + 50
	
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
func (c *Character) GainExp(amount int) bool {
	c.Exp += amount
	leveledUp := false
	
	for c.Exp >= c.ExpToNext {
		c.LevelUp()
		leveledUp = true
	}
	
	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	return leveledUp
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
	bp.EquipPower = (c.CurrentStats.HP - c.BaseStats.HP)/10 +
		(c.CurrentStats.Attack - c.BaseStats.Attack)*2
	
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

// 学习技能
func (c *Character) LearnSkill(skillID string) bool {
	for _, id := range c.Skills {
		if id == skillID {
			return false // 已学习
		}
	}
	c.Skills = append(c.Skills, skillID)
	c.UpdatedAt = fmt.Sprintf("%d", time.Now().Unix())
	return true
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
