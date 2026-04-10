# Novelgen + RPG 系统集成指南

## 概述

这个集成方案将你的 novelgen 小说创作系统与 RPG 数值系统深度结合，实现：

1. **自动数值化** - 将小说角色、物品、地点自动转换为 RPG 数据
2. **剧情推演** - 模拟小说剧情发展，验证剧情合理性
3. **数值平衡** - 通过战斗模拟调整角色强度和剧情难度
4. **游戏化** - 为小说提供游戏化的数值支撑

## 目录结构

```
novelgen/
├── books/
│   └── mine/                    # 你的小说项目
│       ├── story/
│       │   ├── craft/          # 创作数据
│       │   │   ├── characters.json   # 角色数据
│       │   │   ├── items.json        # 物品数据
│       │   │   └── locations.json    # 地点数据
│       │   ├── compose/        # 大纲
│       │   │   └── outline.json
│       │   └── ...
│       └── rpg_data.json       # 生成的 RPG 数据
├── internal/rpg/               # RPG 系统核心
│   ├── novelgen_adapter.go     # novelgen 集成适配器
│   ├── simulation.go           # 剧情推演引擎
│   └── ...
└── cmd/
    └── novelgen-rpg/           # 集成命令行工具
        └── main.go
```

## 使用方法

### 1. 快速开始

```bash
# 编译工具
go build -o novelgen-rpg.exe ./cmd/novelgen-rpg

# 转换你的小说项目
./novelgen-rpg.exe -b mine

# 转换并推演特定章节
./novelgen-rpg.exe -b mine -s P1-V1-C1
```

### 2. 在代码中使用

```go
package main

import (
    "fmt"
    "novelgen/internal/rpg"
)

func main() {
    // 加载 novelgen 项目
    project, err := rpg.LoadNovelgenProject(".", "mine")
    if err != nil {
        panic(err)
    }

    // 显示项目摘要
    summary := project.GetProjectSummary()
    fmt.Printf("项目: %s\n", summary["book_name"])
    fmt.Printf("角色: %d\n", summary["characters"])
    fmt.Printf("物品: %d\n", summary["items"])
    fmt.Printf("地点: %d\n", summary["locations"])

    // 转换为 RPG 世界
    world, err := project.ConvertToRPG()
    if err != nil {
        panic(err)
    }

    // 创建推演引擎
    engine := rpg.NewSimulationEngine(world)

    // 推演章节
    result, err := engine.SimulateChapter("P1-V1-C1")
    if err != nil {
        panic(err)
    }

    // 打印推演结果
    fmt.Printf("\n章节: %s\n", result.ChapterName)
    fmt.Printf("步骤数: %d\n", len(result.Steps))
    
    for _, step := range result.Steps {
        fmt.Printf("\n[%s] %s\n", step.Type, step.Description)
        for _, res := range step.Results {
            fmt.Printf("  - %s\n", res.Message)
        }
    }

    // 导出 RPG 数据
    project.ExportRPGData("rpg_output.json")
}
```

## 转换规则

### 角色转换

| novelgen 字段 | RPG 系统 | 转换规则 |
|--------------|---------|---------|
| `name` | `Name` | 直接使用 |
| `role_in_story` | `Type` | 主角→Player, 反派→Enemy, 其他→NPC |
| `skills` | `BaseStats` | 根据技能类型调整属性 |
| `abilities` | `BaseStats` | 特殊能力影响属性（如复活+HP） |
| `background` | `Description` | 直接使用 |
| `race` | `Race` | 直接使用 |

**属性推断示例：**
- 技能包含"剑/刀/格斗" → 攻击+5
- 技能包含"盾/防" → 防御+5
- 技能包含"法/术/咒" → 魔法+5, MP+20
- 能力包含"复活" → HP+50

### 物品转换

| novelgen 字段 | RPG 系统 | 转换规则 |
|--------------|---------|---------|
| `name` | `Name` | 直接使用 |
| `description` | `Description` | 直接使用 |
| `type` | `Type` | 根据关键词推断 |
| `powers` | `Effects` | 转换为消耗品效果 |
| `significance` | `Rarity/Value` | 核心→传说, 重要→史诗 |

**物品类型推断：**
- 包含"消耗/药" → Consumable
- 包含"材料" → Material
- 包含"钥匙" → Key

### 地点转换

| novelgen 字段 | RPG 系统 | 转换规则 |
|--------------|---------|---------|
| `name` | `Name` | 直接使用 |
| `description` | `Description` | 直接使用 |
| `name` | `Type` | 根据关键词推断 |
| `connected_locations` | `Connections` | 创建地图连接 |

**地图类型推断：**
- 包含"矿/洞/穴" → Cave
- 包含"城/镇/村" → Town
- 包含"林/森" → Forest
- 包含"山/峰" → Mountain

## 剧情推演

### 推演类型

1. **战斗推演** (`ObjectiveKill`)
   - 回合制战斗模拟
   - AI自动决策
   - 记录伤害和行动

2. **对话推演** (`ObjectiveTalk`)
   - 触发NPC对话事件
   - 记录对话内容

