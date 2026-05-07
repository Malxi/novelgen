# Novelgen 项目架构文档

> 版本: 0.3.0  
> 最后更新: 2026-04-24  
> 状态: 活跃开发中

---

## 1. 项目概述

Novelgen 是一个 AI 辅助小说创作的命令行工具，提供从创意到成书的完整工作流。项目特色是引入了 RPG-DSL 系统，将小说内容转化为可执行的游戏化数据结构，实现智能化的内容验证和推演。

### 核心价值

- **结构化创作**: 部→卷→章的三级大纲体系
- **AI 驱动**: 每个阶段都有专门的 AI Agent
- **RPG 验证**: 通过 DSL 系统验证内容一致性
- **约束写作**: RPG 约束指导 AI 写作，避免战力崩坏等问题

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI 层 (cmd/)                         │
│  root.go → registry.go → [setup/compose/craft/draft/write]  │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                     Agent 层 (internal/agents/)              │
│  ┌─────────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌──────┐         │
│  │ setup   │ │compose │ │craft │ │draft │ │write │ ...     │
│  │ agent   │ │ agent  │ │agent │ │agent │ │agent │         │
│  └─────────┘ └────────┘ └──────┘ └──────┘ └──────┘         │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                    核心服务层                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ LLM Client   │  │ RPG-DSL      │  │ Continuity Logic │  │
│  │ (internal/   │  │ Engine       │  │ (internal/logic/ │  │
│  │  llm/)       │  │ (internal/   │  │  continuity/)    │  │
│  │              │  │  rpg/)       │  │                  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                    数据模型层                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Story Setup  │  │ Outline      │  │ Elements         │  │
│  │ (models/)    │  │ (models/)    │  │ (models/)        │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
novelgen/
├── cmd/                          # CLI 命令实现
│   ├── root.go                   # 根命令
│   ├── registry.go               # 命令注册
│   ├── setup.go                  # 设定命令
│   ├── compose.go                # 大纲命令
│   ├── craft.go                  # 元素命令
│   ├── draft.go                  # 草稿命令
│   ├── write.go                  # 写作命令
│   ├── export.go                 # 导出命令
│   ├── recap_cmd.go              # 回顾命令
│   ├── translate.go              # 翻译命令
│   ├── config.go                 # 配置命令
│   ├── rpg.go                    # RPG 命令
│   ├── rpg_dsl.go                # RPG DSL 命令
│   ├── simulate_dsl.go           # DSL 模拟命令
│   └── check_novel/              # 小说检查工具
│       └── main.go
│
├── internal/                     # 内部包
│   ├── agents/                   # AI Agent 层
│   │   ├── agent_base.go         # Agent 基类
│   │   ├── setup_agent.go        # 设定 Agent
│   │   ├── compose_agent.go      # 大纲 Agent
│   │   ├── craft_agent.go        # 元素 Agent
│   │   ├── draft_agent.go        # 草稿 Agent
│   │   ├── write_agent.go        # 写作 Agent
│   │   ├── recap_agent.go        # 回顾 Agent
│   │   ├── translate_agent.go    # 翻译 Agent
│   │   ├── rpg_enhanced_write_agent.go  # RPG 增强写作
│   │   ├── ai_rpg_pipeline.go    # AI→RPG 管道
│   │   ├── chapter_analysis_agent.go    # 章节分析
│   │   ├── chapter_to_dsl_agent.go      # 章节→DSL
│   │   ├── skill_loader.go       # Skill 加载器
│   │   └── skills/               # Agent Skills 定义
│   │
│   ├── llm/                      # LLM 集成
│   │   ├── client.go             # OpenAI 兼容客户端
│   │   └── config.go             # LLM 配置管理
│   │
│   ├── models/                   # 数据模型
│   │   ├── project.go            # 项目配置
│   │   ├── story_setup.go        # 故事设定
│   │   ├── outline.go            # 大纲模型
│   │   ├── elements.go           # 世界元素
│   │   ├── recap.go              # 章节回顾
│   │   └── review.go             # 评审模型
│   │
│   ├── rpg/                      # RPG 系统
│   │   ├── dsl/                  # DSL 引擎
│   │   │   ├── ast.go            # AST 定义
│   │   │   ├── parser.go         # DSL 解析器
│   │   │   ├── validator.go      # DSL 验证器
│   │   │   ├── converter.go      # DSL→RPG 转换
│   │   │   ├── simulator.go      # 模拟器
│   │   │   ├── evaluator.go      # 表达式求值
│   │   │   ├── hook.go           # Hook 系统
│   │   │   └── merger.go         # DSL 合并器
│   │   ├── benchmark/            # 基准测试
│   │   │   ├── benchmark.go      # 基准测试核心
│   │   │   ├── test_cases.go     # 测试用例
│   │   │   ├── metrics.go        # 指标收集
│   │   │   ├── cross_chapter.go  # 跨章检测
│   │   │   ├── fuzz_generator.go # 模糊测试
│   │   │   ├── chapter_sort.go   # 章节文件排序
│   │   │   └── novel_checker_v2.go # 智能小说检查器
│   │   ├── constraint_system.go  # 约束系统
│   │   ├── simulation.go         # 模拟引擎
│   │   ├── novelgen_adapter.go   # Novelgen 适配器
│   │   ├── novel_simulator.go    # 小说模拟器
│   │   └── shared_models.go      # 共享模型
│   │
│   ├── logic/                    # 业务逻辑
│   │   ├── continuity/           # 连续性检查
│   │   │   ├── character/        # 角色出场
│   │   │   ├── transition/       # 转场桥段
│   │   │   └── recap/            # 回顾处理
│   │   ├── chapter_continuity.go # 写作连续性快照
│   │   ├── state_matrix.go       # 旧版状态折叠适配层
│   │   ├── id_manager.go         # ID 管理
│   │   └── dependency_executor.go # 依赖执行器
│   │
│   └── logger/                   # 日志系统
│       └── logger.go
│
├── docs/                         # 技术文档
│   ├── DSL_RPG_INTEGRATION_SPEC.md    # DSL-RPG 集成规范
│   ├── RPG_DSL_SPEC.md                # DSL 规格文档
│   ├── DSL_INCREMENTAL_WORKFLOW.md    # 渐进式工作流
│   ├── RPG_CONSTRAINT_INTEGRATION.md  # 约束系统集成
│   ├── RPG_WRITE_USAGE.md             # RPG 写作指南
│   ├── NOVELGEN_RPG_INTEGRATION.md    # RPG 集成指南
│   └── rpg-compose-integration.md     # RPG-Compose 集成
│
├── books/                        # 小说项目
│   ├── mine/                     # 示例项目
│   │   ├── novel.json
│   │   ├── story/
│   │   │   ├── setup/            # 故事设定
│   │   │   ├── compose/          # 大纲
│   │   │   ├── craft/            # 世界元素
│   │   │   ├── recaps/           # 章节回顾
│   │   │   ├── reviews/          # 评审结果
│   │   │   └── rpg/              # RPG DSL 文件
│   │   ├── drafts/               # 草稿章节
│   │   └── chapters/             # 最终章节
│   └── fire-galaxy/              # 另一个示例项目
│
├── configs/                      # 配置文件示例
├── examples/                     # 代码示例
│
├── build.ps1 / build.bat         # 构建脚本
├── go.mod / go.sum               # Go 模块
├── README.md                     # 主文档
├── AGENTS.md                     # AI 助手指南
└── ARCHITECTURE.md               # 架构文档（本文件）
```

---

## 3. 核心工作流

### 3.1 创作流程

```
1. init     → 初始化项目结构
2. setup    → AI 生成故事设定（类型、前提、主题）
3. compose  → AI 生成三级大纲（部→卷→章）
4. craft    → AI 生成世界元素（角色、地点、物品）
5. draft    → AI 生成草稿章节
   └─→ review → 评审草稿
   └─→ improve → 改进草稿（含连续性修复）
