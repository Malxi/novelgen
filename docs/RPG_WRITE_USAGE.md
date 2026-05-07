# RPG 增强写作使用指南

## 快速开始

### 1. 查看 RPG 约束报告

```bash
cd /d/Code/nolvegen
go run cmd/rpg-write/main.go -b mine -report
```

输出示例：
```
========== RPG Constraint System Demo ==========
Book: mine
Project Path: .

========== Project Summary ==========
Book Name: mine
Characters: 63
Items: 7
Locations: 73
Parts: 1

========== RPG World Created ==========
Characters in world: 63
Items in world: 7
Maps in world: 73

========== Constraint Report ==========
Character Constraints: 63
Plot Constraints: configured
Power Constraints: configured
Total Suggestions: 12

========== Hard Constraints (Must Follow) ==========
[character] 林砚: 最多复活3次
  Reason: 保持死亡的严肃性，避免读者失去紧张感 (Priority: 9)
[power] 战力系统: 每次战力变化至少间隔5章
  Reason: 给读者建立稳定预期的时间 (Priority: 9)
[power] 复活机制: 每次复活消耗寿命或修为
  Reason: 复活必须有代价，否则失去意义 (Priority: 10)

========== Soft Constraints (Recommended) ==========
[character] 林砚: 战力变化率不超过20%每章
[plot] 全局: 快节奏比例控制在30%-60%
```

### 2. 在代码中使用 RPG 增强写作

#### 基础用法

```go
package main

import (
    "context"
    "novelgen/internal/agents"
    "novelgen/internal/llm"
    "novelgen/internal/models"
)

func main() {
    // 1. 初始化 LLM 客户端
    config := &llm.Config{/* ... */}
    client := config.CreateClient(projectLLM)
    
    // 2. 加载故事数据
    setup, outline := loadStoryData()
    
    // 3. 创建 RPG 增强写作 Agent
    writeAgent, err := agents.NewRPGEnhancedWriteAgent(
        client,
        config,
        projectLLM,
        setup,
        outline,
        ".",        // projectPath
        "mine",     // bookName
    )
    if err != nil {
        panic(err)
    }
    
    // 4. 获取约束报告（可选，用于查看约束）
    constraintReport := writeAgent.GetConstraintReport()
    
    // 5. 生成章节（自动应用 RPG 约束）
    chapter := findChapter(outline, "P1-V1-C1")
    content, err := writeAgent.GenerateChapter(
        context.Background(),
        chapter,
        chapterContext,
        continuity,
        3000,  // targetWords
    )
    if err != nil {
        panic(err)
    }
    
    // 6. 保存内容
    saveContent(chapter.ID, content)
}
```

#### 带约束验证的迭代写作

```go
// 使用 RPG 增强的迭代写作
content, review, err := writeAgent.IterateChapterWithRPG(
    ctx,
    chapter,
    chapterContext,
    continuity,
    3000,   // targetWords
    initialContent,
    3,      // maxIterations
    80.0,   // qualityThreshold
)

// 检查是否有 RPG 约束违反
for _, sug := range review.Suggestions {
    if sug.Category == "rpg_constraint" {
        fmt.Printf("RPG约束违反: %s\n", sug.Issue)
        fmt.Printf("建议: %s\n", sug.Suggestion)
    }
}
```

### 3. 自定义约束规则

```go
// 创建约束系统
constraintSystem := rpg.NewConstraintSystem(world)

// 修改特定角色的约束
constraintSystem.CharacterRules["主角"] = &rpg.CharacterConstraint{
    Name:             "主角",
    MaxDeaths:        5,
    MaxResurrections: 3,
    PowerChangeRate:  0.15,  // 15% 每章
}

// 修改战力系统约束
constraintSystem.PowerRules.CooldownBetweenChanges = 10  // 10章冷却
constraintSystem.PowerRules.ResurrectionCost = "每次复活跌落一个小境界"

// 重新生成约束报告
constraintReport := constraintSystem.BuildFromRPGData()
```

### 4. 手动验证章节

```go
// 验证已写好的章节
violations := constraintSystem.ValidateChapter("P1-V1-C1", chapterContent)

if len(violations) > 0 {
    fmt.Println("发现约束违反：")
    for _, v := range violations {
        fmt.Printf("[%s] %s: %s\n", v.Severity, v.Target, v.Issue)
        fmt.Printf("  建议: %s\n", v.Suggestion)
    }
    
    // 生成修正提示词
    correctionPrompt := constraintSystem.GenerateCorrectionPrompt(violations)
    fmt.Println(correctionPrompt)
}
```

## 约束类型详解

### 角色约束

```go
type CharacterConstraint struct {
    Name                 string
    MaxDeaths            int      // 最大死亡次数
    MaxResurrections     int      // 最大复活次数
    PowerChangeRate      float64  // 战力变化率（每章）
    RequiredPresence     map[string]float64  // 出场要求
    Relationships        []RelationshipConstraint
}
```

