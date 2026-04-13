package rpg

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/models"
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

// ============================================
// Novelgen → RPG 转换 (已有功能)
// ============================================

// ConvertCharacter 将 novelgen Character 转换为 RPG Character
func (smc *SharedModelConverter) ConvertCharacter(char models.Character) *Character {
	// 推断角色类型
	charType := smc.inferCharacterType(char.RoleInStory)

	// 推断属性
	baseStats := smc.inferStatsFromSkills(char.Skills, char.Abilities)
	growthStats := smc.inferGrowthStats(char.RoleInStory)

	// 创建角色模板
	template := &CharacterTemplate{
		ID:          sanitizeID(char.Name),
		Name:        char.Name,
		Description: char.Background,
		Type:        charType,
		Race:        char.Race,
		BaseStats:   baseStats,
		GrowthStats: growthStats,
		Rarity:      smc.inferRarity(char.RoleInStory),
	}

	// 转换技能
	template.DefaultSkills = smc.convertSkills(char.Skills)

	// 创建角色实例
	character := NewCharacter(template, char.Name)

	// 设置描述（合并外观和背景）
	if char.Appearance != "" {
		character.Description = char.Appearance
	}
	if char.Background != "" {
		if character.Description != "" {
			character.Description += "\n\n"
		}
		character.Description += char.Background
	}

	return character
}

// ConvertItem 将 novelgen Item 转换为 RPG Item
func (smc *SharedModelConverter) ConvertItem(item models.Item) *Item {
	rpgItem := &Item{
		ID:          sanitizeID(item.Name),
		Name:        item.Name,
		Description: item.Description,
		Type:        smc.inferItemType(item.Type),
		Rarity:      smc.inferRarity(item.Significance),
		Weight:      0.1,
		MaxStack:    99,
		Value:       smc.inferItemValue(item.Significance, len(item.Powers)),
		IsUsable:    len(item.Powers) > 0,
		IsDroppable: true,
		IsSellable:  true,
	}

	// 转换能力为效果
	if len(item.Powers) > 0 {
		rpgItem.Effects = smc.convertPowersToEffects(item.Powers)
	}

	return rpgItem
}

// ConvertLocation 将 novelgen Location 转换为 RPG Map
func (smc *SharedModelConverter) ConvertLocation(loc models.Location) *Map {
	gameMap := &Map{
		ID:          sanitizeID(loc.Name),
		Name:        loc.Name,
		Description: loc.Description,
		Type:        smc.inferMapType(loc.Name),
		Width:       20,
		Height:      15,
		TileSize:    32,
		Entities:    make([]MapEntity, 0),
		Teleports:   make([]TeleportPoint, 0),
		Regions:     make([]MapRegion, 0),
		Connections: make([]MapConnection, 0),
		LightLevel:  100,
	}

	// 添加连接
	for _, connected := range loc.ConnectedLocations {
		gameMap.Connections = append(gameMap.Connections, MapConnection{
			Direction: "unknown",
			MapID:     sanitizeID(connected),
		})
	}

	return gameMap
}

// ============================================
// RPG → Novelgen 转换 (新增功能)
// ============================================

