package rpg

import (
	"encoding/json"
	"math/rand"
)

// 地图类型
type MapType string

const (
	MapTypeField      MapType = "field"      // 野外
	MapTypeDungeon    MapType = "dungeon"    // 地下城
	MapTypeTown       MapType = "town"       // 城镇
	MapTypeCastle     MapType = "castle"     // 城堡
	MapTypeForest     MapType = "forest"     // 森林
	MapTypeMountain   MapType = "mountain"   // 山脉
	MapTypeCave       MapType = "cave"       // 洞穴
	MapTypeDesert     MapType = "desert"     // 沙漠
	MapTypeSnowfield  MapType = "snowfield"  // 雪原
	MapTypeSwamp      MapType = "swamp"      // 沼泽
	MapTypeOcean      MapType = "ocean"      // 海洋
	MapTypeSky        MapType = "sky"        // 天空
	MapTypeSpecial    MapType = "special"    // 特殊
)

// 地形类型
type TerrainType string

const (
	TerrainNormal    TerrainType = "normal"    // 普通
	TerrainWater     TerrainType = "water"     // 水域
	TerrainMountain  TerrainType = "mountain"  // 山地
	TerrainForest    TerrainType = "forest"    // 森林
	TerrainDesert    TerrainType = "desert"    // 沙漠
	TerrainSwamp     TerrainType = "swamp"     // 沼泽
	TerrainLava      TerrainType = "lava"      // 熔岩
	TerrainIce       TerrainType = "ice"       // 冰面
	TerrainPoison    TerrainType = "poison"    // 毒沼
	TerrainCloud     TerrainType = "cloud"     // 云
	TerrainWall      TerrainType = "wall"      // 墙壁（不可通行）
)

