# RPG 约束系统集成方案

## 概述

这个集成方案将 RPG 系统深度整合到 novelgen 的 AI 写作流程中，实现了：

1. **RPG 约束指导写作** - AI 在写作时必须遵守 RPG 系统定义的规则
2. **自动约束验证** - 写作完成后自动检查是否违反约束
3. **智能修正建议** - 发现违反时自动生成修正方案
4. **数值化质量控制** - 通过 RPG 数值系统量化故事质量

## 核心组件

### 1. 约束系统 (ConstraintSystem)

文件：`internal/rpg/constraint_system.go`

负责将 RPG 世界数据转换为 AI 可理解的约束规则：

```go
type ConstraintSystem struct {
    World          *GameWorld
    CharacterRules map[string]*CharacterConstraint
    PlotRules      *PlotConstraint
    PowerRules     *PowerSystemConstraint
}
```

**主要功能：**
- 从 RPG 世界数据构建约束规则
- 生成约束提示词
- 验证章节内容是否违反约束
- 生成修正建议

### 2. RPG 增强写作 Agent

文件：`internal/agents/rpg_enhanced_write_agent.go`

扩展了普通写作 Agent，添加了 RPG 约束支持：

```go
type RPGEnhancedWriteAgent struct {
    base             *BaseAgent
    constraintSystem *rpg.ConstraintSystem
    constraintReport *rpg.ConstraintReport
}
```

**主要方法：**
- `GenerateChapter()` - 生成章节（带约束检查）
- `ReviewChapterWithRPG()` - RPG 增强的评审
- `IterateChapterWithRPG()` - RPG 增强的迭代写作
- `correctViolations()` - 自动修正约束违反

### 3. RPG 写作提示词

文件：`internal/prompts/rpg_write_prompts.go`

定义了 RPG 约束相关的提示词模板：

- `rpg-final` - 带约束的章节生成
- `rpg-improve` - 带约束的章节改进
- `rpg-correct` - 约束违反修正

## 约束类型

### 角色约束 (CharacterConstraint)

```go
type CharacterConstraint struct {
    Name                 string
    MaxDeaths            int      // 最大死亡次数
    MaxResurrections     int      // 最大复活次数
    PowerChangeRate      float64  // 战力变化率限制
    RequiredPresence     map[string]float64  // 出场要求
    Relationships        []RelationshipConstraint
}
```

**示例约束：**
- 主角最多死亡 5 次，复活 3 次
- 战力变化率不超过 20% 每章
- 主线剧情必须 80% 出场

### 剧情约束 (PlotConstraint)

```go
type PlotConstraint struct {
    MaxTimeJumpsPerChapter int      // 每章最大时间跳跃
    MinPacingRatio         float64  // 慢节奏最小比例
    MaxPacingRatio         float64  // 快节奏最大比例
    ForbiddenElements      []string // 禁止的元素
}
```

**示例约束：**
- 每章最多 2 次时间跳跃
- 快节奏比例控制在 30%-60%
- 禁止无理由的复活

### 战力系统约束 (PowerSystemConstraint)

```go
type PowerSystemConstraint struct {
    MaxPowerChangesPerArc   int                 // 每弧最大战力变化
    CooldownBetweenChanges  int                 // 变化冷却（章节数）
    ResurrectionCost        string              // 复活代价
    AllowedPowerTransitions map[string][]string // 允许的战力跃迁
}
```

**示例约束：**
- 战力变化至少间隔 5 章
- 复活必须消耗寿命或修为
- 只能按设定路径突破境界

## 使用流程

### 1. 普通写作 vs RPG 增强写作

**普通写作：**
```go
writeAgent := agents.NewWriteAgent(client, config, projectLLM, setup, outline)
content, err := writeAgent.GenerateChapter(ctx, chapter, context, state, 3000)
```

**RPG 增强写作：**
```go
writeAgent, err := agents.NewRPGEnhancedWriteAgent(
    client, config, projectLLM, setup, outline, projectPath, bookName,
)
content, err := writeAgent.GenerateChapter(ctx, chapter, context, state, 3000)
```

### 2. 约束验证流程

```
生成章节内容
    ↓
RPG 约束验证
    ↓
发现违反？
    ├─ 是 → 生成修正提示 → 重新生成
    └─ 否 → 继续
    ↓
RPG 增强评审
    ↓
输出最终结果
```

### 3. 命令行工具