3. **收集推演** (`ObjectiveCollect`)
   - 自动添加物品到背包
   - 记录收集数量

4. **移动推演** (`ObjectiveReach`)
   - 移动角色到目标地点
   - 记录位置变化

### AI决策

**玩家AI：**
- 70% 概率使用技能
- 30% 概率普通攻击

**敌人AI：**
- 80% 概率攻击
- 20% 概率使用技能（如果有）

## 实战示例

### 示例1：转换并推演

```bash
# 转换小说项目
./novelgen-rpg.exe -b mine

# 输出：
# 正在加载 novelgen 项目: ./mine
# 
# ========== 项目摘要 ==========
# 书籍: mine
# 角色: 63
# 物品: 7
# 地点: 73
# 部分: 1
# 
# 正在转换为 RPG 世界...
# ✓ RPG 世界创建成功
# 
# ========== 角色列表 ==========
# [主角] 林砚 - HP:100/100 MP:50/50 战力:216
# [敌人] 矿监周虎 - HP:80/80 MP:20/20 战力:187
# [NPC] 白晓 - HP:50/50 MP:30/30 战力:175
# ...
# 
# ========== 地图列表 ==========
# [cave] 黑风灵石矿丙字三号矿道
# [town] 青牛镇
# ...
# 
# ========== 任务列表 ==========
# [main] 寒矿醒转，首死触发复生 - 目标:2
# [main] 二度殒命，始知复生有耗 - 目标:1
# ...
# 
# ✓ 导出成功
```

### 示例2：推演特定章节

```bash
./novelgen-rpg.exe -b mine -s P1-V1-C1

# 输出：
# ========== 推演章节: P1-V1-C1 ==========
# 章节: 寒矿醒转，首死触发复生
# 步骤数: 2
# 成功: true
# 
# 推演过程:
# 
# [kill] 战斗胜利！击败了 矿监周虎
#   - 林砚 攻击了 矿监周虎，造成 36 点伤害
#   - 矿监周虎 攻击了 林砚，造成 5 点伤害
#   - 林砚 对 矿监周虎 使用了 skill_slash
# 
# [event] 触发了事件: 寒矿醒转，首死触发复生
#   - 执行了 3 个事件命令
```

## 扩展开发

### 自定义转换规则

在 `novelgen_adapter.go` 中修改推断函数：

```go
// 自定义角色属性推断
func (np *NovelgenProject) inferStatsFromCharacter(char NovelgenCharacter) BaseStats {
    base := BaseStats{
        HP: 100, MP: 50, Attack: 10, Defense: 10,
        Magic: 10, Resistance: 10, Speed: 10, Luck: 10,
    }
    
    // 你的自定义规则
    if char.RoleInStory == "天才" {
        base.Magic += 20
        base.MP += 50
    }
    
    return base
}
```

### 添加新的推演类型

在 `simulation.go` 中添加新的推演方法：

```go
func (se *SimulationEngine) simulateCustomObjective(step SimulationStep, objective QuestObjective) SimulationStep {
    // 你的自定义推演逻辑
    step.Description = "自定义推演完成"
    return step
}
```

## 数据流向

```
novelgen 项目
    │
    ├── characters.json ──┐
    ├── items.json ───────┼──► novelgen_adapter.go ──► RPG World
    ├── locations.json ───┤
    └── outline.json ─────┘
                                    │
                                    ▼
                            simulation.go
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
                战斗推演        对话推演        收集推演
                    │               │               │
                    └───────────────┴───────────────┘
                                    │
                                    ▼
                            rpg_data.json
```

## 应用场景

1. **剧情验证** - 通过推演验证剧情是否合理
2. **数值平衡** - 调整角色属性使战斗更平衡
3. **游戏开发** - 为小说改编游戏提供数值基础
4. **AI创作辅助** - 为AI提供结构化的世界数据
5. **读者互动** - 创建小说相关的游戏化体验

## 注意事项

1. **角色重名** - novelgen 中的重名角色会被合并
2. **属性推断** - 自动推断的属性可能需要手动调整
3. **大纲格式** - 确保 outline.json 格式正确
4. **内存占用** - 大型项目可能占用较多内存

## 故障排除

### 问题：无法加载项目

**解决：**
- 检查项目路径是否正确
- 确认 `books/<book_name>/story/craft/` 目录存在
- 确认 JSON 文件格式正确

### 问题：角色属性不合理

**解决：**
- 修改 `inferStatsFromCharacter()` 函数
- 在 novelgen 中添加更详细的技能描述
- 手动调整生成的 RPG 数据

### 问题：推演失败

**解决：**
- 检查章节ID是否正确
- 确认对应的任务已创建
- 查看错误日志定位问题

## 未来计划

- [ ] 支持更多推演类型（探索、解谜等）
- [ ] 添加可视化推演界面
- [ ] 支持多章节连续推演
- [ ] 添加数值平衡分析工具
- [ ] 导出为游戏引擎格式（Unity、Unreal等）

## 许可证

MIT
