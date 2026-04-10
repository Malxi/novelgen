package rpg

import (
	"encoding/json"
)

// 装备类型
type EquipmentType string

const (
	EquipTypeWeapon    EquipmentType = "weapon"    // 武器
	EquipTypeArmor     EquipmentType = "armor"     // 护甲
	EquipTypeHelmet    EquipmentType = "helmet"    // 头盔
	EquipTypeShield    EquipmentType = "shield"    // 盾牌
	EquipTypeAccessory EquipmentType = "accessory" // 饰品
	EquipTypeBoots     EquipmentType = "boots"     // 靴子
	EquipTypeGloves    EquipmentType = "gloves"    // 手套
	EquipTypeCloak     EquipmentType = "cloak"     // 披风
)

// 武器类型
type WeaponType string

const (
	WeaponTypeSword       WeaponType = "sword"       // 剑
	WeaponTypeGreatsword  WeaponType = "greatsword"  // 大剑
	WeaponTypeAxe         WeaponType = "axe"         // 斧
	WeaponTypeMace        WeaponType = "mace"        // 锤
	WeaponTypeSpear       WeaponType = "spear"       // 枪
	WeaponTypeDagger      WeaponType = "dagger"      // 匕首
	WeaponTypeBow         WeaponType = "bow"         // 弓
	WeaponTypeCrossbow    WeaponType = "crossbow"    // 弩
	WeaponTypeStaff       WeaponType = "staff"       // 法杖
	WeaponTypeWand        WeaponType = "wand"        // 魔杖
	WeaponTypeFist        WeaponType = "fist"        // 拳套
	WeaponTypeScythe      WeaponType = "scythe"      // 镰刀
	WeaponTypeWhip        WeaponType = "whip"        // 鞭
	WeaponTypeInstrument  WeaponType = "instrument"  // 乐器
)

// 护甲类型
type ArmorType string

const (
	ArmorTypeCloth   ArmorType = "cloth"   // 布甲
	ArmorTypeLeather ArmorType = "leather" // 皮甲
	ArmorTypeMail    ArmorType = "mail"    // 链甲
	ArmorTypePlate   ArmorType = "plate"   // 板甲
	ArmorTypeRobe    ArmorType = "robe"    // 长袍
)

// 装备属性
type EquipmentStats struct {
	HP         int `json:"hp,omitempty"`
	MP         int `json:"mp,omitempty"`
	Attack     int `json:"attack,omitempty"`
	Defense    int `json:"defense,omitempty"`
	Magic      int `json:"magic,omitempty"`
	Resistance int `json:"resistance,omitempty"`
	Speed      int `json:"speed,omitempty"`
	Luck       int `json:"luck,omitempty"`
	CritRate   int `json:"crit_rate,omitempty"`   // 暴击率加成
	CritDamage int `json:"crit_damage,omitempty"` // 暴击伤害加成
}

// 武器特有属性
type WeaponStats struct {
	MinDamage    int    `json:"min_damage"`     // 最小伤害
	MaxDamage    int    `json:"max_damage"`     // 最大伤害
	AttackSpeed  float64 `json:"attack_speed"`  // 攻击速度
	Range        int    `json:"range"`          // 攻击范围
	IsTwoHanded  bool   `json:"is_two_handed"`  // 是否双手武器
	AmmoType     string `json:"ammo_type,omitempty"` // 弹药类型
}

// 装备定义
type Equipment struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        EquipmentType  `json:"type"`
	
	Icon        string         `json:"icon,omitempty"`
	Model       string         `json:"model,omitempty"` // 3D模型路径
	
	Rarity      Rarity         `json:"rarity"`
	LevelRequired int          `json:"level_required"`
	
	// 职业限制
	ClassAllowed []string      `json:"class_allowed,omitempty"`
	
	// 基础属性
	Stats       EquipmentStats `json:"stats"`
	
	// 特殊效果
	Effects     []EquipmentEffect `json:"effects,omitempty"`
	
	// 套装ID
	SetID       string         `json:"set_id,omitempty"`
	
	// 耐久度
	Durability  int            `json:"durability"`
	MaxDurability int          `json:"max_durability"`
	
	// 价值
	Value       int            `json:"value"`
	
	// 绑定类型
	BindType    string         `json:"bind_type,omitempty"` // none, equip, pickup
	
	// 唯一装备
	IsUnique    bool           `json:"is_unique,omitempty"`
	
	Tags        []string       `json:"tags,omitempty"`
}

// 武器定义（继承装备）
type Weapon struct {
	Equipment
	WeaponType WeaponType `json:"weapon_type"`
	WeaponStats WeaponStats `json:"weapon_stats"`
	Element    ElementType `json:"element,omitempty"` // 元素属性
	Skills     []string   `json:"skills,omitempty"`   // 武器技能
}

// 装备效果
type EquipmentEffect struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Trigger     string      `json:"trigger,omitempty"` // 触发条件
	Chance      float64     `json:"chance,omitempty"`
	Value       interface{} `json:"value"`
}

// 装备实例
type EquipmentInstance struct {
	EquipmentID  string                 `json:"equipment_id"`
	Durability   int                    `json:"durability"`
	Enchantments []Enchantment          `json:"enchantments,omitempty"`
	RefineLevel  int                    `json:"refine_level,omitempty"` // 精炼等级
	CustomStats  EquipmentStats         `json:"custom_stats,omitempty"`
	CustomData   map[string]interface{} `json:"custom_data,omitempty"`
}

