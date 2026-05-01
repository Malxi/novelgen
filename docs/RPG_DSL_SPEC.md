# RPG-DSL 规格文档

> 版本: 0.1.0 (Draft)  
> 作者: NovelGen Team  
> 最后更新: 2026-04-20

---

## 1. 概述

### 1.1 什么是 RPG-DSL

RPG-DSL（Role-Playing Game Domain-Specific Language）是专为小说 RPG 推演设计的领域特定语言。它让 AI 能够生成结构化、可执行的游戏指令，而不是模糊的自然语言描述。

### 1.2 设计目标

- **AI 友好**: 类似自然语言的结构，降低 AI 生成难度
- **精确可执行**: 每个指令都有明确的游戏语义
- **类型安全**: 严格的语法规范，避免歧义
- **可验证**: 支持静态检查和运行时验证

### 1.3 与现有方案对比

| 特性 | 旧 JSON Schema | RPG-DSL |
|------|---------------|---------|
| AI 理解成本 | 高（需理解复杂 schema） | 低（声明式指令） |
| 解析复杂度 | 高（多字段推断） | 低（语法明确） |
| 错误检测 | 运行时才发现 | 静态检查即可发现 |
| 可读性 | 差（JSON 嵌套） | 好（类似剧本） |
| 扩展性 | 修改 schema | 添加新指令 |

---

## 2. 语法规范

### 2.1 基本结构

RPG-DSL 采用类 HCL（HashiCorp Configuration Language）语法：

```dsl
# 这是注释
block_type "identifier" {
  key = value
  nested_block {
    # ...
  }
}
```

### 2.2 顶层块（Top-Level Blocks）

```dsl
# 1. 元数据块 - 定义故事基本信息
metadata {
  title       = "逐星"
  genre       = ["科幻", "废土求生", "虫族"]
  power_system = "基因进化"
  dsl_version  = "0.1.0"
}

# 2. 世界块 - 定义游戏世界
world {
  # 地点、物品、规则等
}

# 3. 角色块 - 定义所有角色
characters {
  # 玩家、NPC、敌人等
}

# 4. 剧情块 - 定义故事流程
storyline {
  # 章节、事件等
}

# 5. 系统块 - 定义游戏机制
systems {
  # 升级、突破、技能等
}
```

---

## 3. 详细块定义

### 3.1 metadata 块

```dsl
metadata {
  title        = "逐星"                    # 故事标题
  subtitle     = "从废土到星空"             # 副标题（可选）
  genre        = ["科幻", "废土", "虫族"]   # 题材标签
  power_system = "基因进化"                # 力量体系
  tone         = "史诗"                    # 基调
  dsl_version  = "0.1.0"                  # DSL 版本
}
```

### 3.2 world 块

```dsl
world {
  # 3.2.1 地点定义
  location "休眠基地" {
    id          = "loc_cryo_base"
    type        = "indoor"           # indoor | outdoor | dungeon | city
    description = "废弃的休眠设施，到处都是锈迹..."
    
    # 连接的其他地点
    connections = [
      { to = "loc_wasteland", direction = "出口", condition = "none" },
      { to = "loc_underground", direction = "电梯", condition = "keycard" }
    ]
    
    # 地点特性
    properties {
      temperature = "cold"           # 环境影响
      danger_level = 2               # 危险等级 1-10
      resources = ["metal", "energy"]
    }
    
    # 地点事件
    on_enter {
      action = "trigger_event"
      event  = "check_temperature"
    }
  }
  
  # 3.2.2 物品定义
  item "虫族晶核" {
    id     = "item_crystal"
    type   = "material"              # material | equipment | consumable | quest
    rarity = "common"                # common | uncommon | rare | epic | legendary
    
    # 物品效果
    effects {
      currency = { type = "crystal", value = 10 }
    }
    
    # 获取方式
    sources {
      drop_from = ["enemy_wasp", "enemy_beetle"]
      drop_rate = 0.8
    }
  }
  
  # 3.2.3 环境规则
  rule "低温环境" {
    trigger = "location_temperature == cold"
    effect  = "player.hp_regen *= 0.5"
  }
}
```

### 3.3 characters 块

