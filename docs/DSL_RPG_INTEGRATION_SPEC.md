# Novelgen DSL-RPG 集成规范

> 版本: 0.2.0  
> 状态: 设计草案  
> 作者: NovelGen Team  
> 最后更新: 2026-04-20

---

## 1. 概述

### 1.1 背景

当前 novelgen 的工作流程：

```
AI 生成小说文本
    ↓
提取结构化数据 (characters.json, items.json, locations.json)
    ↓
转换为 RPG World (novelgen_adapter.go)
    ↓
剧情推演 (simulation.go)
    ↓
导出 rpg_data.json
```

**问题：**
1. 从小说文本到 JSON 的提取依赖启发式规则，容易丢失语义
2. 大纲到 RPG 的转换逻辑分散，难以维护和扩展
3. 缺乏标准化的 Hook/Trigger 机制来驱动剧情
4. 表达式求值能力有限，难以支持复杂条件

### 1.2 目标

引入 **DSL-RPG** 层，让 AI 直接生成可执行的 RPG 配置：

```
AI 生成小说文本
    ↓
AI 生成 RPG-DSL 配置 (story.rpg)
    ↓
DSL 解析器 (dsl/parser.go)
    ↓
DSL 验证器 (dsl/validator.go)
    ↓
DSL → RPG World 转换器 (dsl/converter.go)
    ↓
增强推演引擎 (dsl/hook_enhanced.go + dsl/evaluator.go)
    ↓
导出 rpg_data.json
```

### 1.3 优势

| 特性 | 原方案 | DSL-RPG 方案 |
|------|--------|--------------|
| AI 生成复杂度 | 需理解多个 JSON schema | 统一的 DSL 语法 |
| 语义保留 | 易丢失（启发式提取） | 精确表达（AI 直接写 DSL）|
| 可验证性 | 运行时才发现问题 | 静态验证 + 行号定位 |
| Hook/Trigger | 硬编码 | 声明式配置 |
| 表达式能力 | 基础运算 | 50+ 内置函数 |
| 可调试性 | 困难 | 完整日志系统 |

---

## 2. 架构设计

### 2.1 集成架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Novelgen Pipeline                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌────────────┐  │
│  │   AI Writer  │────▶│ RPG-DSL Gen  │────▶│   story    │  │
│  │   (LLM)      │     │   (LLM)      │     │   .rpg     │  │
│  └──────────────┘     └──────────────┘     └─────┬──────┘  │
│                                                   │         │
│  ┌────────────────────────────────────────────────▼──────┐  │
│  │                    DSL-RPG Engine                      │  │
│  │  ┌─────────┐  ┌───────────┐  ┌───────────┐            │  │
│  │  │ Parser  │─▶│ Validator │─▶│ Converter │            │  │
│  │  │ (Enhanced│  │ (Enhanced│  │           │            │  │
│  │  │  Error) │  │  Error)  │  │           │            │  │
│  │  └─────────┘  └───────────┘  └─────┬─────┘            │  │
│  │                                     │                  │  │
│  │  ┌──────────────────────────────────▼──────────────┐  │  │
│  │  │              RPG World                          │  │  │
│  │  │  Characters, Items, Maps, Quests, Events...    │  │  │
│  │  └──────────────────────────────────┬──────────────┘  │  │
│  │                                     │                  │  │
│  │  ┌──────────────────────────────────▼──────────────┐  │  │
│  │  │           Enhanced Simulation Engine            │  │  │
│  │  │  • Hook System (on_kill, on_damage, etc.)      │  │  │
│  │  │  • Counter & Milestones                        │  │  │
│  │  │  • Expression Evaluator (50+ functions)        │  │  │
│  │  │  • Event Logging                               │  │  │
│  │  └──────────────────────────────────┬──────────────┘  │  │
│  └──────────────────────────────────────│─────────────────┘  │
│                                         │                    │
│  ┌──────────────────────────────────────▼──────────────┐    │
│  │                    Outputs                           │    │
│  │  • rpg_data.json        • simulation_log.json       │    │
│  │  • story_report.md      • balance_analysis.json     │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块职责

