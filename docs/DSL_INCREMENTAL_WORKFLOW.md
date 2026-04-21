# DSL-RPG 渐进式生成工作流

> 解决 Outline → Craft → DSL 的信息补充问题

---

## 核心设计：三层 DSL 结构

```
books/<book>/story/rpg/
├── 01_outline.rpg      # Phase 1: Outline 生成（基础框架）
├── 02_craft.rpg        # Phase 2: Craft 生成（详细信息）  
└── 03_systems.rpg      # Phase 3: 系统规则（可选）

# 运行时自动合并为完整 DSL
merge(01_outline.rpg + 02_craft.rpg + 03_systems.rpg) → final.rpg
```

---

## Phase 1: Outline DSL（基础框架）

```dsl
# 01_outline.rpg
# 只包含 outline.json 能确定的信息

metadata {
  title       = "火银河"
  dsl_version = "0.2.0"
  phase       = "outline"  # 标记当前阶段
}

# 角色框架：只有名字和基本关系
characters {
  player "陆星眠" {
    id   = "char_luxingmian"
    # 注意：没有 stats, skills 等详细信息
    # 标记为 "待填充"
    __placeholder__ = true
  }
  
  enemy "虫族工蜂" {
    id   = "enemy_insect_worker"
    __placeholder__ = true
  }
}

# 地点框架：只有名字和连接关系
world {
  location "西昌航天发射场地下休眠中心" {
    id   = "loc_cryo_center"
    __placeholder__ = true
    
    # 连接关系可以从 outline 推断
    connection "休眠中心入口通道" {
      direction = "exit"
    }
  }
  
  location "休眠中心入口通道" {
    id   = "loc_cryo_entrance"
    __placeholder__ = true
  }
}

# 剧情框架：从 outline.json 直接转换
storyline {
  chapter "P1-V1-C1" {
    id    = "ch1_awakening"
    title = "休眠仓的冷光"
    
    objective "苏醒并探索" {
      step 1 "苏醒" {
        description = "从休眠仓中醒来"
        event "status_change" {
          subject = "休眠状态"
          change  = "resolved"
        }
      }
      
      step 2 "遭遇虫族" {
        description = "首次遭遇虫族工蜂"
        event "combat" {
          enemies = ["enemy_insect_worker"]
        }
      }
    }
  }
}
```

**特点：**
- ✅ 可以由 AI 直接从 outline.json 生成
- ✅ 包含完整的故事结构（章节、事件顺序）
- ⚠️ 缺少具体数值和描述
- 🔧 使用 `__placeholder__` 标记待填充字段

---

## Phase 2: Craft DSL（详细信息）

```dsl
# 02_craft.rpg
# 基于 craft/*.json 生成的详细信息

metadata {
  phase = "craft"  # 标记为补充阶段
}

# 角色详情：补充 stats, skills, background
characters {
  player "陆星眠" {
    id = "char_luxingmian"
    
    # 覆盖基础框架中的 placeholder
    __placeholder__ = false
    
    # 从 characters.json 提取
    background = "21世纪航天工程师，因实验事故意外进入休眠仓"
    age        = 28
    gender     = "男"
    
    stats {
      str = 12
      agi = 14
      int = 16
      vit = 10
      hp  = 100
      mp  = 50
    }
    
    skills = ["工程学", "快速学习", "适应环境"]
    
    traits {
      trait "冬眠者体质" {
        description = "对辐射和毒素有天然抗性"
        effects     = ["radiation_resist+20", "toxin_resist+15"]
      }
    }
  }
}

# 物品详情：从 items.json 提取
world {
  item "联邦民用身份识别手环" {
    id          = "item_civ_id_band"
    description = "可联网的智能手环，显示当前时间为星元327年"
    type        = "key_item"
    rarity      = "common"
    
    effects {
      effect "时间显示" {
        description = "显示当前星元纪年时间"
      }
      effect "身份识别" {
        description = "联邦公民身份认证"
      }
    }
  }
}

# 地点详情：从 locations.json 提取
world {
  location "西昌航天发射场地下休眠中心" {
    id = "loc_cryo_center"
    
    __placeholder__ = false
    
    description = "位于西昌航天发射场地下300米的休眠设施，建于2247年"
    atmosphere  = "阴暗潮湿，布满灰尘，设备大多已损坏"
    history     = "原本用于深空探索人员的长期休眠实验"
    
    sensory {
      visual = ["布满灰尘的休眠仓", "闪烁的红色警示灯", "破损的管线"]
      audio  = ["冷却系统的低鸣", "远处传来的嘶鸣声"]
      smell  = ["霉味", "金属锈蚀的味道"]
    }
  }
}
```