```dsl
characters {
  # 3.3.1 玩家角色
  player "陆沉" {
    id    = "char_player"
    class = "engineer"               # 职业
    
    # 基础属性
    stats {
      str = 10    # 力量
      agi = 12    # 敏捷
      int = 15    # 智力
      vit = 10    # 体质
      hp  = 100   # 生命值
      mp  = 50    # 能量值
    }
    
    # 初始装备
    equipment {
      weapon = "item_survival_knife"
      armor  = "item_tattered_clothes"
    }
    
    # 初始技能
    skills = ["skill_analyze", "skill_repair"]
    
    # 初始物品
    inventory {
      "item_ration" = 3
      "item_water"  = 2
    }
    
    # 特殊能力（伏笔）
    traits {
      "regeneration" = {
        unlocked = false
        trigger  = "on_first_damage"
      }
    }
  }
  
  # 3.3.2 敌人定义
  enemy "虫族工蜂" {
    id   = "enemy_wasp"
    type = "insect"                  # 种族类型
    
    # 等级模板
    template {
      base_level = 1
      hp_formula = "50 + level * 10"
      stats_per_level {
        str = 2
        agi = 3
      }
    }
    
    # 战斗行为
    behavior {
      ai_type = "aggressive"
      preferred_target = "nearest"
      skills = ["skill_claw", "skill_acid_spray"]
    }
    
    # 掉落物
    drops {
      fixed = [
        { item = "item_crystal", min = 1, max = 1 }
      ]
      random = [
        { item = "item_chitin", chance = 0.3, min = 1, max = 2 }
      ]
    }
    
    # 出现地点
    spawn_locations = ["loc_wasteland", "loc_ruins"]
    spawn_weight    = 10
  }
  
  # 3.3.3 NPC 定义
  npc "老周" {
    id   = "npc_old_zhou"
    role = "merchant"                # merchant | quest_giver | trainer | info
    
    default_location = "loc_settlement"
    
    # NPC 行为
    services {
      trade {
        buy_price_modifier  = 0.7
        sell_price_modifier = 1.3
        accepts_items = ["item_crystal", "item_material"]
      }
      quest {
        provides = ["quest_collect_crystals"]
      }
    }
    
    # 对话树
    dialogue {
      greeting = "年轻人，想要点什么？"
      
      options {
        "我想买东西"  -> action "open_shop"
        "有什么任务吗" -> check_quest "quest_collect_crystals"
        "这是什么地方" -> dialogue "explain_settlement"
      }
    }
  }
}
```

### 3.4 storyline 块

```dsl
storyline {
  # 3.4.1 故事线（卷/部）
  arc "废土苏醒" {
    id       = "arc_awakening"
    position = 1
    
    # 包含的章节
    chapters = ["chap_001", "chap_002", "chap_003", "chap_004"]
    
    # 解锁条件
    unlock_condition = "always"
    
    # 完成奖励
    completion_reward {
      exp    = 1000
      title  = "废土生存者"
      unlock = ["arc_starship"]
    }
  }
  
  # 3.4.2 章节定义
  chapter "冷舱梦醒" {
    id       = "chap_001"
    arc      = "arc_awakening"
    position = 1
    
    # 章节目标（按顺序执行）
    objective "逃离休眠基地" {
      id   = "obj_escape"
      type = "sequence"              # sequence | parallel | optional
      
      # 目标步骤
      step 1 {
        description = "从休眠仓中醒来"
        
        event {
          type   = "spawn"
          actor  = "char_player"
          location = "loc_cryo_room"
          
          on_complete {
            narration = "你醒来时，休眠仓的指示灯闪烁着诡异的红光..."
          }
        }
      }
      
      step 2 {
        description = "检查周围环境"
        
        event {
          type = "explore"
          location = "loc_cryo_room"
          
          discoveries {
            "broken_terminal" = {
              type = "info"
              text = "终端显示日期：公元3024年..."
            }
            "emergency_pack" = {
              type = "item"
              item = "item_survival_knife"
              quantity = 1
            }
          }
        }
      }
      
      step 3 {
        description = "遭遇虫族工蜂"
        
        # 触发条件
        trigger {
          type     = "enter_location"
          location = "loc_entrance"
        }
        
        event {
          type = "combat"
          
          # 战斗设置
          setup {
            location = "loc_corridor"
            
            enemies = [
              { id = "enemy_wasp", count = 1, level = 1, elite = false }
            ]
            
            environment {
              narrow_space = true    # 影响战斗：闪避降低
              poor_lighting = true   # 影响战斗：命中率降低
            }
          }
          
          # 战斗流程
          phases {
            phase 1 {
              name = "遭遇"
              trigger = "combat_start"
              narration = "黑暗中传来窸窣声，一只虫族工蜂突然扑出！"
            }
            
            phase 2 {
              name = "受伤"
              trigger = "player_hp < 50%"
              narration = "你的手臂被腐蚀液擦中，剧痛袭来..."
              
              on_trigger {
                unlock_trait "regeneration"
                narration = "但伤口竟然开始缓慢愈合！"
              }
            }
          }
          
          # 战斗结果
          on_victory {
            narration = "工蜂倒在地上，身体逐渐僵硬..."
            
            rewards {
              exp   = 100
              items = [
                { id = "item_crystal", guaranteed = true }
              ]
            }
          }
          
          on_defeat {
            narration = "眼前一黑，你失去了意识..."
            result = "game_over"         # 或 "retry"
          }
        }
      }
      
      step 4 {
        description = "离开基地"
        
        event {
          type   = "move"
          actor  = "char_player"
          to     = "loc_wasteland"
          
          on_complete {
            narration = "终于，你来到了地面。废土的风沙扑面而来..."
            trigger_chapter = "chap_002"
          }
        }
      }
    }
    
    # 章节完成奖励
    completion {
      exp      = 150
      items    = []
      unlocks  = ["loc_wasteland"]
      story_flags = ["awakened", "first_kill"]
    }
  }
}
```