| 模块 | 文件 | 职责 |
|------|------|------|
| **Parser** | `dsl/parser_enhanced.go` | 解析 DSL 文件，提供精确错误定位 |
| **Validator** | `dsl/validator.go` | 验证 DSL 语义，引用检查 |
| **Converter** | `dsl/converter.go` | DSL → RPG World 转换 |
| **Hook Manager** | `dsl/hook_enhanced.go` | 事件 Hook、计数器、里程碑 |
| **Evaluator** | `dsl/functions_enhanced.go` | 表达式求值、50+ 函数 |
| **Logger** | `dsl/logger.go` | 执行日志、性能统计 |
| **Adapter** | `internal/rpg/dsl_adapter.go` | Novelgen ↔ DSL-RPG 桥接 |

---

## 3. DSL 文件规范

### 3.1 文件位置

```
books/<book_name>/
├── story/
│   ├── craft/
│   │   ├── characters.json      # 原数据（保留兼容）
│   │   ├── items.json
│   │   └── locations.json
│   ├── compose/
│   │   └── outline.json         # 原大纲（保留兼容）
│   └── rpg/                     # NEW: DSL 配置目录
│       ├── story.rpg            # 主 DSL 文件
│       ├── hooks.rpg            # Hook 配置
│       ├── triggers.rpg         # 触发器配置
│       └── expressions.rpg      # 自定义表达式
```

### 3.2 最小 DSL 示例

```dsl
# books/fire-galaxy/story/rpg/story.rpg

metadata {
  title        = "火银河"
  subtitle     = "从休眠者到星际战士"
  genre        = ["科幻", "废土", "虫族", "进化"]
  power_system = "基因进化"
  tone         = "史诗"
  dsl_version  = "0.2.0"
}

world {
  # 地点定义
  location "锈墙据点" {
    id          = "loc_rustwall"
    type        = "city"
    description = "人类在废墟中建立的临时避难所"
    
    connection "休眠基地" {
      direction = "north"
      distance  = 500
    }
  }
  
  location "休眠基地" {
    id          = "loc_cryo"
    type        = "dungeon"
    description = "三百年前的冬眠设施，虫族巢穴"
  }
  
  # 物品定义
  item "纳米治疗针" {
    id     = "item_nanomed"
    type   = "consumable"
    rarity = "common"
    effect = "restore_hp(30)"
  }
  
  # 规则定义
  rule "腐蚀免疫规则" {
    trigger = "on_damage"
    condition = "damage_type == 'acid' && target.has_trait('腐蚀免疫')"
    effect    = "damage = 0"
  }
}

characters {
  player "陆星眠" {
    id    = "char_luxingmian"
    class = "adaptable_survivor"
    
    stats {
      str = 12
      agi = 15
      int = 14
      vit = 10
      hp  = 100
      mp  = 50
    }
    
    skills = ["快速学习", "适应环境", "腐蚀免疫"]
    
    traits {
      trait "时空穿越者" {
        description = "来自21世纪，对废土世界有特殊认知"
        effects     = ["exp_bonus(10%)"]
      }
    }
  }
  
  enemy "低阶镰虫" {
    id   = "enemy_insect_low"
    type = "insect"
    
    stats {
      str = 10
      agi = 12
      vit = 8
      hp  = 60
    }
    
    drops {
      drop "虫族甲壳" { chance = 30 }
      drop "酸性体液" { chance = 50 }
    }
  }
  
  npc "陈野" {
    id   = "npc_chenye"
    role = "scavenger_captain"
    
    dialogue {
      greeting = "小子，新来的？想活下去就得听我的。"
      
      option "询问生存技巧" {
        response = "第一，别在夜里出门。第二，听到虫鸣立刻趴下。"
      }
      
      option "加入拾荒队" {
        condition = "player.level >= 2"
        response  = "你还太弱，先去清理几只虫子再来找我。"
      }
    }
  }
}

storyline {
  chapter "第一章：苏醒" {
    id = "ch1_awakening"
    
    objective "逃离休眠基地" {
      step 1 "探索环境" {
        description = "检查休眠舱周边"
        event "discover" {
          target = "emergency_gear"
        }
      }
      
      step 2 "遭遇镰虫" {
        description = "与低阶镰虫战斗"
        event "combat" {
          enemies = ["enemy_insect_low"]
          
          on_complete {
            narration = "你击败了第一只虫族，获得了宝贵的经验"
            exp       = 50
            items     = ["虫族甲壳"]
            
            trigger "unlock_ability" {
              ability = "adaptation"
              message = "你的身体开始适应这个危险的世界..."
            }
          }
        }
      }
      
      step 3 "抵达据点" {
        description = "跟随指引前往锈墙据点"
        event "reach" {
          location = "loc_rustwall"
        }
      }
    }
  }
}

systems {
  progression "基因进化系统" {
    id = "sys_gene_evolution"
    
    level 1 "适应者" {
      requirements = "完成第一章"
      bonuses      = ["hp+20", "acid_resist+10"]
    }
    
    level 2 "觉醒者" {
      requirements = "kill_count('enemy_insect') >= 5"
      bonuses      = ["str+2", "agi+2", "unlock_skill('虫族感知')"]
    }
  }
  
  counter "虫族猎人" {
    track       = "insect_kills"
    description = "累计击杀虫族数量"
    
    milestone 5 {
      reward {
        title       = "初级虫族猎人"
        description = "你已经能够应对基础的虫族威胁"
        exp         = 100
      }
    }
    
    milestone 25 {
      reward {
        title       = "虫族克星"
        description = "虫族见到你都会颤抖"
        item        = "chitin_armor"
        exp         = 500
      }
    }
  }
}
```