// 套装定义
type EquipmentSet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Pieces      []string `json:"pieces"` // 装备ID列表
	
	// 套装效果
	Bonuses     []SetBonus `json:"bonuses"`
}

// 套装效果
type SetBonus struct {
	PiecesRequired int            `json:"pieces_required"`
	Stats          EquipmentStats `json:"stats,omitempty"`
	Effects        []Effect       `json:"effects,omitempty"`
	Description    string         `json:"description"`
}

// 装备管理器
type EquipmentManager struct {
	equipments map[string]*Equipment
	weapons    map[string]*Weapon
	sets       map[string]*EquipmentSet
}

func NewEquipmentManager() *EquipmentManager {
	return &EquipmentManager{
		equipments: make(map[string]*Equipment),
		weapons:    make(map[string]*Weapon),
		sets:       make(map[string]*EquipmentSet),
	}
}

func (em *EquipmentManager) AddEquipment(equip *Equipment) {
	em.equipments[equip.ID] = equip
}

func (em *EquipmentManager) AddWeapon(weapon *Weapon) {
	em.weapons[weapon.ID] = weapon
	// 同时添加到装备列表
	em.equipments[weapon.ID] = &weapon.Equipment
}

func (em *EquipmentManager) AddSet(set *EquipmentSet) {
	em.sets[set.ID] = set
}

func (em *EquipmentManager) GetEquipment(id string) *Equipment {
	return em.equipments[id]
}

func (em *EquipmentManager) GetWeapon(id string) *Weapon {
	return em.weapons[id]
}

func (em *EquipmentManager) GetSet(id string) *EquipmentSet {
	return em.sets[id]
}

func (em *EquipmentManager) GetAllEquipments() []*Equipment {
	result := make([]*Equipment, 0, len(em.equipments))
	for _, equip := range em.equipments {
		result = append(result, equip)
	}
	return result
}

func (em *EquipmentManager) GetAllWeapons() []*Weapon {
	result := make([]*Weapon, 0, len(em.weapons))
	for _, weapon := range em.weapons {
		result = append(result, weapon)
	}
	return result
}

func (em *EquipmentManager) GetEquipmentsByType(equipType EquipmentType) []*Equipment {
	result := make([]*Equipment, 0)
	for _, equip := range em.equipments {
		if equip.Type == equipType {
			result = append(result, equip)
		}
	}
	return result
}

func (em *EquipmentManager) GetWeaponsByType(weaponType WeaponType) []*Weapon {
	result := make([]*Weapon, 0)
	for _, weapon := range em.weapons {
		if weapon.WeaponType == weaponType {
			result = append(result, weapon)
		}
	}
	return result
}

// 检查角色是否可以装备
func (em *EquipmentManager) CanEquip(equipmentID string, character *Character) (bool, string) {
	equip := em.equipments[equipmentID]
	if equip == nil {
		return false, "装备不存在"
	}
	
	// 检查等级
	if character.Level < equip.LevelRequired {
		return false, "等级不足"
	}
	
	// 检查职业
	if len(equip.ClassAllowed) > 0 {
		hasClass := false
		for _, classID := range equip.ClassAllowed {
			if character.ClassID == classID {
				hasClass = true
				break
			}
		}
		if !hasClass {
			return false, "职业不符"
		}
	}
	
	return true, ""
}

// 计算套装效果
func (em *EquipmentManager) CalculateSetBonus(character *Character) ([]EquipmentStats, []Effect) {
	setCounts := make(map[string]int)
	
	// 统计套装件数
	equipIDs := []string{
		character.Equipment.Weapon,
		character.Equipment.Armor,
		character.Equipment.Helmet,
		character.Equipment.Accessory1,
		character.Equipment.Accessory2,
	}
	
	for _, equipID := range equipIDs {
		if equipID == "" {
			continue
		}
		equip := em.equipments[equipID]
		if equip != nil && equip.SetID != "" {
			setCounts[equip.SetID]++
		}
	}
	
	stats := make([]EquipmentStats, 0)
	effects := make([]Effect, 0)
	
	// 计算套装效果
	for setID, count := range setCounts {
		set := em.sets[setID]
		if set == nil {
			continue
		}
		
		for _, bonus := range set.Bonuses {
			if count >= bonus.PiecesRequired {
				stats = append(stats, bonus.Stats)
				effects = append(effects, bonus.Effects...)
			}
		}
	}
	
	return stats, effects
}

// 创建装备实例
func NewEquipmentInstance(equipmentID string) *EquipmentInstance {
	return &EquipmentInstance{
		EquipmentID:  equipmentID,
		Durability:   100,
		Enchantments: make([]Enchantment, 0),
		RefineLevel:  0,
		CustomData:   make(map[string]interface{}),
	}
}

// 计算武器伤害
func (w *Weapon) CalculateDamage() int {
	if w.WeaponStats.MinDamage == w.WeaponStats.MaxDamage {
		return w.WeaponStats.MinDamage
	}
	// 简化：返回平均值
	return (w.WeaponStats.MinDamage + w.WeaponStats.MaxDamage) / 2
}

// 序列化
func (e *Equipment) ToJSON() string {
	data, _ := json.MarshalIndent(e, "", "  ")
	return string(data)
}

func (w *Weapon) ToJSON() string {
	data, _ := json.MarshalIndent(w, "", "  ")
	return string(data)
}

func (es *EquipmentSet) ToJSON() string {
	data, _ := json.MarshalIndent(es, "", "  ")
	return string(data)
}