### 3.5 systems 块

```dsl
systems {
  # 3.5.1 等级系统
  progression "exp_level" {
    type = "experience_based"
    
    # 经验公式
    formula {
      exp_to_next = "level * 100"
      exp_from_enemy = "enemy_level * 10"
      exp_from_quest = "quest_difficulty * 50"
    }
    
    # 升级奖励
    level_up {
      stat_points = 5
      skill_points = 1
      hp_restore = "max_hp"
      mp_restore = "max_mp"
    }
  }
  
  # 3.5.2 突破系统
  breakthrough "基因进化" {
    id = "sys_gene_evolution"
    type = "stage_based"
    
    stage "普通人" {
      max_level = 10
      attribute_multiplier = 1.0
    }
    
    stage "觉醒者" {
      max_level = 30
      requirement {
        level = 10
        quest = "quest_awakening"
        item  = "item_awakening_potion"
      }
      attribute_multiplier = 1.5
      unlock_skills = ["skill_gene_release"]
      special_ability = "regeneration_enhanced"
    }
    
    stage "进化者" {
      max_level = 50
      requirement {
        level = 30
        item  = "item_evolution_core"
        achievement = "kill_elite_10"
      }
      attribute_multiplier = 2.0
    }
  }
  
  # 3.5.3 技能系统
  skill_system {
    # 技能学习
    learning {
      method = "level_and_quest"       # level_only | quest_only | level_and_quest
      max_skills = 6
    }
    
    # 技能升级
    upgrade {
      max_level = 5
      cost_formula = "current_level * 100"
    }
  }
  
  # 3.5.4 Hook & Tracker 系统（MVP Phase 2）
  hooks {
    # 技能使用统计
    hook "on_skill_use" {
      id = "hook_skill_tracker"
      
      # 条件过滤
      condition = "skill_type == 'combat'"
      
      # 计数器
      counter "combat_skill_uses" {
        max = 100
        
        # 里程碑奖励
        on_milestone 10 {
          reward { title = "技能初学者" }
        }
        on_milestone 50 {
          reward { title = "技能熟练者", unlock_skill = "skill_combo" }
        }
        on_milestone 100 {
          reward { title = "技能大师", stat_bonus = { agi = 5 } }
        }
      }
    }
    
    # 击杀统计
    hook "on_kill" {
      id = "hook_kill_tracker"
      
      counter "wasp_killed" {
        filter = "enemy_id == 'enemy_wasp'"
        
        on_milestone 10 {
          reward { item = "item_wasp_slayer_badge" }
          set_flag = "wasp_slayer"
        }
      }
    }
    
    # 承伤记录（觉醒触发器）
    hook "on_damage_taken" {
      id = "hook_damage_tracker"
      
      # 记录濒死体验
      counter "near_death_experiences" {
        filter = "damage >= max_hp * 0.5"
        max = 10
      }
    }
  }
  
  # 3.5.5 Trigger 触发器系统（MVP Phase 2）
  triggers {
    # 觉醒契机触发器
    trigger "觉醒契机" {
      id = "trigger_awakening"
      
      # 触发条件（AND 关系）
      condition {
        and = [
          { stat = "hp", op = "<", value = 20 },
          { flag = "in_combat", value = true },
          { counter = "near_death_experiences", op = ">=", value = 3 }
        ]
      }
      
      # 触发效果
      on_trigger {
        unlock_trait "regeneration"
        narration = "生死之间，你感受到体内某种力量在苏醒..."
        trigger_quest "quest_awakening"
        set_flag "awakened"
      }
      
      # 只触发一次
      once = true
    }
    
    # 隐藏任务触发器
    trigger "神秘商人" {
      id = "trigger_merchant"
      
      condition {
        and = [
          { location = "loc_wasteland" },
          { time = "night" },
          { random = 0.3 }  # 30% 概率
        ]
      }
      
      on_trigger {
        spawn_npc "npc_mysterious_merchant"
        narration = "风沙中突然出现一个身影..."
      }
    }
  }
}
```