**默认规则：**
- 主角：最多死亡 5 次，复活 3 次
- 反派：最多死亡 1 次，不能复活
- NPC：最多死亡 3 次，复活 2 次

### 剧情约束

```go
type PlotConstraint struct {
    MaxTimeJumpsPerChapter int      // 每章最大时间跳跃
    MinPacingRatio         float64  // 慢节奏最小比例
    MaxPacingRatio         float64  // 快节奏最大比例
    ForbiddenElements      []string // 禁止的元素
}
```

**默认规则：**
- 每章最多 2 次时间跳跃
- 快节奏比例 30%-60%
- 禁止：无理由复活、战力突然暴涨

### 战力系统约束

```go
type PowerSystemConstraint struct {
    MaxPowerChangesPerArc   int                 // 每弧最大战力变化
    CooldownBetweenChanges  int                 // 变化冷却（章节数）
    ResurrectionCost        string              // 复活代价
    AllowedPowerTransitions map[string][]string // 允许的战力跃迁
}
```

**默认规则：**
- 战力变化冷却：5 章
- 复活代价：消耗寿命或修为
- 境界突破路径：练气→筑基→金丹→元婴→化神→合体→大乘→渡劫

## 提示词集成

### 在系统提示词中添加约束

```go
// 生成系统提示词
systemPrompt := constraintSystem.ToSystemPrompt(constraintReport)

// 添加到 AI 请求
messages := []llm.Message{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: userPrompt},
}
```

### 在用户提示词中添加约束

```go
// 生成完整约束提示词
constraintPrompt := constraintSystem.ToPromptFormat(constraintReport)

// 添加到用户输入
userPrompt := fmt.Sprintf(`
%s

=== 章节信息 ===
章节: %s
概要: %s

请根据以上约束写作...
`, constraintPrompt, chapter.ID, chapter.Summary)
```

## 最佳实践

### 1. 约束强度分级

```go
// 硬性约束 - 必须遵守
hardConstraints := []rpg.ConstraintSuggestion{
    {Type: "hard", Target: "主角", Constraint: "最多复活3次", Priority: 10},
    {Type: "hard", Target: "战力系统", Constraint: "变化冷却5章", Priority: 9},
}

// 软性约束 - 建议遵守
softConstraints := []rpg.ConstraintSuggestion{
    {Type: "soft", Target: "节奏", Constraint: "快节奏40%", Priority: 5},
}
```

### 2. 动态调整约束

```go
// 根据章节位置调整约束
if chapterIndex < 50 {
    // 前期：严格限制战力变化
    constraintSystem.PowerRules.CooldownBetweenChanges = 10
} else {
    // 后期：适当放宽
    constraintSystem.PowerRules.CooldownBetweenChanges = 5
}
```

### 3. 约束验证流程

```go
// 1. 写作前：生成约束提示
constraintPrompt := constraintSystem.ToPromptFormat(report)

// 2. 写作中：AI 遵守约束（通过提示词引导）
content := generateWithConstraints(constraintPrompt)

// 3. 写作后：验证约束
violations := constraintSystem.ValidateChapter(chapterID, content)

// 4. 如有违反：自动修正
if len(violations) > 0 {
    corrected := correctViolations(content, violations)
}
```

## 故障排除

### 问题：RPG 项目加载失败

**原因：**
- 缺少 craft 数据文件（characters.json, items.json, locations.json）
- JSON 格式错误

**解决：**
```bash
# 确保文件存在
ls books/mine/story/craft/
# 应该有: characters.json, items.json, locations.json

# 验证 JSON 格式
cat books/mine/story/craft/characters.json | jq .
```

### 问题：约束过于严格

**解决：**
```go
// 放宽约束
constraintSystem.PowerRules.CooldownBetweenChanges = 3  // 从5改为3
constraintSystem.CharacterRules["主角"].MaxResurrections = 5  // 从3改为5
```

### 问题：约束验证误报

**解决：**
```go
// 调整检测敏感度
// 修改 constraint_system.go 中的检测阈值
func (cs *ConstraintSystem) detectPowerChanges(text string) []string {
    // 增加更多排除模式
    excludePatterns := []string{"感悟", "领悟", "体会"}  // 这些不算战力变化
    // ...
}
```

## 总结

RPG 增强写作通过以下方式提升 AI 写作质量：

1. **预防性控制**：写作前提供约束指导
2. **自动化验证**：写作后自动检查
3. **智能化修正**：自动修复约束违反
4. **数值化评估**：量化故事质量

使用 RPG 约束系统，可以有效避免：
- 战力崩坏
- 角色死亡滥用
- 时间线混乱
- 节奏失衡

让你的小说更加合理、连贯、引人入胜！