---

## 4. 集成流程

### 4.1 完整数据流

```
┌────────────────────────────────────────────────────────────────┐
│ Phase 1: 生成阶段                                               │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. AI 生成小说大纲                                            │
│     └── books/fire-galaxy/story/compose/outline.json           │
│                                                                 │
│  2. AI 生成 RPG-DSL 配置                                       │
│     └── books/fire-galaxy/story/rpg/story.rpg                  │
│                                                                 │
│  [可选] AI 生成 Hook/Trigger 配置                              │
│     └── books/fire-galaxy/story/rpg/hooks.rpg                  │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Phase 2: 解析验证阶段                                          │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  3. DSL Parser 解析                                            │
│     ├── 读取 story.rpg                                         │
│     ├── 生成 AST                                               │
│     └── 错误定位（行号/列号）                                  │
│                                                                 │
│  4. DSL Validator 验证                                         │
│     ├── ID 唯一性检查                                          │
│     ├── 引用有效性检查                                         │
│     ├── 类型匹配检查                                           │
│     └── 返回 ValidationReport                                  │
│                                                                 │
│  [如果验证失败]                                                │
│     └── 返回增强错误报告（带源代码上下文）                     │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Phase 3: 转换阶段                                              │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  5. DSL Converter 转换                                         │
│     ├── metadata → World.Context                              │
│     ├── characters → CharacterManager                         │
│     ├── locations → MapManager                                │
│     ├── items → ItemManager                                   │
│     ├── storyline → QuestManager + EventManager               │
│     └── systems → 扩展属性                                    │
│                                                                 │
│  6. 初始化 Hook 系统                                           │
│     ├── 注册 on_kill hooks                                    │
│     ├── 注册 on_damage hooks                                  │
│     └── 初始化 counters                                       │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Phase 4: 推演阶段                                              │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  7. 创建 Simulation Engine                                     │
│     ├── 加载 RPG World                                        │
│     ├── 配置 Hook Manager                                     │
│     └── 配置 Expression Evaluator                             │
│                                                                 │
│  8. 执行推演                                                   │
│     ├── 按章节顺序执行                                        │
│     ├── 记录每个步骤的结果                                    │
│     ├── 触发 Hooks 并记录                                     │
│     └── 更新 Counter 状态                                     │
│                                                                 │
│  9. 生成推演报告                                               │
│     ├── 战斗统计                                              │
│     ├── 角色成长                                              │
│     ├── 事件触发                                              │
│     └── 数值平衡分析                                          │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ Phase 5: 输出阶段                                              │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  10. 导出数据                                                  │
│      ├── rpg_data.json       (完整 RPG 数据)                  │
│      ├── simulation_log.json (推演日志)                       │
│      └── balance_report.md   (平衡性分析)                     │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### 4.2 CLI 命令设计

```bash
# 1. 验证 DSL 文件
novelgen-rpg validate -b fire-galaxy
# 输出:
# ✓ story.rpg: 语法正确
# ✓ hooks.rpg: 语法正确
# ✓ 总计: 15 个角色, 8 个物品, 12 个地点