---

## 4. 内置函数（MVP Phase 2）

### 4.1 数学函数

```dsl
stats {
  # random(min, max) - 随机整数
  damage = random(10, 20) + str * 2
  
  # random_float(min, max) - 随机浮点数
  crit_chance = random_float(0.1, 0.3)
  
  # min(a, b) / max(a, b) - 最值
  actual_damage = max(1, base_damage - defense)
  
  # clamp(val, min, max) - 限制范围
  hp_percentage = clamp(hp / max_hp * 100, 0, 100)
  
  # round(val) / floor(val) / ceil(val) - 取整
  exp_gain = round(base_exp * bonus_multiplier)
}
```

### 4.2 逻辑函数

```dsl
trigger {
  # and(...) - 逻辑与
  condition = and(
    level >= 10,
    has_flag("awakened")
  )
  
  # or(...) - 逻辑或
  condition = or(
    has_item("item_crystal"),
    has_item("item_gem"),
    kill_count("enemy_wasp") >= 10
  )
  
  # not(x) - 逻辑非
  condition = not(has_flag("completed_chapter_1"))
}
```

### 4.3 查询函数

```dsl
# has_flag(name) - 检查标志
condition = has_flag("first_blood")

# has_item(id) - 检查物品
condition = has_item("item_keycard")
item_count = item_quantity("item_crystal")

# kill_count(enemy_id) - 击杀统计
condition = kill_count("enemy_wasp") >= 10

# skill_use_count(skill_id) - 技能使用统计
condition = skill_use_count("skill_fireball") >= 50

# current_location() - 当前位置
condition = current_location() == "loc_wasteland"

# current_time() - 当前时间（游戏内）
condition = current_time() == "night"

# get_stat(target, stat) - 获取属性
condition = get_stat("char_player", "hp") < 20
```

### 4.4 随机与选择函数

```dsl
# random_choice(list) - 随机选择一项
event {
  narration = random_choice([
    "你感到一阵寒意...",
    "黑暗中似乎有东西在移动...",
    "周围突然安静了下来..."
  ])
}

# weighted_choice(choices) - 按权重选择
drop {
  item = weighted_choice({
    "item_common" = 50,
    "item_uncommon" = 30,
    "item_rare" = 15,
    "item_epic" = 5
  })
}

# shuffle(list) - 打乱顺序
spawn_points = shuffle(["point_a", "point_b", "point_c"])

# pick_n(list, n) - 随机选择 N 项
ambush_enemies = pick_n(["enemy_1", "enemy_2", "enemy_3", "enemy_4"], 2)
```

### 4.5 字符串函数

```dsl
# format(template, ...) - 格式化字符串
message = format("{} 对 {} 造成了 {} 点伤害!", attacker, target, damage)

# concat(...) - 连接字符串
full_name = concat(first_name, " ", last_name)
```

### 4.6 函数使用示例

```dsl
# 动态难度调整
enemy "虫族工蜂" {
  id = "enemy_wasp"
  
  template {
    # 根据玩家等级动态调整
    level = max(1, player_level - 1 + random(0, 2))
    hp = "50 + level * 10"
    
    # 精英怪概率
    is_elite = random_float(0, 1) < 0.1  # 10% 精英
    
    stats {
      str = "8 + level * 2 + (is_elite ? 5 : 0)"
      agi = "10 + level * 3"
    }
  }
}

# 复杂触发条件
trigger "绝境突破" {
  id = "trigger_breakthrough"
  
  condition = and(
    get_stat("char_player", "hp") < max_hp * 0.2,
    has_flag("in_combat"),
    or(
      kill_count("enemy_elite") >= 1,
      skill_use_count("skill_ultimate") >= 3
    ),
    not(has_flag("already_broken_through"))
  )
  
  on_trigger {
    unlock_stage "觉醒者"
    narration = "在生死边缘，你突破了极限！"
    heal = max_hp * 0.5  # 恢复50%生命
  }
}
```

---

## 5. 事件类型详细定义

### 5.1 事件通用结构

```dsl
event {
  type = "EVENT_TYPE"                # 见下方列表
  
  # 触发条件（可选）
  trigger {
    type = "auto" | "manual" | "enter_location" | "interact" | "condition"
    condition = "expression"         # 条件表达式
  }
  
  # 前置条件（可选）
  require {
    flags = ["flag_name"]
    items = ["item_id"]
    stats = { level = 5 }
  }
  
  # 成功结果
  on_complete {
    # ...
  }
  
  # 失败结果
  on_fail {
    # ...
  }
}
```

事件可以包含一个或多个 `state_delta` 块，用于把章节文本中的状态变化转成模拟器可折叠的结构化事实。字段均为可选，但生产方应尽量提供 `target`、`kind`、`field`、`to`，数值类状态同时提供 `delta` 与 `unit`。

