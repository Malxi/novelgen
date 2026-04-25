# RPG-DSL 系统文档索引

> 版本: 0.3.0  
> 最后更新: 2026-04-24

---

## 概述

RPG-DSL（Role-Playing Game Domain-Specific Language）是 Novelgen 的核心创新，将小说内容转化为可执行的游戏化数据结构，实现智能化的内容验证和推演。

---

## 文档导航

### 入门文档

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [NOVELGEN_RPG_INTEGRATION.md](NOVELGEN_RPG_INTEGRATION.md) | RPG 集成指南，介绍基本概念和使用方法 | 新手用户 |
| [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) | RPG 增强写作使用指南 | 内容创作者 |

### 技术规范

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) | DSL-RPG 集成规范，完整的架构设计和数据流 | 架构师/开发者 |
| [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) | DSL 规格文档，详细的语法定义和函数库 | DSL 开发者 |
| [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md) | 渐进式生成工作流，分阶段 DSL 生成策略 | 高级用户 |

### 集成文档

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [RPG_CONSTRAINT_INTEGRATION.md](RPG_CONSTRAINT_INTEGRATION.md) | 约束系统集成方案，RPG 约束指导写作 | 开发者/内容创作者 |
| [rpg-compose-integration.md](rpg-compose-integration.md) | RPG 与 Compose 大纲系统集成 | 开发者 |

---

## 快速开始

### 1. 了解基本概念

阅读 [NOVELGEN_RPG_INTEGRATION.md](NOVELGEN_RPG_INTEGRATION.md) 了解：
- RPG 系统是什么
- 为什么要将小说转化为 RPG 数据
- 基本使用方法

### 2. 学习 DSL 语法

阅读 [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) 学习：
- DSL 语法规范
- 各种块定义（metadata, world, characters, storyline, systems）
- 内置函数库

### 3. 理解集成架构

阅读 [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) 了解：
- 完整的系统架构
- 数据流转过程
- 模块职责划分

### 4. 实践 RPG 写作

阅读 [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) 学习：
- 如何使用 RPG 约束指导写作
- 约束验证流程
- 最佳实践

---

## 核心概念

### DSL 文件结构

```
books/<book>/story/rpg/
├── 01_outline.rpg      # Phase 1: 大纲框架
├── 02_craft.rpg        # Phase 2: 详细信息
├── 03_systems.rpg      # Phase 3: 系统规则
└── final.rpg           # 合并后的完整 DSL
```

### DSL 示例

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
            defense = 10
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

### 处理流程

```
AI 分析 → DSL 文件 → Parser → Validator → Converter → Simulator → 报告
```

---

## 主要功能

### 1. DSL 解析与验证

- **Parser**: 解析 DSL 文本为 AST
- **Validator**: 验证引用、类型、完整性
- **错误定位**: 精确到行号的错误报告

### 2. RPG 转换

- **Converter**: DSL → RPG World 对象
- **Adapter**: Novelgen 数据 ↔ DSL 双向转换
- **Merger**: 合并多个 DSL 片段

### 3. 模拟推演

- **Simulator**: 执行剧情推演
- **事件系统**: 战斗、对话、收集等
- **Hook 系统**: 事件触发和计数器

### 4. 约束系统

- **CharacterConstraint**: 角色死亡/复活限制
- **PlotConstraint**: 剧情节奏控制
- **PowerSystemConstraint**: 战力系统平衡

### 5. 基准测试

- **Benchmark**: 自动化测试用例
- **Metrics**: 指标收集和报告
- **Cross-Chapter**: 跨章一致性检测

---

## 使用场景

### 场景 1: 内容验证

```bash
# 转换小说为 RPG DSL
novelgen rpg dsl convert -b mine

# 运行模拟检测问题
novelgen simulate-dsl books/mine/story/rpg/final.rpg

# 查看报告
cat simulation_report.json
```

### 场景 2: 约束写作

```go
// 创建 RPG 增强写作 Agent
agent := agents.NewRPGEnhancedWriteAgent(...)

// 生成章节（自动应用约束）
content := agent.GenerateChapter(ctx, chapter, ...)
```

### 场景 3: 基准测试

```bash
# 运行基准测试
novelgen rpg bench

# 运行指定测试用例
novelgen rpg bench --only power_collapse_frequent_breakthrough
```

---

## 开发指南

### 添加新的 DSL 功能

1. 在 `dsl/ast.go` 定义 AST 节点
2. 在 `dsl/parser.go` 添加解析规则
3. 在 `dsl/validator.go` 添加验证规则
4. 在 `dsl/converter.go` 添加转换逻辑
5. 在 `dsl/simulator.go` 添加模拟逻辑

### 添加新的约束类型

1. 在 `constraint_system.go` 定义约束结构
2. 实现 `BuildFromRPGData()` 构建逻辑
3. 实现 `ValidateChapter()` 检查逻辑
4. 在提示词中添加约束描述

### 添加新的模拟事件

1. 在 `simulation.go` 定义事件类型
2. 实现事件处理函数
3. 在 DSL 中添加事件语法支持
4. 更新 Parser 和 Converter

---

## 常见问题

### Q: DSL 和 JSON 有什么区别？

A: DSL 是类自然语言的声明式语法，更适合 AI 生成和人类阅读。JSON 是机器格式，嵌套复杂，AI 生成容易出错。

### Q: 为什么要分阶段生成 DSL？

A: 因为信息是渐进的。Outline 阶段只有框架，Craft 阶段才有详细信息。分阶段可以独立验证和增量更新。

### Q: 约束系统如何工作？

A: 从 RPG 数据提取规则（如主角最多复活 3 次），转换为 AI 提示词，写作时 AI 遵守这些规则，写作后自动验证。

### Q: 模拟能检测什么问题？

A: 战力崩坏、角色死亡滥用、时间线混乱、节奏失衡、逻辑矛盾等。

---

## 相关资源

- [ARCHITECTURE.md](../ARCHITECTURE.md) - 项目架构总览
- [README.md](../README.md) - 主文档和命令参考
- `internal/rpg/dsl/` - DSL 引擎源码
- `internal/rpg/benchmark/` - 基准测试源码
- `internal/agents/rpg_enhanced_write_agent.go` - RPG 增强写作 Agent

---

## 更新日志

### v0.3.0 (2026-04-24)
- 整理文档索引
- 统一文档格式
- 添加快速开始指南

### v0.2.0 (2026-04-20)
- 添加渐进式工作流文档
- 完善 DSL 规格
- 添加约束系统集成方案

### v0.1.0 (2026-04-19)
- 初始 DSL 设计
- 基础解析器和验证器
- RPG 集成方案
