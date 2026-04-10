# RPG Maker 风格数值系统

一个完整的数据化RPG数值系统，类似于RPG Maker，用于AI构建世界和小说创作。

## 系统架构

### 核心系统

1. **角色系统** (`character.go`)
   - 角色模板和实例
   - 属性成长
   - 关系系统
   - 战力计算

2. **职业系统** (`class.go`)
   - 多种职业类型（战士、法师、盗贼等）
   - 技能树
   - 属性修正
   - 进阶路线

3. **技能系统** (`skill.go`)
   - 主动/被动/反应/终极技能
   - 元素克制
   - 命中率/暴击计算
   - 伤害公式

4. **物品系统** (`item.go`)
   - 消耗品、材料、任务物品
   - 堆叠系统
   - 使用效果

5. **装备系统** (`equipment.go`)
   - 武器、护甲、饰品
   - 套装效果
   - 耐久度

6. **地图系统** (`map.go`)
   - 多种地图类型
   - 网格系统
   - 实体管理
   - 随机遭遇

7. **事件系统** (`event.go`) - 核心系统
   - RPG Maker风格的事件命令
   - 条件分支
   - 变量/开关控制
   - 消息显示

8. **任务系统** (`quest.go`)
   - 主线/支线/日常任务
   - 多种目标类型
   - 奖励系统

9. **剧情推演系统** (`simulation.go`)
   - 自动推演小说剧情
   - 战斗模拟
   - 对话模拟
   - 收集模拟
   - 推演报告生成

### 故事适配器 (`story_adapter.go`)

将小说大纲自动转换为RPG数值数据。

## 使用方法

### 1. 基础使用

```go
import "novelgen/internal/rpg"

// 创建游戏世界
world := rpg.CreateExampleWorld()

// 获取玩家
player := world.Player

// 计算战力
bp := world.GetPlayerBattlePower()
fmt.Printf("战力: %d\n", bp.Total)

// 使用技能
result := world.UseSkill("skill_slash", []string{enemy.ID})

// 使用物品
effects := world.UseItem("item_health_potion", player.ID)

// 接取任务
world.AcceptQuest("quest_main_1")
```

### 2. 从小说大纲转换

```go
// 从大纲文件创建故事世界
storyWorld, err := rpg.NewStoryWorld("outline.json")
if err != nil {
    log.Fatal(err)
}

// 获取摘要
summary := storyWorld.GetStorySummary()

// 导出RPG数据
jsonData := storyWorld.ExportToJSON()
```

### 3. 命令行工具

```bash
# 编译工具
go build -o story2rpg.exe ./cmd/story2rpg

# 查看帮助
./story2rpg.exe -h

# 转换大纲
./story2rpg.exe -i outline.json

# 指定输出文件
./story2rpg.exe -i outline.json -o rpg_data.json
```

### 4. 剧情推演

```go
// 创建推演引擎
engine := rpg.NewSimulationEngine(world)

// 推演单个章节
result, err := engine.SimulateChapter("P1-V1-C1")
if err != nil {
    log.Fatal(err)
}

// 打印推演过程
for _, step := range result.Steps {
    fmt.Printf("[%s] %s\n", step.Type, step.Description)
    for _, res := range step.Results {
        fmt.Printf("  - %s\n", res.Message)
    }
}

// 生成推演报告
report := engine.GetSimulationReport()
fmt.Println(report)

// 导出推演数据
jsonData := engine.ExportSimulation()
```

### 5. 推演命令行工具

```bash
# 编译工具
go build -o simulate.exe ./cmd/simulate

# 查看帮助
./simulate.exe -h

# 推演所有章节
./simulate.exe -i outline.json

# 推演单个章节
./simulate.exe -i outline.json -c P1-V1-C1

# 指定输出文件
./simulate.exe -i outline.json -c P1-V1-C1 -o result.json
```

## 大纲JSON格式

```json
{
  "parts": [
    {
      "id": "P1",
      "title": "第一部分标题",
      "summary": "部分简介",
      "volumes": [
        {
          "id": "P1-V1",
          "title": "卷一标题",
          "summary": "卷简介",
          "chapters": [
            {
              "id": "P1-V1-C1",
              "title": "章节标题",
              "summary": "章节简介",
              "characters": ["角色1", "角色2"],
              "location": "地点名称",
              "events": [
                {
                  "type": "premise",
                  "characters": ["角色1"],
                  "subject": "角色1",
                  "change": "发生的变化",
                  "details": "详细描述"
                }
              ],
              "beats": [
                "情节节拍1",
                "情节节拍2"
              ],
              "opening_beat": "开场节拍",
              "closing_beat": "结束节拍",
              "state_change": "状态变化",
              "conflict": "冲突",
              "pacing": "fast"
            }
          ]
        }
      ]
    }
  ]
}
```

## 推演规则

### 战斗推演
- 回合制战斗系统
- 玩家AI：优先使用技能，70%概率普通攻击
- 敌人AI：80%概率攻击，20%概率使用技能
- 最大回合数：20回合
- 伤害计算：攻击力 - 防御力/2

### 对话推演
- 触发NPC对话事件
- 记录对话结果

### 收集推演
- 自动添加物品到背包
- 记录收集数量和类型

### 移动推演
- 自动移动玩家到目标地点
- 记录位置变化

## 转换规则

### 角色转换
- 根据角色名称推断属性（主角、敌人、NPC）
- 自动分配基础属性和成长属性
- 主角自动设置为玩家角色

### 地点转换
- 根据地点名称推断地图类型
- 矿/洞/穴 -> 洞穴
- 城/镇/村 -> 城镇
- 林/森 -> 森林
- 山/峰 -> 山脉

### 任务转换
- 每个章节转换为一个任务
- 根据章节ID判断任务类型（主线/支线）
- 从事件推断任务目标

### 事件转换
- 章节的情节节拍转换为事件命令
- 每个节拍对应一个显示文本命令

## 文件结构

```
internal/rpg/
├── types.go              # 基础类型
├── character.go          # 角色系统
├── class.go              # 职业系统
├── skill.go              # 技能系统
├── item.go               # 物品系统
├── equipment.go          # 装备系统
├── map.go                # 地图系统
├── event.go              # 事件系统
├── quest.go              # 任务系统
├── world.go              # 世界管理器
├── examples.go           # 示例数据
├── story_adapter.go      # 故事适配器
├── simulation.go         # 剧情推演引擎
├── rpg_test.go           # 基础测试
├── story_adapter_test.go # 适配器测试
└── simulation_test.go    # 推演测试

cmd/story2rpg/
└── main.go               # 大纲转RPG工具

cmd/simulate/
└── main.go               # 剧情推演工具
```

## 扩展开发

### 添加新职业

```go
mage := &rpg.Class{
    ID:   "class_mage",
    Name: "法师",
    Type: rpg.ClassTypeMage,
    BaseStats: rpg.BaseStats{
        HP: 70, MP: 100, Attack: 5, Magic: 18,
    },
    // ...
}
world.Classes.AddClass(mage)
```

### 添加新技能

```go
fireball := &rpg.Skill{
    ID:   "skill_fireball",
    Name: "火球术",
    Type: rpg.SkillTypeActive,
    Element: rpg.ElementFire,
    Damage: &rpg.SkillDamage{
        Type: rpg.DamageTypeMagic,
        Power: 30,
    },
}
world.Skills.AddSkill(fireball)
```

### 自定义转换规则

在 `story_adapter.go` 中修改推断函数：
- `inferCharacterStats()` - 角色属性推断
- `inferMapType()` - 地图类型推断
- `inferQuestType()` - 任务类型推断

## 许可证

MIT