# 2. 完整转换流程
novelgen-rpg convert -b fire-galaxy -o rpg_data.json
# 输出:
# 1. 解析 DSL 文件... ✓
# 2. 验证语义... ✓
# 3. 转换 RPG World... ✓
# 4. 初始化 Hook 系统... ✓
# 5. 导出数据... ✓
# 输出: rpg_data.json (245 KB)

# 3. 单章节推演（带调试）
novelgen-rpg simulate -b fire-galaxy -c "ch1_awakening" --verbose
# 输出:
# [INFO] 初始化世界... 
# [INFO] 加载 4 个角色
# [HOOK] 注册 on_kill 处理器
# [SIM] 开始推演: 第一章：苏醒
# [STEP] 1. 探索环境
# [EVENT] discover: emergency_gear
# [STEP] 2. 遭遇镰虫
# [COMBAT] 陆星眠 vs 低阶镰虫
# [ACTION] 陆星眠 使用 普通攻击, 造成 15 伤害
# [ACTION] 低阶镰虫 使用 镰刀斩击, 造成 8 伤害
# [RESULT] 战斗胜利! 获得 50 EXP
# [HOOK] 触发: unlock_ability (adaptation)
# [COUNTER] insect_kills: 1/5
# [STEP] 3. 抵达据点
# [RESULT] 推演完成: 3/3 步骤成功

# 4. 完整推演（所有章节）
novelgen-rpg simulate -b fire-galaxy --all --export-log
# 输出: simulation_log.json

# 5. 平衡性分析
novelgen-rpg analyze -b fire-galaxy
# 输出:
# ========== 平衡性分析报告 ==========
# 角色强度分布:
#   - 陆星眠 (Lv.1): 战力 165 [合理]
#   - 低阶镰虫 (Lv.1): 战力 120 [合理]
#   
# 战斗难度:
#   - 第一章: 简单 (胜率 85%) [合理]
#   
# 成长曲线:
#   - 1→2级: 需击杀 5 虫族 [合理]
#   
# 警告:
#   - 第三章 Boss 战力过高 (350 vs 玩家 180)
#   建议: 降低 Boss HP 20%
```

---

## 5. API 设计

### 5.1 Go API

```go
package main

import (
    "novelgen/internal/rpg/dsl"
)

