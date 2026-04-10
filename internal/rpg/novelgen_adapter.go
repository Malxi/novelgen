package rpg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// NovelgenProject novelgen项目结构
type NovelgenProject struct {
	ProjectPath string
	BookName    string
	
	// 数据文件
	Characters map[string]NovelgenCharacter `json:"-"`
	Items      map[string]NovelgenItem      `json:"-"`
	Locations  map[string]NovelgenLocation  `json:"-"`
	Outline    StoryOutline                 `json:"-"`
}

// NovelgenCharacter novelgen角色结构
type NovelgenCharacter struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	Age           string   `json:"age"`
	Gender        string   `json:"gender"`
	Race          string   `json:"race"`
	Appearance    string   `json:"appearance"`
	Background    string   `json:"background"`
	Personality   []string `json:"personality"`
	Motivation    string   `json:"motivation"`
	Skills        []string `json:"skills"`
	Abilities     []string `json:"abilities"`
	Affiliations  []string `json:"affiliations"`
	RoleInStory   string   `json:"role_in_story"`
	Voice         string   `json:"voice"`
	Notes         string   `json:"notes"`
}

// NovelgenItem novelgen物品结构
type NovelgenItem struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Appearance    string   `json:"appearance"`
	Function      string   `json:"function"`
	Origin        string   `json:"origin"`
	History       string   `json:"history"`
	Owner         string   `json:"owner"`
	Type          string   `json:"type"`
	Powers        []string `json:"powers"`
	Limitations   []string `json:"limitations"`
	RelatedItems  []string `json:"related_items"`
	Secrets       []string `json:"secrets"`
	Significance  string   `json:"significance"`
	Notes         string   `json:"notes"`
}

// NovelgenLocation novelgen地点结构
type NovelgenLocation struct {
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Appearance       string                 `json:"appearance"`
	Atmosphere       string                 `json:"atmosphere"`
	History          string                 `json:"history"`
	Inhabitants      []string               `json:"inhabitants"`
	ConnectedLocs    []string               `json:"connected_locations"`
	Events           []string               `json:"events"`
	Secrets          string                 `json:"secrets"`
	Notes            string                 `json:"notes"`
	SensoryDetails   map[string][]string    `json:"sensory_details"`
}

// LoadNovelgenProject 加载novelgen项目
func LoadNovelgenProject(projectPath, bookName string) (*NovelgenProject, error) {
	project := &NovelgenProject{
		ProjectPath: projectPath,
		BookName:    bookName,
		Characters:  make(map[string]NovelgenCharacter),
		Items:       make(map[string]NovelgenItem),
		Locations:   make(map[string]NovelgenLocation),
	}

	storyPath := filepath.Join(projectPath, "books", bookName, "story")

	// 加载角色
	charPath := filepath.Join(storyPath, "craft", "characters.json")
	if err := project.loadJSON(charPath, &project.Characters); err != nil {
		return nil, fmt.Errorf("加载角色失败: %v", err)
	}

	// 加载物品
	itemPath := filepath.Join(storyPath, "craft", "items.json")
	if err := project.loadJSON(itemPath, &project.Items); err != nil {
		return nil, fmt.Errorf("加载物品失败: %v", err)
	}

	// 加载地点
	locPath := filepath.Join(storyPath, "craft", "locations.json")
	if err := project.loadJSON(locPath, &project.Locations); err != nil {
		return nil, fmt.Errorf("加载地点失败: %v", err)
	}

	// 加载大纲
	outlinePath := filepath.Join(storyPath, "compose", "outline.json")
	outlineData, err := ioutil.ReadFile(outlinePath)
	if err != nil {
		return nil, fmt.Errorf("读取大纲失败: %v", err)
	}
	if err := json.Unmarshal(outlineData, &project.Outline); err != nil {
		return nil, fmt.Errorf("解析大纲失败: %v", err)
	}

	return project, nil
}