// ExportCharacter 将 RPG Character 导出为 novelgen Character
func (smc *SharedModelConverter) ExportCharacter(char *Character) models.Character {
	return models.Character{
		Name:        char.Name,
		Aliases:     []string{},
		Age:         "", // RPG 中可能没有年龄信息
		Gender:      "",
		Race:        char.Race,
		Appearance:  "", // 从 Description 解析
		Personality: []string{},
		Background:  char.Description,
		Motivation:  "",
		Skills:      smc.exportSkills(char.Skills),
		Abilities:   smc.exportAbilities(char),
		Affiliations: []string{},
		RoleInStory: smc.exportRole(char.Type),
		Voice:       "",
		Notes:       fmt.Sprintf("RPG Stats: HP:%d/%d MP:%d/%d ATK:%d DEF:%d", 
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

// ============================================
// 批量导出功能
// ============================================

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

// ============================================
// 辅助方法
// ============================================

func (smc *SharedModelConverter) inferCharacterType(role string) CharacterType {
	switch role {
	case "主角", "protagonist":
		return CharacterTypePlayer
	case "反派", "antagonist", "反派基层喽啰", "反派中层干部":
		return CharacterTypeEnemy
	default:
		return CharacterTypeNPC
	}
}

func (smc *SharedModelConverter) inferStatsFromSkills(skills, abilities []string) BaseStats {
	base := BaseStats{
		HP: 100, MP: 50, Attack: 10, Defense: 10,
		Magic: 10, Resistance: 10, Speed: 10, Luck: 10,
	}

	for _, skill := range skills {
		switch {
		case strContains(skill, "剑", "刀", "格斗"):
			base.Attack += 5
		case strContains(skill, "盾", "防"):
			base.Defense += 5
		case strContains(skill, "法", "术", "咒"):
			base.Magic += 5
			base.MP += 20
		case strContains(skill, "速", "轻", "隐"):
			base.Speed += 5
		}
	}

	for _, ability := range abilities {
		if strContains(ability, "复活", "不死") {
			base.HP += 50
		}
	}

	return base
}

func (smc *SharedModelConverter) inferGrowthStats(role string) GrowthStats {
	if role == "主角" {
		return GrowthStats{
			HP: 12, MP: 8, Attack: 3, Defense: 2,
			Magic: 3, Resistance: 2, Speed: 2, Luck: 1,
		}
	}
	return GrowthStats{
		HP: 8, MP: 4, Attack: 2, Defense: 1.5,
		Magic: 1.5, Resistance: 1.5, Speed: 1, Luck: 0.5,
	}
}

func (smc *SharedModelConverter) inferRarity(significance string) Rarity {
	switch {
	case strContains(significance, "核心", "关键", "主角"):
		return RarityLegendary
	case strContains(significance, "重要", "反派"):
		return RarityEpic
	case strContains(significance, "稀有"):
		return RarityRare
	default:
		return RarityCommon
	}
}

func (smc *SharedModelConverter) inferItemType(itemType string) ItemType {
	switch {
	case strContains(itemType, "消耗", "药"):
		return ItemTypeConsumable
	case strContains(itemType, "材料"):
		return ItemTypeMaterial
	case strContains(itemType, "钥匙", "任务"):
		return ItemTypeKey
	default:
		return ItemTypeMisc
	}
}

func (smc *SharedModelConverter) inferItemValue(significance string, powerCount int) int {
	baseValue := 100
	if strContains(significance, "核心") {
		baseValue = 10000
	} else if strContains(significance, "重要") {
		baseValue = 1000
	}
	return baseValue + powerCount*100
}

func (smc *SharedModelConverter) inferMapType(name string) MapType {
	switch {
	case strContains(name, "矿", "洞", "穴"):
		return MapTypeCave
	case strContains(name, "城", "镇", "村"):
		return MapTypeTown
	case strContains(name, "林", "森", "木"):
		return MapTypeForest
	case strContains(name, "山", "峰", "岭"):
		return MapTypeMountain
	default:
		return MapTypeField
	}
}

func (smc *SharedModelConverter) convertSkills(skills []string) []string {
	rpgSkills := make([]string, 0)
	for _, skill := range skills {
		rpgSkills = append(rpgSkills, sanitizeID(skill))
	}
	return rpgSkills
}

func (smc *SharedModelConverter) convertPowersToEffects(powers []string) []ConsumableEffect {
	effects := make([]ConsumableEffect, 0)
	for _, power := range powers {
		effect := ConsumableEffect{Type: ConsumableEffectBuff, Target: "self"}
		switch {
		case strContains(power, "治疗", "恢复"):
			effect.Type = ConsumableEffectHealHP
			effect.Value = 50
		case strContains(power, "感知", "扫描"):
			effect.Type = ConsumableEffectBuff
			effect.Value = 10
		case strContains(power, "复活"):
			effect.Type = ConsumableEffectRevive
			effect.Value = 50
		}
		effects = append(effects, effect)
	}
	return effects
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

// 工具函数
func strContains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || strFind(s, substr))) {
			return true
		}
	}
	return false
}

func strFind(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