```dsl
state_delta {
  target = "protagonist"
  kind   = "gene"                    # gene | mech | cultivation | item | injury | resource | plot_thread ...
  field  = "stability"               # level | stability | form | energy | module | damage ...
  to     = "65"
  delta  = 65
  unit   = "%"
  note   = "structured gene stability from chapter state anchor"
}
```

Outline 转 DSL 时，`state_anchor.cultivation` 和 `state_anchor.key_items` 会额外派生数值化战斗状态：

- `gene.level`：如 `二级基因适配者` -> `to = "2"`。
- `gene.stability`：如 `基因稳定性65%` -> `to = "65"`, `unit = "%"`.
- `mech.form` / `mech.level`：如 `基础版火种机甲` -> 机甲形态和推断等级。
- `mech.energy`：如 `能量40%` -> `to = "40"`, `unit = "%"`.
- `mech.module` / `mech.module_blueprint`：如 `远程模块可用`、`近战模块蓝图`。
- `mech.damage`：如 `左腿护甲受损`；修复完成可写为 `to = "none"`。

### 4.2 事件类型列表

| 类型 | 说明 | 关键字段 |
|------|------|----------|
| `spawn` | 生成角色/物品 | actor, location, state |
| `move` | 移动 | actor, from, to, method |
| `combat` | 战斗 | enemies, allies, environment |
| `dialogue` | 对话 | speaker, text, choices |
| `acquire` | 获得物品 | actor, item, quantity, source |
| `use` | 使用物品 | actor, item, target |
| `discover` | 发现 | actor, discovery, description |
| `learn` | 学习 | actor, skill, source |
| `upgrade` | 升级 | target, stat/skill, amount |
| `breakthrough` | 突破 | actor, stage |
| `trigger_event` | 触发其他事件 | event_id |
| `modify_flag` | 修改标志 | flag, operation |
| `modify_stat` | 修改属性 | target, stat, operation |
| `change_location` | 改变地点 | location, state |
| `spawn_enemy` | 生成敌人 | enemy, count, location |
| `timer` | 计时器 | duration, on_expire |

### 4.3 复杂事件示例

```dsl
# 多阶段战斗事件
event {
  type = "combat"
  
  setup {
    location = "loc_boss_arena"
    
    enemies = [
      { id = "enemy_queen", count = 1, level = 10, elite = true, boss = true },
      { id = "enemy_guard", count = 3, level = 5 }
    ]
    
    environment {
      hazard = "acid_pool"
      cover = ["pillar_1", "pillar_2"]
    }
  }
  
  # 战斗阶段
  phases {
    phase 1 {
      trigger = "combat_start"
      duration = "3_turns"
      
      modifiers {
        enemy_attack *= 1.5
        player_defense *= 0.8
      }
      
      narration = "虫后进入狂暴状态！"
    }
    
    phase 2 {
      trigger = "enemy_queen_hp < 50%"
      
      on_trigger {
        spawn_enemy { id = "enemy_wasp", count = 2 }
        narration = "虫后发出尖啸，更多工蜂涌入战场！"
      }
    }
  }
  
  # 胜利结果
  on_victory {
    narration = "虫后轰然倒地，整个巢穴都在颤抖..."
    
    rewards {
      exp = 1000
      items = [
        { id = "item_queen_core", guaranteed = true },
        { id = "item_epic_material", chance = 0.5 }
      ]
      unlock_locations = ["loc_nest_depths"]
      story_flags = ["queen_defeated"]
    }
    
    trigger_event = "evt_nest_collapse"
  }
}
```

---

## 6. 执行计划（分阶段实施）

### 哲学：先做能用，再做好用

```
MVP (Minimum Viable Product) → Core → Enhanced → Advanced
```

---

### Phase 1: MVP - 核心骨架（2 周）

**目标**：让 AI 能生成 DSL，系统能解析并执行最简单的故事

#### Week 1: DSL Parser & Validator

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 1.1 定义 AST 结构 | `internal/rpg/dsl/ast.go` | P0 | 只包含 MVP 必需的块 |
| 1.2 实现 Parser | `internal/rpg/dsl/parser.go` | P0 | 支持基础语法 |
| 1.3 实现 Validator | `internal/rpg/dsl/validator.go` | P0 | 基础验证 |
| 1.4 单元测试 | `*_test.go` | P0 | 覆盖率 > 80% |

**MVP 包含的块：**
- `metadata` - 基本信息
- `world.location` - 地点（简化版）
- `characters.player` - 主角
- `characters.enemy` - 敌人（简化版）
- `storyline.chapter` - 章节
- `storyline.objective` - 目标
- `event` - 事件（5 种类型）

