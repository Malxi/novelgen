package rpg

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// ID 生成器类型
type ID string

var idCounter uint64

func NewID(prefix string) ID {
	counter := atomic.AddUint64(&idCounter, 1)
	return ID(fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), counter))
}

// 基础属性结构
type BaseStats struct {
	HP          int `json:"hp"`          // 生命值
	MP          int `json:"mp"`          // 魔法值
	Attack      int `json:"attack"`      // 攻击力
	Defense     int `json:"defense"`     // 防御力
	Magic       int `json:"magic"`       // 魔法攻击
	Resistance  int `json:"resistance"`  // 魔法防御
	Speed       int `json:"speed"`       // 速度
	Luck        int `json:"luck"`        // 幸运
}

// 成长属性
type GrowthStats struct {
	HP          float64 `json:"hp_growth"`          // HP成长率
	MP          float64 `json:"mp_growth"`          // MP成长率
	Attack      float64 `json:"attack_growth"`      // 攻击成长率
	Defense     float64 `json:"defense_growth"`     // 防御成长率
	Magic       float64 `json:"magic_growth"`       // 魔法成长率
	Resistance  float64 `json:"resistance_growth"`  // 魔防成长率
	Speed       float64 `json:"speed_growth"`       // 速度成长率
	Luck        float64 `json:"luck_growth"`        // 幸运成长率
}

// 元素类型
type ElementType string

const (
	ElementNone     ElementType = "none"
	ElementFire     ElementType = "fire"
	ElementWater    ElementType = "water"
	ElementWind     ElementType = "wind"
	ElementEarth    ElementType = "earth"
	ElementLight    ElementType = "light"
	ElementDark     ElementType = "dark"
	ElementThunder  ElementType = "thunder"
	ElementIce      ElementType = "ice"
	ElementPoison   ElementType = "poison"
)

// 属性类型
type AttributeType string

const (
	AttributeStrength     AttributeType = "strength"     // 力量
	AttributeAgility      AttributeType = "agility"      // 敏捷
	AttributeIntelligence AttributeType = "intelligence" // 智力
	AttributeVitality     AttributeType = "vitality"     // 体质
	AttributeSpirit       AttributeType = "spirit"       // 精神
	AttributeDexterity    AttributeType = "dexterity"    // 灵巧
)

// 稀有度
type Rarity string

const (
	RarityCommon    Rarity = "common"     // 普通
	RarityUncommon  Rarity = "uncommon"   // 优秀
	RarityRare      Rarity = "rare"       // 稀有
	RarityEpic      Rarity = "epic"       // 史诗
	RarityLegendary Rarity = "legendary"  // 传说
	RarityMythic    Rarity = "mythic"     // 神话
)

// 条件类型
type ConditionType string

const (
	ConditionHasItem      ConditionType = "has_item"       // 拥有物品
	ConditionHasSkill     ConditionType = "has_skill"      // 拥有技能
	ConditionLevel        ConditionType = "level"          // 等级条件
	ConditionAttribute    ConditionType = "attribute"      // 属性条件
	ConditionQuestComplete ConditionType = "quest_complete" // 完成任务
	ConditionQuestActive  ConditionType = "quest_active"   // 任务进行中
	ConditionTime         ConditionType = "time"           // 时间条件
	ConditionLocation     ConditionType = "location"       // 位置条件
	ConditionRelationship ConditionType = "relationship"   // 关系条件
	ConditionRandom       ConditionType = "random"         // 随机条件
	ConditionFlag         ConditionType = "flag"           // 标志条件
)

// 条件结构
type Condition struct {
	Type       ConditionType  `json:"type"`
	TargetID   string         `json:"target_id,omitempty"`
	Value      interface{}    `json:"value,omitempty"`
	Operator   string         `json:"operator,omitempty"` // >, <, >=, <=, ==, !=
}

// 效果类型
type EffectType string

const (
	EffectDamage        EffectType = "damage"         // 造成伤害
	EffectHeal          EffectType = "heal"           // 治疗
	EffectBuff          EffectType = "buff"           // 增益
	EffectDebuff        EffectType = "debuff"         // 减益
	EffectAddItem       EffectType = "add_item"       // 添加物品
	EffectRemoveItem    EffectType = "remove_item"    // 移除物品
	EffectLearnSkill    EffectType = "learn_skill"    // 学习技能
	EffectTeleport      EffectType = "teleport"       // 传送
	EffectTriggerEvent  EffectType = "trigger_event"  // 触发事件
	EffectStartQuest    EffectType = "start_quest"    // 开始任务
	EffectCompleteQuest EffectType = "complete_quest" // 完成任务
	EffectModifyFlag    EffectType = "modify_flag"    // 修改标志
	EffectModifyRelation EffectType = "modify_relation" // 修改关系
	EffectSpawnEnemy    EffectType = "spawn_enemy"    // 生成敌人
	EffectChangeScene   EffectType = "change_scene"   // 切换场景
)

// 效果结构
type Effect struct {
	Type       EffectType     `json:"type"`
	TargetID   string         `json:"target_id,omitempty"`
	Value      interface{}    `json:"value,omitempty"`
	Duration   int            `json:"duration,omitempty"` // 持续时间(回合/秒)
	Chance     float64        `json:"chance,omitempty"`   // 触发概率 0-1
}

// 战斗计算结果
type BattleResult struct {
	Success    bool        `json:"success"`
	Damage     int         `json:"damage,omitempty"`
	IsCritical bool        `json:"is_critical,omitempty"`
	IsMiss     bool        `json:"is_miss,omitempty"`
	Effects    []Effect    `json:"effects,omitempty"`
}

// 序列化辅助函数
func (s BaseStats) ToJSON() string {
	data, _ := json.Marshal(s)
	return string(data)
}

func (s GrowthStats) ToJSON() string {
	data, _ := json.Marshal(s)
	return string(data)
}
