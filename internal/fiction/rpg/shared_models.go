package rpg

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/contract/story"
)

// SharedModelConverter 共享模型转换器
// 用于在 novelgen models 和 RPG 系统之间双向转换
type SharedModelConverter struct {
	World *GameWorld
}

// NewSharedModelConverter 创建共享模型转换器
func NewSharedModelConverter(world *GameWorld) *SharedModelConverter {
	return &SharedModelConverter{World: world}
}

// ExportCharacter 将 RPG Character 导出为 novelgen Character
func (smc *SharedModelConverter) ExportCharacter(char *Character) models.Character {
	return models.Character{
		Name:         char.Name,
		Aliases:      []string{},
		Age:          "", // RPG 中可能没有年龄信息
		Gender:       "",
		Race:         char.Race,
		Appearance:   "", // 从 Description 解析
		Personality:  []string{},
		Background:   char.Description,
		Motivation:   "",
		Skills:       smc.exportSkills(char.Skills),
		Abilities:    smc.exportAbilities(char),
		Affiliations: []string{},
		RoleInStory:  smc.exportRole(char.Type),
		Voice:        "",
		Notes: fmt.Sprintf("RPG Stats: HP:%d/%d MP:%d/%d ATK:%d DEF:%d",
			char.CurrentStats.HP, char.BaseStats.HP,
			char.CurrentStats.MP, char.BaseStats.MP,
			char.CurrentStats.Attack, char.CurrentStats.Defense),
	}
}

// ExportItem 将 RPG Item 导出为 novelgen Item
func (smc *SharedModelConverter) ExportItem(item *Item) models.Item {
	return models.Item{
		Name:         item.Name,
		Type:         smc.exportItemType(item.Type),
		Description:  item.Description,
		Appearance:   "",
		Function:     smc.exportItemFunction(item),
		Origin:       "",
		History:      "",
		Powers:       smc.exportEffectsToPowers(item.Effects),
		Limitations:  []string{},
		Owner:        "",
		Significance: smc.exportRarityToSignificance(item.Rarity),
		RelatedItems: []string{},
		Secrets:      models.StringList{},
		Notes:        fmt.Sprintf("RPG Value: %d, Stack: %d/%d", item.Value, 1, item.MaxStack),
	}
}

// ExportLocation 将 RPG Map 导出为 novelgen Location
func (smc *SharedModelConverter) ExportLocation(gameMap *Map) models.Location {
	connectedLocs := make([]string, 0)
	for _, conn := range gameMap.Connections {
		connectedLocs = append(connectedLocs, conn.MapID)
	}

	return models.Location{
		Name:               gameMap.Name,
		Type:               smc.exportMapType(gameMap.Type),
		Description:        gameMap.Description,
		Appearance:         "",
		Atmosphere:         "",
		SensoryDetails:     nil,
		Significance:       "RPG游戏地图",
		History:            "",
		Inhabitants:        []string{},
		ConnectedLocations: connectedLocs,
		Events:             []string{},
		Secrets:            "",
		Notes:              fmt.Sprintf("RPG Map Size: %dx%d", gameMap.Width, gameMap.Height),
	}
}

// ExportAllToNovelgen 将所有 RPG 数据导出为 novelgen 格式
func (smc *SharedModelConverter) ExportAllToNovelgen() *NovelgenExport {
	export := &NovelgenExport{
		Characters: make(map[string]models.Character),
		Items:      make(map[string]models.Item),
		Locations:  make(map[string]models.Location),
	}

	// 导出所有角色
	for _, char := range smc.World.Characters.GetAllCharacters() {
		export.Characters[char.Name] = smc.ExportCharacter(char)
	}

	// 导出所有物品
	for _, item := range smc.World.Items.GetAllItems() {
		export.Items[item.Name] = smc.ExportItem(item)
	}

	// 导出所有地图
	for _, gameMap := range smc.World.Maps.GetAllMaps() {
		export.Locations[gameMap.Name] = smc.ExportLocation(gameMap)
	}

	return export
}

// NovelgenExport 导出数据结构
type NovelgenExport struct {
	Characters map[string]models.Character `json:"characters"`
	Items      map[string]models.Item      `json:"items"`
	Locations  map[string]models.Location  `json:"locations"`
}

// ToJSON 导出为 JSON
func (ne *NovelgenExport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(ne, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// 反向导出辅助方法
func (smc *SharedModelConverter) exportSkills(skillIDs []string) []string {
	skills := make([]string, 0)
	for _, skillID := range skillIDs {
		if skill := smc.World.Skills.GetSkill(skillID); skill != nil {
			skills = append(skills, skill.Name)
		} else {
			skills = append(skills, skillID)
		}
	}
	return skills
}

func (smc *SharedModelConverter) exportAbilities(char *Character) []string {
	abilities := make([]string, 0)
	// 从角色的特殊属性导出能力
	if char.CurrentStats.HP > char.BaseStats.HP {
		abilities = append(abilities, "生命值强化")
	}
	if char.CurrentStats.Attack > char.BaseStats.Attack {
		abilities = append(abilities, "攻击力强化")
	}
	return abilities
}

func (smc *SharedModelConverter) exportRole(charType CharacterType) string {
	switch charType {
	case CharacterTypePlayer:
		return "主角"
	case CharacterTypeEnemy:
		return "反派"
	default:
		return "配角"
	}
}

func (smc *SharedModelConverter) exportItemType(itemType ItemType) string {
	switch itemType {
	case ItemTypeConsumable:
		return "消耗品"
	case ItemTypeMaterial:
		return "材料"
	case ItemTypeKey:
		return "任务道具"
	default:
		return "其他"
	}
}

func (smc *SharedModelConverter) exportItemFunction(item *Item) string {
	if len(item.Effects) > 0 {
		switch item.Effects[0].Type {
		case ConsumableEffectHealHP:
			return fmt.Sprintf("恢复%d点生命值", item.Effects[0].Value)
		case ConsumableEffectRevive:
			return "复活角色"
		default:
			return "提供增益效果"
		}
	}
	return "无特殊功能"
}

func (smc *SharedModelConverter) exportEffectsToPowers(effects []ConsumableEffect) []string {
	powers := make([]string, 0)
	for _, effect := range effects {
		switch effect.Type {
		case ConsumableEffectHealHP:
			powers = append(powers, fmt.Sprintf("恢复%d点生命", effect.Value))
		case ConsumableEffectRevive:
			powers = append(powers, "复活能力")
		case ConsumableEffectBuff:
			powers = append(powers, "增益效果")
		}
	}
	return powers
}

func (smc *SharedModelConverter) exportRarityToSignificance(rarity Rarity) string {
	switch rarity {
	case RarityLegendary:
		return "核心道具"
	case RarityEpic:
		return "重要道具"
	case RarityRare:
		return "稀有道具"
	default:
		return "普通道具"
	}
}

func (smc *SharedModelConverter) exportMapType(mapType MapType) string {
	switch mapType {
	case MapTypeCave:
		return "洞穴"
	case MapTypeTown:
		return "城镇"
	case MapTypeForest:
		return "森林"
	case MapTypeMountain:
		return "山脉"
	default:
		return "野外"
	}
}
