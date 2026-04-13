package rpg

import (
	"encoding/json"
	"math/rand"
)

// 物品类型
type ItemType string

const (
	ItemTypeConsumable ItemType = "consumable" // 消耗品
	ItemTypeMaterial   ItemType = "material"   // 材料
	ItemTypeQuest      ItemType = "quest"      // 任务物品
	ItemTypeKey        ItemType = "key"        // 钥匙/重要物品
	ItemTypeMisc       ItemType = "misc"       // 杂项
)

// 消耗品效果类型
type ConsumableEffectType string

const (
	ConsumableEffectHealHP      ConsumableEffectType = "heal_hp"       // 恢复HP
	ConsumableEffectHealMP      ConsumableEffectType = "heal_mp"       // 恢复MP
	ConsumableEffectHealStatus  ConsumableEffectType = "heal_status"   // 治愈状态
	ConsumableEffectBuff        ConsumableEffectType = "buff"          // 临时增益
	ConsumableEffectTeleport    ConsumableEffectType = "teleport"      // 传送
	ConsumableEffectRevive      ConsumableEffectType = "revive"        // 复活
)

// 消耗品效果
type ConsumableEffect struct {
	Type       ConsumableEffectType `json:"type"`
	Value      int                  `json:"value,omitempty"`    // 数值
	Status     string               `json:"status,omitempty"`   // 状态类型
	Duration   int                  `json:"duration,omitempty"` // 持续时间
	Target     string               `json:"target,omitempty"`   // 目标: self, ally, all_allies, area
}

// 物品定义
type Item struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        ItemType       `json:"type"`
	Rarity      Rarity         `json:"rarity"`
	
	Icon        string         `json:"icon,omitempty"`     // 图标路径
	Weight      float64        `json:"weight"`             // 重量
	MaxStack    int            `json:"max_stack"`          // 最大堆叠数量
	Value       int            `json:"value"`              // 基础价值（金币）
	
	// 消耗品特有
	Effects     []ConsumableEffect `json:"effects,omitempty"`
	
	// 材料特有
	CraftingRecipes []string   `json:"crafting_recipes,omitempty"` // 可用于的合成配方ID
	
	// 任务特有
	QuestID     string         `json:"quest_id,omitempty"`
	
	// 使用限制
	LevelRequired int          `json:"level_required,omitempty"`
	ClassRequired []string     `json:"class_required,omitempty"` // 职业限制
	
	// 特殊标记
	IsUsable    bool           `json:"is_usable"`          // 是否可使用
	IsDroppable bool           `json:"is_droppable"`       // 是否可丢弃
	IsSellable  bool           `json:"is_sellable"`        // 是否可出售
	
	Tags        []string       `json:"tags,omitempty"`     // 标签
}

// 物品实例（用于存储具体状态）
type ItemInstance struct {
	ItemID    string                 `json:"item_id"`
	Count     int                    `json:"count"`
	Durability int                   `json:"durability,omitempty"` // 耐久度
	Enchantments []Enchantment       `json:"enchantments,omitempty"` // 附魔
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// 附魔
type Enchantment struct {
	Type      string `json:"type"`
	Level     int    `json:"level"`
	Effect    Effect `json:"effect"`
}

// 物品管理器
type ItemManager struct {
	items map[string]*Item
}

func NewItemManager() *ItemManager {
	return &ItemManager{
		items: make(map[string]*Item),
	}
}

func (im *ItemManager) AddItem(item *Item) {
	im.items[item.ID] = item
}

func (im *ItemManager) GetItem(id string) *Item {
	return im.items[id]
}

func (im *ItemManager) GetAllItems() []*Item {
	result := make([]*Item, 0, len(im.items))
	for _, item := range im.items {
		result = append(result, item)
	}
	return result
}

func (im *ItemManager) GetItemsByType(itemType ItemType) []*Item {
	result := make([]*Item, 0)
	for _, item := range im.items {
		if item.Type == itemType {
			result = append(result, item)
		}
	}
	return result
}

func (im *ItemManager) GetItemsByRarity(rarity Rarity) []*Item {
	result := make([]*Item, 0)
	for _, item := range im.items {
		if item.Rarity == rarity {
			result = append(result, item)
		}
	}
	return result
}

// 使用物品
func (im *ItemManager) UseItem(itemID string, target *Character) []Effect {
	item := im.items[itemID]
	if item == nil || !item.IsUsable {
		return nil
	}
	
	effects := make([]Effect, 0)
	
	for _, eff := range item.Effects {
		switch eff.Type {
		case ConsumableEffectHealHP:
			healAmount := eff.Value
			effects = append(effects, Effect{
				Type:   EffectHeal,
				TargetID: target.ID,
				Value:  healAmount,
			})
			target.CurrentStats.HP = min(target.CurrentStats.HP+healAmount, target.BaseStats.HP)
		case ConsumableEffectHealMP:
			healAmount := eff.Value
			effects = append(effects, Effect{
				Type:   EffectHeal,
				TargetID: target.ID,
				Value:  healAmount,
			})
			target.CurrentStats.MP = min(target.CurrentStats.MP+healAmount, target.BaseStats.MP)
		case ConsumableEffectHealStatus:
			target.State = CharacterStateNormal
			effects = append(effects, Effect{
				Type:   EffectBuff,
				TargetID: target.ID,
				Value:  "status_healed",
			})
		case ConsumableEffectBuff:
			effects = append(effects, Effect{
				Type:     EffectBuff,
				TargetID: target.ID,
				Value:    eff,
				Duration: eff.Duration,
			})
		case ConsumableEffectRevive:
			if target.State == CharacterStateDead {
				revivePercent := eff.Value
				target.State = CharacterStateNormal
				target.CurrentStats.HP = target.BaseStats.HP * revivePercent / 100
				effects = append(effects, Effect{
					Type:   EffectHeal,
					TargetID: target.ID,
					Value:  target.CurrentStats.HP,
				})
			}
		}
	}
	
	return effects
}

// 创建物品实例
func NewItemInstance(itemID string, count int) *ItemInstance {
	return &ItemInstance{
		ItemID: itemID,
		Count:  count,
		Durability: 100,
		Enchantments: make([]Enchantment, 0),
		CustomData: make(map[string]interface{}),
	}
}

// 序列化
func (i *Item) ToJSON() string {
	data, _ := json.MarshalIndent(i, "", "  ")
	return string(data)
}

// ExportToMap 导出为map
func (im *ItemManager) ExportToMap() map[string]interface{} {
	return map[string]interface{}{
		"items": im.items,
	}
}

// 生成随机掉落
func GenerateDrop(dropItems []DropItem, itemMgr *ItemManager) []ItemInstance {
	result := make([]ItemInstance, 0)
	
	for _, drop := range dropItems {
		if rand.Float64() <= drop.Chance {
			count := drop.MinCount
			if drop.MaxCount > drop.MinCount {
				count += rand.Intn(drop.MaxCount - drop.MinCount + 1)
			}
			result = append(result, *NewItemInstance(drop.ItemID, count))
		}
	}
	
	return result
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