**MVP 包含的事件类型：**
1. `spawn` - 生成角色
2. `move` - 移动
3. `combat` - 战斗（基础版）
4. `dialogue` - 对话
5. `acquire` - 获得物品

#### Week 2: Converter & CLI

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 2.1 DSL → World 转换器 | `internal/rpg/dsl/converter.go` | P0 | 对接现有 RPG 系统 |
| 2.2 CLI 命令 | `cmd/rpg_dsl.go` | P0 | `novelgen rpg dsl import` |
| 2.3 AI Skill | `internal/agents/skills/compose-dsl/SKILL.md` | P0 | 简化版 Skill |
| 2.4 端到端测试 | `tests/dsl_integration_test.go` | P0 | 完整流程测试 |

**MVP 成功标准：**
- [ ] AI 能生成可解析的 DSL
- [ ] 解析后能正确创建 RPG World
- [ ] 能运行基础模拟（战斗、移动、对话）
- [ ] 单元测试全部通过

---

### Phase 2: Core - 核心功能（2 周）

**目标**：添加 Hook、Trigger、函数，让故事更有动态性

#### Week 3: Hook & Trigger 系统

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 3.1 Hook 系统 | `internal/rpg/dsl/hook.go` | P1 | on_kill, on_damage 等 |
| 3.2 Counter/Tracker | `internal/rpg/dsl/tracker.go` | P1 | 统计系统 |
| 3.3 Trigger 系统 | `internal/rpg/dsl/trigger.go` | P1 | 条件触发器 |
| 3.4 条件表达式 | `internal/rpg/dsl/expression.go` | P1 | 基础条件解析 |

**支持的 Hook：**
```dsl
hook "on_kill" { }
hook "on_damage_taken" { }
hook "on_skill_use" { }
hook "on_level_up" { }
hook "on_location_enter" { }
```

#### Week 4: 内置函数

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 4.1 函数注册系统 | `internal/rpg/dsl/functions/registry.go` | P1 | 函数管理 |
| 4.2 数学函数 | `internal/rpg/dsl/functions/math.go` | P1 | random, min, max |
| 4.3 逻辑函数 | `internal/rpg/dsl/functions/logic.go` | P1 | and, or, not |
| 4.4 查询函数 | `internal/rpg/dsl/functions/query.go` | P1 | has_flag, kill_count |
| 4.5 表达式求值 | `internal/rpg/dsl/evaluator.go` | P1 | 表达式解析 |

**Core 阶段成功标准：**
- [ ] 支持 5+ 种 Hook
- [ ] 支持 10+ 种内置函数
- [ ] Trigger 能正确触发
- [ ] AI 能使用函数生成动态内容

---

### Phase 3: Enhanced - 增强功能（2 周）

**目标**：并行事件、模板系统、复杂战斗

#### Week 5: 高级事件 & 战斗

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 5.1 并行事件 | `internal/rpg/dsl/parallel.go` | P2 | parallel objective |
| 5.2 战斗阶段 | `internal/rpg/dsl/combat_phase.go` | P2 | 多阶段战斗 |
| 5.3 环境效果 | `internal/rpg/dsl/environment.go` | P2 | 地形、天气影响 |
| 5.4 NPC AI | `internal/rpg/dsl/npc_ai.go` | P2 | NPC 行为树 |

#### Week 6: 模板 & 复用

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 6.1 模板系统 | `internal/rpg/dsl/template.go` | P2 | 敌人类型模板 |
| 6.2 模块化 | `internal/rpg/dsl/module.go` | P2 | import 支持 |
| 6.3 库文件 | `dsl_libs/` | P2 | 预设模板库 |
| 6.4 AI 优化 | `internal/agents/skills/compose-dsl/` | P2 | 进阶 Skill |

**Enhanced 成功标准：**
- [ ] 支持并行事件
- [ ] 支持战斗阶段
- [ ] 支持模板复用
- [ ] 有 3+ 个预设模板库

---

### Phase 4: Advanced - 高级功能（可选，2 周）

**目标**：可视化、调试、高级 AI

#### Week 7-8: 工具 & 生态

| 任务 | 文件 | 优先级 | 说明 |
|------|------|--------|------|
| 7.1 可视化编辑器 | `webui/dsl_editor/` | P3 | 拖拽式编辑 |
| 7.2 DSL 调试器 | `cmd/rpg_dsl_debug.go` | P3 | 单步执行 |
| 7.3 性能分析 | `internal/rpg/dsl/profiler.go` | P3 | 性能优化 |
| 7.4 文档生成 | `cmd/rpg_dsl_doc.go` | P3 | 自动生成文档 |