6. write    → AI 生成最终章节
   └─→ review → 评审章节
   └─→ improve → 改进章节（含 RPG 约束检查）
7. export   → 导出为 Markdown/TXT
```

### 3.2 RPG-DSL 验证流程

```
故事数据 (JSON)
    ↓
AI 分析 → RPG DSL 文件 (.rpg)
    ↓
DSL Parser → AST
    ↓
DSL Validator → 验证报告
    ↓
DSL Converter → RPG World
    ↓
Simulator → 模拟报告（问题检测）
    ↓
Constraint System → 约束规则
    ↓
反馈给 AI 写作 Agent
```

---

## 4. 关键组件详解

### 4.1 CLI 命令系统

基于 Cobra 框架构建，命令注册机制在 `cmd/registry.go`：

```go
// 命令注册示例
func RegisterSetupCommand(rootCmd *cobra.Command) {
    setupCmd := &cobra.Command{
        Use:   "setup",
        Short: "创建故事设定",
    }
    
    setupCmd.AddCommand(
        newSetupGenCmd(),
        newSetupRegenCmd(),
        newSetupImproveCmd(),
        newSetupImportCmd(),
    )
    
    rootCmd.AddCommand(setupCmd)
}
```

### 4.2 Agent 系统

每个创作阶段都有专门的 Agent，继承自 `BaseAgent`：

```go
type BaseAgent struct {
    client     *llm.Client
    config     *llm.Config
    model      string
    skills     map[string]*Skill
}