// 传送点
type TeleportPoint struct {
	ID          string  `json:"id"`
	Name        string  `json:"name,omitempty"`
	TargetMapID string  `json:"target_map_id"`
	TargetX     float64 `json:"target_x"`
	TargetY     float64 `json:"target_y"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Condition   *Condition `json:"condition,omitempty"` // 使用条件
}

// 地图实体（NPC、敌人、物品等）
type MapEntity struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // npc, enemy, item, trigger
	EntityID    string                 `json:"entity_id"` // 实际实体ID
	Name        string                 `json:"name,omitempty"`
	X           float64                `json:"x"`
	Y           float64                `json:"y"`
	Direction   string                 `json:"direction,omitempty"`
	IsVisible   bool                   `json:"is_visible"`
	IsActive    bool                   `json:"is_active"`
	RespawnTime int                    `json:"respawn_time,omitempty"` // 重生时间（秒）
	SpawnCondition *Condition          `json:"spawn_condition,omitempty"`
	CustomData  map[string]interface{} `json:"custom_data,omitempty"`
}

// 区域
type MapRegion struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	X           float64     `json:"x"`
	Y           float64     `json:"y"`
	Width       float64     `json:"width"`
	Height      float64     `json:"height"`
	Type        string      `json:"type,omitempty"` // 区域类型: safe, battle, shop, etc
	Music       string      `json:"music,omitempty"`
	Events      []string    `json:"events,omitempty"` // 触发的事件ID
}

// 地图格子
type MapTile struct {
	X           int         `json:"x"`
	Y           int         `json:"y"`
	Terrain     TerrainType `json:"terrain"`
	IsPassable  bool        `json:"is_passable"`
	IsTransparent bool      `json:"is_transparent"`
	EventID     string      `json:"event_id,omitempty"` // 格子触发的事件
	Height      int         `json:"height,omitempty"`   // 高度
}

// 地图定义
type Map struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        MapType        `json:"type"`
	
	// 尺寸
	Width       int            `json:"width"`
	Height      int            `json:"height"`
	TileSize    int            `json:"tile_size"` // 格子大小（像素）
	
	// 网格数据
	Tiles       [][]MapTile    `json:"tiles,omitempty"`
	
	// 实体
	Entities    []MapEntity    `json:"entities"`
	
	// 传送点
	Teleports   []TeleportPoint `json:"teleports"`
	
	// 区域
	Regions     []MapRegion    `json:"regions,omitempty"`
	
	// 环境
	Background  string         `json:"background,omitempty"` // 背景图
	Music       string         `json:"music,omitempty"`
	Ambience    string         `json:"ambience,omitempty"`   // 环境音效
	
	// 天气
	Weather     string         `json:"weather,omitempty"`
	WeatherIntensity float64   `json:"weather_intensity,omitempty"`
	
	// 光照
	LightLevel  int            `json:"light_level"` // 0-100
	IsIndoor    bool           `json:"is_indoor"`
	
	// 战斗设置
	EncounterRate float64    `json:"encounter_rate,omitempty"` // 遇敌率
	EncounterEnemies []string `json:"encounter_enemies,omitempty"` // 可能遇到的敌人
	
	// 连接的其他地图
	Connections []MapConnection `json:"connections,omitempty"`
	
	// 父地图（用于子区域）
	ParentMapID string         `json:"parent_map_id,omitempty"`
	
	// 限制
	LevelMin    int            `json:"level_min,omitempty"`
	LevelMax    int            `json:"level_max,omitempty"`
	QuestRequired string       `json:"quest_required,omitempty"`
	
	Tags        []string       `json:"tags,omitempty"`
}

// 地图连接
type MapConnection struct {
	Direction   string  `json:"direction"` // north, south, east, west, up, down
	MapID       string  `json:"map_id"`
	X           float64 `json:"x,omitempty"`
	Y           float64 `json:"y,omitempty"`
	Condition   *Condition `json:"condition,omitempty"`
}

// 地图管理器
type MapManager struct {
	maps map[string]*Map
}

func NewMapManager() *MapManager {
	return &MapManager{
		maps: make(map[string]*Map),
	}
}

func (mm *MapManager) AddMap(m *Map) {
	mm.maps[m.ID] = m
}

func (mm *MapManager) GetMap(id string) *Map {
	return mm.maps[id]
}

func (mm *MapManager) GetAllMaps() []*Map {
	result := make([]*Map, 0, len(mm.maps))
	for _, m := range mm.maps {
		result = append(result, m)
	}
	return result
}

func (mm *MapManager) GetMapsByType(mapType MapType) []*Map {
	result := make([]*Map, 0)
	for _, m := range mm.maps {
		if m.Type == mapType {
			result = append(result, m)
		}
	}
	return result
}

// 获取指定位置的实体
func (mm *MapManager) GetEntitiesAt(mapID string, x, y float64, radius float64) []MapEntity {
	m := mm.maps[mapID]
	if m == nil {
		return nil
	}
	
	result := make([]MapEntity, 0)
	for _, entity := range m.Entities {
		dx := entity.X - x
		dy := entity.Y - y
		distance := dx*dx + dy*dy
		if distance <= radius*radius {
			result = append(result, entity)
		}
	}
	return result
}

// 获取指定位置的传送点
func (mm *MapManager) GetTeleportAt(mapID string, x, y float64) *TeleportPoint {
	m := mm.maps[mapID]
	if m == nil {
		return nil
	}
	
	for _, tp := range m.Teleports {
		if tp.X == x && tp.Y == y {
			return &tp
		}
	}
	return nil
}

// 检查位置是否可通行
func (mm *MapManager) IsPassable(mapID string, x, y int) bool {
	m := mm.maps[mapID]
	if m == nil {
		return false
	}
	
	if x < 0 || x >= m.Width || y < 0 || y >= m.Height {
		return false
	}
	
	if len(m.Tiles) > 0 && len(m.Tiles[y]) > x {
		return m.Tiles[y][x].IsPassable
	}
	
	return true
}

// 获取指定位置的区域
func (mm *MapManager) GetRegionAt(mapID string, x, y float64) *MapRegion {
	m := mm.maps[mapID]
	if m == nil {
		return nil
	}
	
	for _, region := range m.Regions {
		if x >= region.X && x < region.X+region.Width &&
		   y >= region.Y && y < region.Y+region.Height {
			return &region
		}
	}
	return nil
}

// 添加实体到地图
func (mm *MapManager) AddEntity(mapID string, entity MapEntity) bool {
	m := mm.maps[mapID]
	if m == nil {
		return false
	}
	
	m.Entities = append(m.Entities, entity)
	return true
}

// 移除实体
func (mm *MapManager) RemoveEntity(mapID, entityID string) bool {
	m := mm.maps[mapID]
	if m == nil {
		return false
	}
	
	for i, entity := range m.Entities {
		if entity.ID == entityID {
			m.Entities = append(m.Entities[:i], m.Entities[i+1:]...)
			return true
		}
	}
	return false
}

// 移动实体
func (mm *MapManager) MoveEntity(mapID, entityID string, newX, newY float64) bool {
	m := mm.maps[mapID]
	if m == nil {
		return false
	}
	
	for i := range m.Entities {
		if m.Entities[i].ID == entityID {
			m.Entities[i].X = newX
			m.Entities[i].Y = newY
			return true
		}
	}
	return false
}

// 获取随机遭遇
func (mm *MapManager) GetRandomEncounter(mapID string) string {
	m := mm.maps[mapID]
	if m == nil || len(m.EncounterEnemies) == 0 {
		return ""
	}
	
	// 检查遇敌率
	if rand.Float64() > m.EncounterRate {
		return ""
	}
	
	// 随机选择敌人
	index := rand.Intn(len(m.EncounterEnemies))
	return m.EncounterEnemies[index]
}

// 创建网格地图
func CreateGridMap(id, name string, width, height int, defaultTerrain TerrainType) *Map {
	tiles := make([][]MapTile, height)
	for y := 0; y < height; y++ {
		tiles[y] = make([]MapTile, width)
		for x := 0; x < width; x++ {
			tiles[y][x] = MapTile{
				X:          x,
				Y:          y,
				Terrain:    defaultTerrain,
				IsPassable: defaultTerrain != TerrainWall && defaultTerrain != TerrainWater && defaultTerrain != TerrainMountain,
				IsTransparent: defaultTerrain != TerrainWall,
			}
		}
	}
	
	return &Map{
		ID:          id,
		Name:        name,
		Width:       width,
		Height:      height,
		TileSize:    32,
		Tiles:       tiles,
		Entities:    make([]MapEntity, 0),
		Teleports:   make([]TeleportPoint, 0),
		Regions:     make([]MapRegion, 0),
		Connections: make([]MapConnection, 0),
		LightLevel:  100,
		EncounterRate: 0.1,
		EncounterEnemies: make([]string, 0),
	}
}

// 序列化
func (m *Map) ToJSON() string {
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}

// ExportToMap 导出为map
func (mm *MapManager) ExportToMap() map[string]interface{} {
	return map[string]interface{}{
		"maps": mm.maps,
	}
}