**Advanced 成功标准：**
- [ ] 有可视化编辑器
- [ ] 能单步调试 DSL
- [ ] AI 能生成复杂 DSL
- [ ] 完整的开发者文档

---

### 实施建议

#### 立即开始（Phase 1）

如果要现在就开始，建议 **简化 DSL**：

```dsl
# 最简 MVP DSL
metadata {
  title = "逐星"
}

characters {
  player "陆沉" {
    stats { hp = 100, str = 10 }
  }
  
  enemy "虫族工蜂" {
    stats { hp = 50, str = 8 }
    drops { item = "item_crystal" }
  }
}

world {
  location "休眠基地" {
    id = "loc_base"
  }
}

storyline {
  chapter "冷舱梦醒" {
    objective "逃离" {
      step 1 {
        event {
          type = "combat"
          enemies = [{ id = "enemy_wasp", count = 1 }]
          on_victory {
            narration = "你战胜了工蜂！"
            exp = 100
          }
        }
      }
    }
  }
}
```

**验证流程：**
1. AI 生成最简 DSL
2. Parser 解析 AST
3. Validator 检查
4. Converter 生成 RPG World
5. Simulator 执行
6. 输出结果

---

### 决策点

在实施过程中需要决策：

| 决策 | 建议 | 理由 |
|------|------|------|
| 语法风格 | HCL-like | 清晰、AI友好 |
| 是否并行 | Phase 3 再考虑 | MVP 不需要 |
| 条件表达式 | Phase 2 添加 | 让 Trigger 可用 |
| 内置函数 | Phase 2 添加 | 增强动态性 |
| 可视化 | Phase 4 考虑 | 先保证核心功能 |

---

## 6. 文件结构

```
internal/rpg/dsl/
├── ast.go              # AST 结构定义
├── parser.go           # DSL 解析器
├── parser_test.go      # 解析器测试
├── validator.go        # DSL 验证器
├── validator_test.go   # 验证器测试
├── converter.go        # DSL -> RPG World 转换器
├── converter_test.go   # 转换器测试
├── formatter.go        # DSL 格式化工具
├── errors.go           # 错误定义
└── examples/           # 示例 DSL 文件
    ├── simple_combat.dsl
    ├── chapter_001.dsl
    └── full_story.dsl

internal/agents/skills/compose-dsl/
└── SKILL.md            # AI DSL 生成技能

cmd/
├── rpg_dsl.go          # DSL 相关命令

books/fire-galaxy/story/dsl/
├── story.dsl           # 生成的 DSL 文件
└── imported/           # 导入的库文件
```

---

## 7. 示例：完整章节 DSL

```dsl
metadata {
  title = "逐星 - 第一章"
  dsl_version = "0.1.0"
}

characters {
  player "陆沉" {
    id = "char_player"
    class = "engineer"
    stats { str = 10, agi = 12, int = 15, vit = 10 }
  }
  
  enemy "虫族工蜂" {
    id = "enemy_wasp"
    template {
      base_level = 1
      hp_formula = "50 + level * 10"
    }
    drops {
      fixed = [{ item = "item_crystal", min = 1, max = 1 }]
    }
  }
}

world {
  location "休眠舱" {
    id = "loc_cryo_room"
    type = "indoor"
  }
  
  location "走廊" {
    id = "loc_corridor"
    type = "indoor"
    connections = [{ to = "loc_cryo_room", direction = "北" }]
  }
}

storyline {
  chapter "冷舱梦醒" {
    id = "chap_001"
    
    objective "逃离休眠基地" {
      step 1 {
        event {
          type = "spawn"
          actor = "char_player"
          location = "loc_cryo_room"
          on_complete {
            narration = "你醒来时，休眠仓的指示灯闪烁着红光..."
          }
        }
      }
      
      step 2 {
        trigger { type = "enter_location", location = "loc_corridor" }
        
        event {
          type = "combat"
          setup {
            enemies = [{ id = "enemy_wasp", count = 1, level = 1 }]
          }
          on_victory {
            narration = "工蜂倒在地上..."
            rewards { exp = 100, items = [{ id = "item_crystal" }] }
          }
        }
      }
    }
  }
}
```

---

## 8. 附录

### A. 语法 BNF 定义

```bnf
<dsl>         ::= <block>*

<block>       ::= <block_type> <string> "{" <body> "}"
                | <block_type> "{" <body> "}"

<block_type>  ::= "metadata" | "world" | "characters" | "storyline" 
                | "systems" | "location" | "item" | "player" | "enemy" 
                | "npc" | "chapter" | "arc" | "objective" | "step" 
                | "event" | "phase" | "trigger" | "require" | "on_complete"
                | "on_fail" | "on_victory" | "on_defeat" | "effects" | "stats"

<body>        ::= (<attribute> | <block>)*

<attribute>   ::= <identifier> "=" <value>

<value>       ::= <string> | <number> | <boolean> | <list> | <object>

<string>      ::= '"' <char>* '"'

<number>      ::= <int> | <float>

<boolean>     ::= "true" | "false"

<list>        ::= "[" (<value> ("," <value>)*)? "]"

<object>      ::= "{" (<attribute> ("," <attribute>)*)? "}"
```