// loadJSON 加载JSON文件
func (np *NovelgenProject) loadJSON(path string, v interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ConvertToRPG 转换为RPG世界
func (np *NovelgenProject) ConvertToRPG() (*GameWorld, error) {
	world := NewGameWorld()

	// 1. 转换角色
	if err := np.convertCharacters(world); err != nil {
		return nil, fmt.Errorf("转换角色失败: %v", err)
	}

	// 2. 转换物品
	if err := np.convertItems(world); err != nil {
		return nil, fmt.Errorf("转换物品失败: %v", err)
	}

	// 3. 转换地点
	if err := np.convertLocations(world); err != nil {
		return nil, fmt.Errorf("转换地点失败: %v", err)
	}

	// 4. 转换大纲为任务和事件
	storyWorld := &StoryWorld{
		GameWorld:    world,
		Outline:      &np.Outline,
		CharacterMap: make(map[string]string),
		LocationMap:  make(map[string]string),
		QuestMap:     make(map[string]string),
		EventMap:     make(map[string]string),
	}
	storyWorld.ConvertOutlineToRPG()

	return world, nil
}

// convertCharacters 转换角色
func (np *NovelgenProject) convertCharacters(world *GameWorld) error {
	for name, char := range np.Characters {
		// 推断角色类型
		charType := np.inferCharacterType(char)
		
		// 推断属性
		baseStats := np.inferStatsFromCharacter(char)
		growthStats := np.inferGrowthFromCharacter(char)
		
		// 创建角色模板
		template := &CharacterTemplate{
			ID:          sanitizeID(name),
			Name:        name,
			Description: char.Background,
			Type:        charType,
			Race:        char.Race,
			BaseStats:   baseStats,
			GrowthStats: growthStats,
			Rarity:      np.inferRarityFromRole(char.RoleInStory),
		}

		// 从技能推断默认技能
		template.DefaultSkills = np.inferSkills(char.Skills)

		world.Characters.AddTemplate(template)

		// 创建角色实例
		character := NewCharacter(template, name)
		world.Characters.AddCharacterInstance(character)

		// 如果是主角，设置为玩家
		if char.RoleInStory == "主角" || char.RoleInStory == " protagonist" {
			world.SetPlayer(character)
		}
	}

	return nil
}

// convertItems 转换物品
func (np *NovelgenProject) convertItems(world *GameWorld) error {
	for name, item := range np.Items {
		// 推断物品类型
		itemType := np.inferItemType(item)
		
		// 创建物品
		rpgItem := &Item{
			ID:          sanitizeID(name),
			Name:        name,
			Description: item.Description,
			Type:        itemType,
			Rarity:      np.inferRarityFromSignificance(item.Significance),
			Weight:      0.1,
			MaxStack:    99,
			Value:       np.inferItemValue(item),
			IsUsable:    len(item.Powers) > 0,
			IsDroppable: true,
			IsSellable:  true,
		}

		// 转换能力为效果
		if len(item.Powers) > 0 {
			rpgItem.Effects = np.convertPowersToEffects(item.Powers)
		}

		world.Items.AddItem(rpgItem)
	}

	return nil
}

// convertLocations 转换地点
func (np *NovelgenProject) convertLocations(world *GameWorld) error {
	for name, loc := range np.Locations {
		// 推断地图类型
		mapType := np.inferMapTypeFromLocation(loc)
		
		// 创建地图
		gameMap := &Map{
			ID:          sanitizeID(name),
			Name:        name,
			Description: loc.Description,
			Type:        mapType,
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
		for _, connected := range loc.ConnectedLocs {
			gameMap.Connections = append(gameMap.Connections, MapConnection{
				Direction: "unknown",
				MapID:     sanitizeID(connected),
			})
		}

		world.Maps.AddMap(gameMap)
	}

	return nil
}

// 推断方法

func (np *NovelgenProject) inferCharacterType(char NovelgenCharacter) CharacterType {
	switch char.RoleInStory {
	case "主角", "protagonist":
		return CharacterTypePlayer
	case "反派", "antagonist", "反派基层喽啰", "反派中层干部":
		return CharacterTypeEnemy
	default:
		return CharacterTypeNPC
	}
}

func (np *NovelgenProject) inferStatsFromCharacter(char NovelgenCharacter) BaseStats {
	// 根据角色在故事中的角色推断属性
	base := BaseStats{
		HP: 100, MP: 50, Attack: 10, Defense: 10,
		Magic: 10, Resistance: 10, Speed: 10, Luck: 10,
	}

	// 根据技能调整
	for _, skill := range char.Skills {
		switch {
		case strings.Contains(skill, "剑") || strings.Contains(skill, "刀") || strings.Contains(skill, "格斗"):
			base.Attack += 5
		case strings.Contains(skill, "盾") || strings.Contains(skill, "防"):
			base.Defense += 5
		case strings.Contains(skill, "法") || strings.Contains(skill, "术") || strings.Contains(skill, "咒"):
			base.Magic += 5
			base.MP += 20
		case strings.Contains(skill, "速") || strings.Contains(skill, "轻") || strings.Contains(skill, "隐"):
			base.Speed += 5
		}
	}

	// 根据能力调整
	for _, ability := range char.Abilities {
		if strings.Contains(ability, "复活") || strings.Contains(ability, "不死") {
			base.HP += 50
		}
	}

	return base
}

func (np *NovelgenProject) inferGrowthFromCharacter(char NovelgenCharacter) GrowthStats {
	// 主角成长更高
	if char.RoleInStory == "主角" {
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

func (np *NovelgenProject) inferRarityFromRole(role string) Rarity {
	switch role {
	case "主角":
		return RarityLegendary
	case "反派":
		return RarityEpic
	case "反派中层干部":
		return RarityRare
	default:
		return RarityCommon
	}
}

func (np *NovelgenProject) inferRarityFromSignificance(significance string) Rarity {
	switch {
	case strings.Contains(significance, "核心") || strings.Contains(significance, "关键"):
		return RarityLegendary
	case strings.Contains(significance, "重要"):
		return RarityEpic
	default:
		return RarityCommon
	}
}

func (np *NovelgenProject) inferItemType(item NovelgenItem) ItemType {
	switch {
	case strings.Contains(item.Type, "消耗") || strings.Contains(item.Type, "药"):
		return ItemTypeConsumable
	case strings.Contains(item.Type, "材料"):
		return ItemTypeMaterial
	case strings.Contains(item.Type, "钥匙") || strings.Contains(item.Type, "任务"):
		return ItemTypeKey
	default:
		return ItemTypeMisc
	}
}

func (np *NovelgenProject) inferItemValue(item NovelgenItem) int {
	// 根据重要性推断价值
	baseValue := 100
	
	if strings.Contains(item.Significance, "核心") {
		baseValue = 10000
	} else if strings.Contains(item.Significance, "重要") {
		baseValue = 1000
	}
	
	// 根据能力数量调整
	baseValue += len(item.Powers) * 100
	
	return baseValue
}

func (np *NovelgenProject) inferSkills(skills []string) []string {
	rpgSkills := make([]string, 0)
	
	for _, skill := range skills {
		// 简化处理，实际应该映射到具体的技能ID
		rpgSkills = append(rpgSkills, sanitizeID(skill))
	}
	
	return rpgSkills
}

func (np *NovelgenProject) convertPowersToEffects(powers []string) []ConsumableEffect {
	effects := make([]ConsumableEffect, 0)
	
	for _, power := range powers {
		effect := ConsumableEffect{
			Type:   ConsumableEffectBuff,
			Target: "self",
		}
		
		// 根据能力描述推断效果
		switch {
		case strings.Contains(power, "治疗") || strings.Contains(power, "恢复"):
			effect.Type = ConsumableEffectHealHP
			effect.Value = 50
		case strings.Contains(power, "感知") || strings.Contains(power, "扫描"):
			effect.Type = ConsumableEffectBuff
			effect.Value = 10
		case strings.Contains(power, "复活"):
			effect.Type = ConsumableEffectRevive
			effect.Value = 50
		}
		
		effects = append(effects, effect)
	}
	
	return effects
}

func (np *NovelgenProject) inferMapTypeFromLocation(loc NovelgenLocation) MapType {
	switch {
	case strings.Contains(loc.Name, "矿") || strings.Contains(loc.Name, "洞") || strings.Contains(loc.Name, "穴"):
		return MapTypeCave
	case strings.Contains(loc.Name, "城") || strings.Contains(loc.Name, "镇") || strings.Contains(loc.Name, "村"):
		return MapTypeTown
	case strings.Contains(loc.Name, "林") || strings.Contains(loc.Name, "森") || strings.Contains(loc.Name, "木"):
		return MapTypeForest
	case strings.Contains(loc.Name, "山") || strings.Contains(loc.Name, "峰") || strings.Contains(loc.Name, "岭"):
		return MapTypeMountain
	default:
		return MapTypeField
	}
}

// GetProjectSummary 获取项目摘要
func (np *NovelgenProject) GetProjectSummary() map[string]interface{} {
	return map[string]interface{}{
		"book_name":    np.BookName,
		"characters":   len(np.Characters),
		"items":        len(np.Items),
		"locations":    len(np.Locations),
		"parts":        len(np.Outline.Parts),
	}
}

// ExportRPGData 导出RPG数据到文件
func (np *NovelgenProject) ExportRPGData(outputPath string) error {
	world, err := np.ConvertToRPG()
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"project_summary": np.GetProjectSummary(),
		"world_data":      world.SaveToJSON(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(outputPath, jsonData, 0644)
}