**特点：**
- ✅ 包含完整的数值和描述
- ✅ 使用相同 ID 与 outline DSL 对应
- ✅ `__placeholder__ = false` 表示已填充
- 🔧 可以增量生成（每个角色/地点独立文件）

---

## DSL 合并算法

```go
// internal/rpg/dsl/merger.go

package dsl

// DSLMerger 合并多个 DSL 片段
type DSLMerger struct {
    fragments []*DSL
}

// Merge 按优先级合并 DSL
// 后加载的片段覆盖先加载的片段
func (dm *DSLMerger) Merge() (*DSL, error) {
    result := &DSL{
        Metadata:   &Metadata{},
        World:      &World{},
        Characters: &Characters{},
        Storyline:  &Storyline{},
        Systems:    &Systems{},
    }
    
    for i, fragment := range dm.fragments {
        phase := fragment.Metadata.Phase
        
        switch phase {
        case "outline":
            // Phase 1: 初始化框架
            dm.mergeOutline(result, fragment)
            
        case "craft":
            // Phase 2: 填充详细信息
            dm.mergeCraft(result, fragment)
            
        case "systems":
            // Phase 3: 添加系统规则
            dm.mergeSystems(result, fragment)
        }
        
        log.Printf("[DSL Merger] 合并第 %d 个片段 (phase=%s)", i+1, phase)
    }
    
    // 验证是否有未填充的 placeholder
    if err := dm.validatePlaceholders(result); err != nil {
        return nil, err
    }
    
    return result, nil
}

// mergeOutline 合并基础框架
func (dm *DSLMerger) mergeOutline(base, outline *DSL) {
    // 元数据
    base.Metadata.Title = outline.Metadata.Title
    base.Metadata.Genre = outline.Metadata.Genre
    
    // 角色框架
    for _, char := range outline.Characters.GetAll() {
        if char.IsPlaceholder {
            base.Characters.AddPlaceholder(char)
        } else {
            base.Characters.Add(char)
        }
    }
    
    // 地点框架
    for _, loc := range outline.World.Locations {
        if loc.IsPlaceholder {
            base.World.AddLocationPlaceholder(loc)
        } else {
            base.World.Locations = append(base.World.Locations, loc)
        }
    }
    
    // 剧情框架（outline 的剧情最完整）
    base.Storyline = outline.Storyline
}

// mergeCraft 合并详细信息
func (dm *DSLMerger) mergeCraft(base, craft *DSL) {
    // 填充角色详情
    for _, char := range craft.Characters.GetAll() {
        if placeholder := base.Characters.GetPlaceholder(char.ID); placeholder != nil {
            // 用 craft 数据填充 placeholder
            base.Characters.FillPlaceholder(char.ID, char)
        } else {
            // 或者添加新角色
            base.Characters.Add(char)
        }
    }
    
    // 填充地点详情
    for _, loc := range craft.World.Locations {
        if placeholder := base.World.GetLocationPlaceholder(loc.ID); placeholder != nil {
            base.World.FillLocationPlaceholder(loc.ID, loc)
        }
    }
    
    // 添加物品
    for _, item := range craft.World.Items {
        base.World.Items = append(base.World.Items, item)
    }
}

// validatePlaceholders 检查是否还有未填充的 placeholder
func (dm *DSLMerger) validatePlaceholders(dsl *DSL) error {
    var unfilled []string
    
    for _, char := range dsl.Characters.GetAll() {
        if char.IsPlaceholder {
            unfilled = append(unfilled, fmt.Sprintf("角色: %s", char.ID))
        }
    }
    
    for _, loc := range dsl.World.Locations {
        if loc.IsPlaceholder {
            unfilled = append(unfilled, fmt.Sprintf("地点: %s", loc.ID))
        }
    }
    
    if len(unfilled) > 0 {
        return fmt.Errorf("以下元素未填充详细信息:\n%s", strings.Join(unfilled, "\n"))
    }
    
    return nil
}
```

