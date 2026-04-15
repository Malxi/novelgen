# Novelgen + RPG 集成改进总结

## 问题分析

原有的 novelgen 和 rpg 集成存在以下问题：

1. **RPG 系统独立运行** - 只能做验证和推演，没有直接影响 AI 写作
2. **缺乏约束机制** - AI 写作时不知道 RPG 系统的规则
3. **建议未传递到 AI** - RPG 检测的问题没有反馈给写作 Agent
4. **单向数据流** - novelgen → RPG 转换，但没有反向约束

## 解决方案

### 核心改进

#### 1. 约束系统 (ConstraintSystem)

**文件：** `internal/rpg/constraint_system.go`

将 RPG 世界数据转换为 AI 可理解的约束规则：

```go
type ConstraintSystem struct {
    World          *GameWorld
    CharacterRules map[string]*CharacterConstraint
    PlotRules      *PlotConstraint
    PowerRules     *PowerSystemConstraint
}
```

**功能：**
- 从 RPG 数据构建约束规则
- 生成约束提示词（系统提示词 + 用户提示词）
- 验证章节内容是否违反约束
- 生成修正建议

#### 2. RPG 增强写作 Agent

**文件：** `internal/agents/rpg_enhanced_write_agent.go`

扩展普通写作 Agent，集成 RPG 约束：

```go
type RPGEnhancedWriteAgent struct {
    base             *BaseAgent
    constraintSystem *rpg.ConstraintSystem
    constraintReport *rpg.ConstraintReport
}
```

**新增功能：**
- `GenerateChapter()` - 生成章节（带约束检查）
- `ReviewChapterWithRPG()` - RPG 增强的评审
- `IterateChapterWithRPG()` - RPG 增强的迭代写作
- `correctViolations()` - 自动修正约束违反

#### 3. RPG 写作提示词

**文件：** `internal/prompts/rpg_write_prompts.go`

定义 RPG 约束相关的提示词模板：

- `rpg-final` - 带约束的章节生成
- `rpg-improve` - 带约束的章节改进
- `rpg-correct` - 约束违反修正

### 约束类型

#### 角色约束
- 最大死亡次数
- 最大复活次数
- 战力变化率限制
- 出场要求

#### 剧情约束
- 时间跳跃限制
- 节奏比例控制
- 禁止元素列表

#### 战力系统约束
- 战力变化冷却期
- 复活代价
- 允许的战力跃迁路径

## 工作流程

### 改进后的写作流程

```
1. 加载 RPG 世界数据
   └─→ 从 novelgen 项目转换

2. 构建约束系统
   └─→ 生成约束规则
   └─→ 生成约束提示词

3. AI 写作（带约束）
   └─→ 系统提示词：约束规则
   └─→ 用户提示词：约束详情 + 章节信息
   └─→ AI 生成内容

4. 约束验证
   └─→ 检查约束违反
   └─→ 发现违反？
       ├─ 是 → 生成修正提示 → 重新生成
       └─ 否 → 继续

5. RPG 增强评审
   └─→ 普通评审 + RPG 约束检查
   └─→ 评分调整（约束违反扣分）

6. 输出最终结果
```

### 数据流

```
novelgen 项目
    │
    ├── characters.json ──┐
    ├── items.json ───────┼──► RPG World ──► ConstraintSystem
    ├── locations.json ───┤                      │
    └── outline.json ─────┘                      ▼
                                          ConstraintReport
                                                │
                    ┌───────────────────────────┼───────────────────────────┐
                    ▼                           ▼                           ▼
              系统提示词                    用户提示词                    验证检查
                    │                           │                           │
                    └───────────────────────────┴───────────────────────────┘
                                                │
                                                ▼
                                           AI 写作 Agent
                                                │
                    ┌───────────────────────────┼───────────────────────────┐
                    ▼                           ▼                           ▼
              生成内容                      约束验证                    修正建议
                    │                           │                           │
                    └───────────────────────────┴───────────────────────────┘
                                                │
                                                ▼
                                           最终输出
```

## 使用示例

### 命令行工具

```bash
# 查看 RPG 约束报告
go run cmd/rpg-write/main.go -b mine -report

# 输出：
# ========== Hard Constraints (Must Follow) ==========
# [character] 林砚: 最多复活3次
# [power] 战力系统: 每次战力变化至少间隔5章
# [power] 复活机制: 每次复活消耗寿命或修为
```

### 代码中使用

```go
// 创建 RPG 增强写作 Agent
writeAgent, err := agents.NewRPGEnhancedWriteAgent(
    client, config, projectLLM, setup, outline,
    projectPath, bookName,
)

// 生成章节（自动应用约束）
content, err := writeAgent.GenerateChapter(ctx, chapter, context, state, 3000)

// RPG 增强迭代写作
content, review, err := writeAgent.IterateChapterWithRPG(
    ctx, chapter, context, state, 3000, initialContent, 3, 80.0,
)
```

## 效果对比

### 改进前

```
AI 写作 → 输出内容
    ↓
可能的问题：
- 主角死亡次数过多
- 战力突然暴涨
- 时间线混乱
- 没有约束检查
```

### 改进后

```
AI 写作（带约束） → 输出内容
    ↓
约束验证
    ↓
发现死亡次数超限？
    ├─ 是 → 自动修正 → 重新生成
    └─ 否 → 继续
    ↓
RPG 增强评审
    ↓
高质量输出
```

## 文件清单

### 新增文件

1. `internal/rpg/constraint_system.go` - 约束系统核心
2. `internal/agents/rpg_enhanced_write_agent.go` - RPG 增强写作 Agent
3. `internal/prompts/rpg_write_prompts.go` - RPG 写作提示词
4. `cmd/rpg-write/main.go` - 示例命令行工具
5. `docs/RPG_CONSTRAINT_INTEGRATION.md` - 集成方案文档
6. `docs/RPG_WRITE_USAGE.md` - 使用指南

### 修改文件

1. `internal/prompts/base.go` - 注册 RPG 提示词

## 优势

1. **预防性质量控制** - 在写作前就提供约束指导
2. **自动化验证** - 写作后自动检查约束违反
3. **智能化修正** - 自动发现并修正问题
4. **数值化评估** - 通过 RPG 数值量化故事质量
5. **可扩展性** - 易于添加新的约束类型

## 未来扩展

1. **动态约束** - 根据故事发展自动调整约束
2. **约束学习** - 从优秀作品中学习约束模式
3. **多维度约束** - 添加情感、主题等约束
4. **可视化界面** - 图形化展示约束和违反
5. **约束推荐** - 根据小说类型推荐约束配置

## 总结

通过这次改进，RPG 系统从单纯的验证工具转变为 AI 写作的主动约束系统：

- **Before**: novelgen → RPG (单向转换，仅验证)
- **After**: novelgen ↔ RPG (双向交互，约束指导)

这让 AI 写作更加可控，有效避免了战力崩坏、角色死亡滥用等常见问题，显著提升了小说的一致性和可读性。