### B. 与现有系统集成

```go
// 使用 DSL 的流程
func LoadStoryFromDSL(dslFile string) (*rpg.World, error) {
    // 1. 读取 DSL 文件
    content, err := os.ReadFile(dslFile)
    if err != nil {
        return nil, err
    }
    
    // 2. 解析 DSL
    ast, err := dsl.Parse(content)
    if err != nil {
        return nil, fmt.Errorf("parse error: %w", err)
    }
    
    // 3. 验证 DSL
    if err := dsl.Validate(ast); err != nil {
        return nil, fmt.Errorf("validation error: %w", err)
    }
    
    // 4. 转换为 RPG World
    world, err := dsl.ConvertToWorld(ast)
    if err != nil {
        return nil, fmt.Errorf("conversion error: %w", err)
    }
    
    return world, nil
}
```

---

## 附录 A: 已决策事项

| 问题 | 决策 | 说明 |
|------|------|------|
| 1. DSL 语法风格 | **HCL-like** | 块结构清晰，AI友好，类似Terraform |
| 2. 条件表达式 | **Phase 2 添加** | 使用 `and()`, `or()`, `not()` 函数形式 |
| 3. 并行执行 | **Phase 3 考虑** | MVP不需要，先做串行 |
| 4. 内置函数 | **Phase 2 添加** | 数学、逻辑、查询函数 |
| 5. 版本升级策略 | **向后兼容** | 新版本解析器支持旧格式 |
| 6. Hook & Trigger | **Phase 2 添加** | on_kill, on_damage, 触发器系统 |
| 7. 动态难度 | **支持** | 使用函数实现自适应等级 |

## 附录 B: 示例完整 DSL

```dsl
metadata {
  title = "逐星"
  genre = ["科幻", "废土求生", "虫族"]
  power_system = "基因进化"
  dsl_version = "0.1.0"
}

characters {
  player "陆沉" {
    id = "char_player"
    class = "engineer"
    stats { str = 10, agi = 12, int = 15, vit = 10, hp = 100 }
    skills = ["skill_analyze"]
  }
  
  enemy "虫族工蜂" {
    id = "enemy_wasp"
    stats { str = 8, agi = 10, hp = 50 }
    drops { item = "item_crystal", chance = 0.8 }
  }
}

world {
  location "休眠基地" {
    id = "loc_base"
    type = "indoor"
  }
  
  location "废土" {
    id = "loc_wasteland"
    type = "outdoor"
    connections = [{ to = "loc_base", direction = "入口" }]
  }
}

storyline {
  chapter "冷舱梦醒" {
    id = "chap_001"
    
    objective "逃离休眠基地" {
      step 1 {
        event {
          type = "spawn"
          actor = "char_player"
          location = "loc_base"
          on_complete {
            narration = "你醒来时，休眠仓的指示灯闪烁着红光..."
          }
        }
      }
      
      step 2 {
        event {
          type = "combat"
          enemies = [{ id = "enemy_wasp", count = 1, level = 1 }]
          on_victory {
            narration = "工蜂倒在地上..."
            exp = 100
            items = ["item_crystal"]
          }
          on_defeat {
            narration = "眼前一黑..."
            result = "retry"
          }
        }
      }
      
      step 3 {
        event {
          type = "move"
          actor = "char_player"
          to = "loc_wasteland"
          on_complete {
            narration = "废土的风沙扑面而来..."
          }
        }
      }
    }
    
    completion {
      exp = 150
      flags = ["awakened", "first_kill"]
    }
  }
}

systems {
  progression "exp_level" {
    formula {
      exp_to_next = "level * 100"
    }
  }
  
  hooks {
    hook "on_kill" {
      counter "wasp_killed" {
        filter = "enemy_id == 'enemy_wasp'"
        on_milestone 10 {
          reward { title = "虫族杀手" }
        }
      }
    }
  }
  
  triggers {
    trigger "觉醒" {
      condition = and(
        get_stat("char_player", "hp") < 20,
        has_flag("in_combat")
      )
      on_trigger {
        unlock_trait "regeneration"
        narration = "生死之间，某种力量在苏醒..."
      }
      once = true
    }
  }
}
```

---

**下一步行动：**
1. Review 本文档
2. 确认 Phase 1 范围
3. 开始实施