func main() {
    // 1. 创建 DSL-RPG 引擎
    engine := dsl.NewRPGDslEngine(dsl.EngineOptions{
        LogLevel:    dsl.LogLevelInfo,
        LogOutput:   os.Stdout,
        EnableHooks: true,
    })
    
    // 2. 加载并解析 DSL
    result, err := engine.LoadFromFile("books/fire-galaxy/story/rpg/story.rpg")
    if err != nil {
        // 错误包含精确位置信息
        if parseErr, ok := err.(*dsl.DSLParseError); ok {
            fmt.Printf("解析错误 at %s: %s\n", parseErr.Pos, parseErr.Message)
            fmt.Printf("上下文:\n%s\n", parseErr.Context)
        }
        return
    }
    
    // 3. 获取 RPG World
    world := result.World
    
    // 4. 配置 Hook 系统
    engine.RegisterHook("on_kill", dsl.HookConfig{
        Filter: "enemy_type == 'insect'",
        Action: "increment_counter('insect_kills')",
    })
    
    // 5. 执行推演
    simulation := engine.SimulateChapter("ch1_awakening")
    
    // 6. 获取结果
    for _, step := range simulation.Steps {
        fmt.Printf("[%s] %s\n", step.Type, step.Description)
        for _, event := range step.Events {
            fmt.Printf("  -> %s\n", event.Description)
        }
    }
    
    // 7. 导出数据
    engine.Export("rpg_data.json")
}
```

### 5.2 DSL API（AI 使用）

```dsl
# 定义 Hook
hook "虫族击杀记录" {
  event = "on_kill"
  
  condition {
    enemy_type = "insect"
  }
  
  actions {
    action "增加计数器" {
      type   = "increment_counter"
      target = "insect_kills"
    }
    
    action "检查里程碑" {
      type = "check_milestone"
      counter = "insect_kills"
      milestones = [5, 25, 50]
    }
  }
}

# 定义 Trigger
trigger "基因觉醒" {
  condition = "level >= 2 && insect_kills >= 5"
  
  effect {
    unlock_skill = "gene_awakening"
    message      = "你的基因开始觉醒，获得了新的能力！"
  }
}

# 使用表达式
event "难度检查" {
  condition = "player.power > enemy.power * 1.5"
  
  on_true {
    narration = "这场战斗对你来说轻而易举"
    exp_bonus = "10%"
  }
  
  on_false {
    narration = "这是一场势均力敌的战斗"
  }
}
```

---

## 6. 迁移路径

### 6.1 向后兼容

```go
// internal/rpg/dsl_adapter.go

// NovelgenToDSL 将旧版 novelgen 项目转换为 DSL
func NovelgenToDSL(project *NovelgenProject) (*dsl.DSL, error) {
    dslDoc := &dsl.DSL{}
    
    // 1. 转换元数据
    dslDoc.Metadata = &dsl.Metadata{
        Title:       project.BookName,
        DSLVersion:  "0.2.0",
    }
    
    // 2. 转换角色
    for name, char := range project.Characters {
        dslChar := convertCharacterToDSL(char)
        dslDoc.Characters = append(dslDoc.Characters, dslChar)
    }
    
    // 3. 转换地点
    for name, loc := range project.Locations {
        dslLoc := convertLocationToDSL(loc)
        dslDoc.World.Locations = append(dslDoc.World.Locations, dslLoc)
    }
    
    // 4. 转换大纲为 storyline
    dslDoc.Storyline = convertOutlineToStoryline(project.Outline)
    
    return dslDoc, nil
}
```

### 6.2 渐进式迁移

```bash
# Step 1: 现有项目继续使用旧流程
novelgen-rpg convert-old -b mine  # 使用 novelgen_adapter

# Step 2: 生成 DSL 模板（供 AI 参考）
novelgen-rpg export-dsl-template -b mine -o template.rpg

# Step 3: AI 基于模板生成完整 DSL
# 编辑 books/mine/story/rpg/story.rpg

# Step 4: 切换到新流程
novelgen-rpg convert -b mine  # 使用 DSL-RPG