// Agent 注册在 internal/agents/registry.go
```

**Agent 列表：**
- `SetupAgent` - 故事设定生成
- `ComposeAgent` - 大纲生成
- `CraftAgent` - 世界元素生成
- `DraftAgent` - 草稿生成/评审/改进
- `WriteAgent` - 最终章节生成
- `RPGEnhancedWriteAgent` - RPG 约束增强写作
- `RecapAgent` - 章节回顾提取
- `TranslateAgent` - 翻译

### 4.3 RPG-DSL 引擎

#### DSL 语法示例

```dsl
metadata {
    title = "矿脉仙途"
    genre = ["修仙", "穿越", "悬疑"]
    dsl_version = "0.2.0"
}

characters {
    player "林砚" {
        id = "char_player"
        stats {
            hp = 100
            mp = 50
            attack = 15
        }
    }
}

storyline {
    chapter "P1-V1-C1" {
        objective "寒矿醒转" {
            step 1 "苏醒" {
                event "status_change" {
                    subject = "休眠状态"
                    change = "resolved"
                }
            }
        }
    }
}
```

#### DSL 处理流程

1. **Parser** (`dsl/parser.go`): 解析 DSL 文本为 AST
2. **Validator** (`dsl/validator.go`): 验证引用、类型、完整性
3. **Converter** (`dsl/converter.go`): 转换为 RPG World 对象
4. **Simulator** (`dsl/simulator.go`): 执行模拟，检测问题
5. **Merger** (`dsl/merger.go`): 合并多个 DSL 片段

### 4.4 约束系统

将 RPG 数据转换为 AI 写作约束：

```go
type ConstraintSystem struct {
    World          *GameWorld
    CharacterRules map[string]*CharacterConstraint  // 角色约束
    PlotRules      *PlotConstraint                   // 剧情约束
    PowerRules     *PowerSystemConstraint            // 战力约束
}
```

**约束类型：**
- **角色约束**: 死亡次数、复活限制、战力变化率
- **剧情约束**: 时间跳跃、节奏比例、禁止元素
- **战力约束**: 变化冷却、复活代价、突破路径

### 4.5 连续性系统

确保章节间逻辑连贯：

- **Character Presence**: 检测角色出场合理性
- **Transition Bridge**: 生成章节转场桥段
- **Recap**: 提取章节关键信息供后续章节使用

---

## 5. 数据流

### 5.1 创作数据流

```
novel.json (项目配置)
    ↓
story/setup/story_setup.json (故事设定)
    ↓
story/compose/outline.json (大纲)
    ↓
story/craft/{characters,locations,items}.json (世界元素)
    ↓
drafts/C{n}.md (草稿) → story/reviews/ (评审)
    ↓
chapters/chapter-{n}.md (最终章节)
    ↓