---

## 完整工作流程

```bash
# Phase 1: Outline 生成
novelgen outline generate -b fire-galaxy
# 生成:
#   story/compose/outline.json
#   story/rpg/01_outline.rpg

# Phase 2: Craft 生成（可以逐个角色/地点）
novelgen craft character -b fire-galaxy -n "陆星眠"
# 生成:
#   story/craft/characters.json (更新)
#   story/rpg/02_craft.rpg (追加)

novelgen craft location -b fire-galaxy -n "西昌航天发射场"
# 更新 02_craft.rpg

# Phase 3: DSL 合并与验证
novelgen-rpg validate -b fire-galaxy
# 输出:
#   1. 加载 01_outline.rpg... ✓
#   2. 加载 02_craft.rpg... ✓
#   3. 合并 DSL... ✓
#   4. 检查 placeholder... 
#      ⚠️  未填充: 地点 "休眠中心入口通道"

# Phase 4: 交互式补充缺失信息
novelgen-rpg prompt-missing -b fire-galaxy
# 输出缺失元素的 AI Prompt，可复制到 LLM

# Phase 5: 完整转换
novelgen-rpg convert -b fire-galaxy -o rpg_data.json
# 自动合并所有 DSL 片段
```

---

## 备选方案对比

### 方案 A: 延迟解析（Lazy Resolution）

```dsl
# 使用特殊语法标记延迟解析
characters {
  player "陆星眠" {
    id = "char_luxingmian"
    
    # 延迟解析：等 craft 数据加载后再求值
    stats = $ref("craft://characters/陆星眠/stats")
    background = $ref("craft://characters/陆星眠/background")
  }
}
```

**优点：** 更灵活，支持动态更新  
**缺点：** 实现复杂，调试困难

### 方案 B: 智能推断（Smart Inference）

```go
// 在没有详细信息时，基于名称和上下文推断
func InferCharacterStats(name string, context StoryContext) BaseStats {
    base := BaseStats{HP: 100, MP: 50, Attack: 10}
    
    // 根据名字推断
    if strings.Contains(name, "虫") || strings.Contains(name, "兽") {
        base.Attack += 5
        base.HP -= 10  // 虫族通常攻高血低
    }
    
    // 根据上下文推断
    if context.IsFirstChapter {
        base.HP = 50   // 第一章敌人较弱
    }
    
    return base
}
```

**优点：** 无需等待 craft 完成  
**缺点：** 推断可能不准确

### 方案 C: 推荐方案（渐进式合并）

**优点：**
- ✅ 清晰的分阶段结构
- ✅ 易于调试（每个阶段可独立查看）
- ✅ 支持增量更新（修改 craft 无需重新生成 outline）
- ✅ 可追踪信息来源

**缺点：**
- ⚠️ 需要实现合并算法
- ⚠️ 需要处理冲突（同名字段）

---

## 推荐实现顺序

1. **立即实现**: DSL Merger 基础框架
2. **高优先级**: Outline → DSL 自动生成
3. **中优先级**: Craft → DSL 自动追加
4. **低优先级**: 智能推断作为 fallback

这样可以在 craft 未完成时先用 outline DSL 进行基础推演，craft 完成后再获得详细信息！