# Step 5: 验证一致性
diff <(novelgen-rpg export-old -b mine) <(novelgen-rpg export-dsl -b mine)
```

---

## 7. 开发计划

### Phase 1: 基础设施（已完成 ✅）

- [x] 增强 Parser（带精确错误定位）
- [x] 增强 Validator（带行号/列号）
- [x] 50+ 内置函数
- [x] 增强 Hook 系统（计数器、里程碑）
- [x] 日志系统

### Phase 2: 集成层（进行中）

- [ ] `internal/rpg/dsl_adapter.go` - Novelgen ↔ DSL 桥接
- [ ] CLI 命令集成
- [ ] 向后兼容层

### Phase 3: AI 工作流（待设计）

- [ ] DSL 生成 Prompt 模板
- [ ] AI 自我验证循环
- [ ] 错误自动修复建议

### Phase 4: 高级功能（未来）

- [ ] 可视化推演界面
- [ ] 实时调试器
- [ ] 多人协作 DSL 编辑

---

## 8. 示例：完整工作流程

### 8.1 创建新小说项目

```bash
# 1. 初始化项目
mkdir -p books/my-novel/story/rpg

# 2. AI 生成大纲（现有流程）
# ... generate outline.json ...

# 3. AI 生成 DSL
# books/my-novel/story/rpg/story.rpg
cat > books/my-novel/story/rpg/story.rpg << 'EOF'
metadata {
  title = "我的小说"
  dsl_version = "0.2.0"
}

characters {
  player "主角" {
    id = "char_hero"
    stats { hp = 100 }
  }
}
EOF

# 4. 验证 DSL
novelgen-rpg validate -b my-novel

# 5. 转换并推演
novelgen-rpg simulate -b my-novel --all

# 6. 查看报告
cat books/my-novel/simulation_report.md
```

### 8.2 调试 DSL 错误

```bash
# 假设 DSL 有语法错误
novelgen-rpg validate -b my-novel
# 输出:
# ✗ story.rpg: 解析错误
#   at line 15, column 8: expected '}', got 'stats'
#   
#   Source context:
#      13 |   player "主角" {
#      14 |     id = "char_hero"
#   >  15 |     stats {
#                  ^
#      16 |       hp = 100
#      17 |     }
#   
#   hint: 确保在嵌套块前使用 '=' 符号
```

---

## 9. 附录

### 9.1 文件清单

```
# 核心 DSL 实现（已完成）
internal/rpg/dsl/
├── ast.go                 # AST 定义
├── parser.go              # 基础解析器
├── parser_enhanced.go     # 增强解析器 ✅
├── validator.go           # 验证器
├── converter.go           # 转换器
├── evaluator.go           # 基础求值器
├── functions_enhanced.go  # 增强函数库 ✅
├── hook.go                # 基础 Hook
├── hook_enhanced.go       # 增强 Hook ✅
├── errors_enhanced.go     # 错误处理 ✅
├── logger.go              # 日志系统 ✅
└── enhanced_test.go       # 测试套件 ✅

# 集成层（待实现）
internal/rpg/
└── dsl_adapter.go         # Novelgen ↔ DSL 桥接

# CLI
cmd/
├── rpg_dsl.go             # DSL 子命令
└── story2rpg/
    └── main.go            # 现有转换工具

# 文档
docs/
├── RPG_DSL_SPEC.md        # DSL 语法规范
├── DSL_RPG_INTEGRATION_SPEC.md  # 本文件
└── NOVELGEN_RPG_INTEGRATION.md  # 旧集成文档
```

### 9.2 相关命令速查

```bash
# DSL 验证
novelgen-rpg validate -b <book>

# DSL → RPG
novelgen-rpg convert -b <book> -o <output.json>

# 单章节推演
novelgen-rpg simulate -b <book> -c <chapter_id>

# 全部推演
novelgen-rpg simulate -b <book> --all --export-log

# 平衡性分析
novelgen-rpg analyze -b <book>

# 导出 DSL 模板
novelgen-rpg export-dsl-template -b <book> -o <template.rpg>

# 调试模式（详细日志）
novelgen-rpg simulate -b <book> -c <chapter> --verbose
```

---

**下一步行动：**

1. Review 本 Spec
2. 确定 Phase 2 优先级
3. 开发 `dsl_adapter.go`
4. 更新 CLI 命令
5. 创建 AI Prompt 模板