```bash
# RPG 增强写作
go run cmd/rpg-write/main.go -b mine -c P1-V1-C1

# 查看约束报告
go run cmd/rpg-write/main.go -b mine -c P1-V1-C1 -report-only
```

## 集成效果

### 写作前：约束指导

AI 在写作前会收到约束提示：

```
=== RPG系统约束规则 ===

【角色约束】
角色: 林砚
  - 最大死亡次数: 5
  - 最大复活次数: 3
  - 战力变化率限制: 20%每章

【剧情约束】
  - 每章最多2次时间跳跃
  - 快节奏比例: 30%-60%
  - 禁止元素:
    * 无理由的复活
    * 战力突然暴涨

【战力系统约束】
  - 战力变化冷却: 5章
  - 复活代价: 每次复活消耗寿命或修为
```

### 写作中：自动检查

AI 在写作时会自动检查：
- 是否超过死亡/复活限制
- 战力变化是否符合冷却期
- 时间跳跃是否在允许范围内

### 写作后：约束验证

```go
violations := constraintSystem.ValidateChapter(chapterID, content)
for _, v := range violations {
    // 处理违反
    logger.Warn("RPG约束违反: %s - %s", v.Target, v.Issue)
}
```

### 违反修正

发现违反时自动生成修正提示：

```
=== RPG约束违反，需要修正 ===

问题 1:
  类型: power
  对象: 战力系统
  问题: 战力变化过于频繁: 4次
  严重程度: critical
  建议: 减少突破次数，增加修炼过程的描写

请根据以上约束违反情况修改内容...
```

## 配置示例

### 自定义约束规则

```go
// 创建自定义约束系统
constraintSystem := rpg.NewConstraintSystem(world)

// 修改角色约束
constraintSystem.CharacterRules["主角"] = &rpg.CharacterConstraint{
    Name: "主角",
    MaxDeaths: 3,
    MaxResurrections: 1,
    PowerChangeRate: 0.15,
}

// 修改战力约束
constraintSystem.PowerRules.CooldownBetweenChanges = 10
```

### 在提示词中使用

```go
// 生成约束提示词
constraintPrompt := constraintSystem.ToPromptFormat(constraintReport)

// 添加到 AI 输入
input := RPGWriteGenInput{
    // ... 其他字段
    RPGConstraints: constraintPrompt,
}
```

## 最佳实践

### 1. 约束粒度

- **硬性约束**：死亡次数、复活次数等必须严格遵守
- **软性约束**：战力变化率、时间跳跃等可以灵活处理
- **建议性约束**：角色出场比例等作为参考

### 2. 约束调整

根据小说类型调整约束：

**修仙小说：**
- 战力变化冷却：5-10 章
- 复活代价：修为跌落或寿命减少

**都市小说：**
- 战力变化冷却：不适用
- 重点约束：角色关系逻辑

**科幻小说：**
- 重点约束：科技水平一致性

### 3. 迭代优化

```go
// 第一次迭代：生成内容
content, _ := writeAgent.GenerateChapter(...)

// 第二次迭代：修正约束违反
corrected, _ := writeAgent.correctViolations(...)

// 第三次迭代：质量提升
final, _ := writeAgent.GenerateChapterWithRPGSuggestions(...)
```

## 未来扩展

### 1. 动态约束

根据故事发展动态调整约束：

```go
// 后期剧情放宽战力变化限制
if chapterIndex > 100 {
    constraintSystem.PowerRules.CooldownBetweenChanges = 3
}
```

### 2. 约束学习

从优秀作品中学习约束模式：

```go
// 分析成功小说的约束模式
pattern := constraintSystem.LearnFromNovel(successfulNovel)
constraintSystem.ApplyPattern(pattern)
```

### 3. 多维度约束

添加更多约束维度：

```go
type EmotionalConstraint struct {
    MinEmotionalIntensity float64
    MaxEmotionalIntensity float64
    RequiredEmotions      []string
}
```

## 总结

RPG 约束系统集成方案通过将 RPG 数值系统与 AI 写作深度结合，实现了：

1. **预防性质量控制** - 在写作前就提供约束指导
2. **自动化验证** - 写作后自动检查约束违反
3. **智能化修正** - 自动发现并修正问题
4. **数值化评估** - 通过 RPG 数值量化故事质量

这种集成方式让 AI 写作更加可控，有效避免了战力崩坏、角色死亡滥用等常见问题，提升了小说的一致性和可读性。