story/recaps/{chapter_id}.json (章节回顾)
```

### 5.2 RPG 数据流

```
story/craft/*.json + story/compose/outline.json
    ↓
novelgen_adapter.go (转换为 RPG World)
    ↓
constraint_system.go (生成约束规则)
    ↓
rpg_enhanced_write_agent.go (约束指导写作)
    ↓
simulation.go (剧情推演)
    ↓
rpg_data.json (输出)
```

### 5.3 DSL 数据流

```
AI 分析章节/大纲
    ↓
story/rpg/{01_outline,02_craft,03_systems}.rpg
    ↓
dsl/merger.go (合并)
    ↓
dsl/parser.go (解析)
    ↓
dsl/validator.go (验证)
    ↓
dsl/simulator.go (模拟)
    ↓
模拟报告 (问题检测)
```

---

## 6. 扩展开发

### 6.1 添加新命令

1. 在 `cmd/` 创建命令文件
2. 实现 `*cobra.Command`
3. 在 `cmd/registry.go` 注册

### 6.2 添加新 Agent

1. 在 `internal/agents/` 创建 Agent
2. 继承 `BaseAgent`
3. 实现 `Execute()` 方法
4. 在 `internal/agents/registry.go` 注册

### 6.3 添加 DSL 功能

1. 在 `dsl/ast.go` 定义 AST 节点
2. 在 `dsl/parser.go` 添加解析规则
3. 在 `dsl/validator.go` 添加验证规则
4. 在 `dsl/converter.go` 添加转换逻辑

### 6.4 添加约束规则

在 `constraint_system.go` 中：

```go
// 添加新约束类型
type MyConstraint struct {
    // 约束字段
}

// 在 BuildFromRPGData 中构建约束
func (cs *ConstraintSystem) buildMyConstraints() *MyConstraint {
    // 实现
}

// 在 ValidateChapter 中检查约束
func (cs *ConstraintSystem) validateMyConstraint(chapterID, content string) []ConstraintViolation {
    // 实现
}
```

---

## 7. 测试与基准

### 7.1 基准测试系统

位于 `internal/rpg/benchmark/`：

```bash
# 运行基准测试
novelgen rpg bench

# 运行指定测试用例
novelgen rpg bench --only power_collapse_frequent_breakthrough
```

**功能：**
- 测试用例生成（含难度分级）
- 跨章一致性检测
- 模糊测试
- 性能基准测试
- 指标收集与报告

### 7.2 AI→RPG 管道测试

```bash
# 检查小说项目
novelgen check-novel -b mine

# 输出：
# - AI 分析章节
# - 转换为 RPG DSL
# - 运行模拟
# - 生成问题报告
```

---

## 8. 配置说明

### 8.1 LLM 配置

```json
{
  "provider": "openai",
  "model": "gpt-4",
  "api_key": "...",
  "base_url": "..."
}
```

配置位置：
- 项目级: `llm_config.json`
- 全局级: `~/.novelgen/llm_config.json`

### 8.2 项目配置

```json
{
  "book_name": "mine",
  "chapter_count": 40,
  "genre": ["修仙", "穿越"],
  "language": "zh"
}
```

---

## 9. 相关文档

- [README.md](../README.md) - 主文档和命令参考
- [docs/DSL_RPG_INTEGRATION_SPEC.md](docs/DSL_RPG_INTEGRATION_SPEC.md) - DSL-RPG 集成规范
- [docs/RPG_DSL_SPEC.md](docs/RPG_DSL_SPEC.md) - DSL 规格文档
- [docs/RPG_CONSTRAINT_INTEGRATION.md](docs/RPG_CONSTRAINT_INTEGRATION.md) - 约束系统集成
- [docs/RPG_WRITE_USAGE.md](docs/RPG_WRITE_USAGE.md) - RPG 写作指南
- [docs/DSL_INCREMENTAL_WORKFLOW.md](docs/DSL_INCREMENTAL_WORKFLOW.md) - 渐进式工作流
- [docs/NOVELGEN_RPG_INTEGRATION.md](docs/NOVELGEN_RPG_INTEGRATION.md) - RPG 集成指南

---

## 10. 设计决策

### 10.1 为什么使用 DSL？

1. **AI 友好**: 类自然语言结构，降低生成难度
2. **精确可执行**: 每个指令有明确语义
3. **可验证**: 静态检查发现问题
4. **可扩展**: 添加新指令无需修改 schema

### 10.2 为什么引入 RPG 系统？

1. **数值化验证**: 量化检查故事一致性
2. **约束指导**: 避免战力崩坏等问题
3. **游戏化**: 为小说改编提供基础
4. **自动化**: 减少人工检查成本

### 10.3 为什么分阶段生成 DSL？

1. **信息渐进**: Outline → Craft → Systems
2. **可调试**: 每个阶段可独立验证
3. **增量更新**: 修改局部无需重新生成
4. **来源追踪**: 知道每个字段来自哪里

---

## 11. 性能考虑

- **并发支持**: draft/write 命令支持并发处理
- **缓存机制**: AI 响应可缓存避免重复调用
- **批量处理**: craft 支持批量生成元素
- **流式输出**: 大文件处理使用流式读写

---

## 12. 安全考虑

- **API Key 管理**: 支持本地和全局配置，避免提交到版本控制
- **输入验证**: DSL 解析器验证所有输入
- **错误处理**: 完善的错误恢复机制
- **日志记录**: 详细日志便于调试
