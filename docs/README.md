# Novelgen 技术文档

> 最后更新: 2026-04-24

---

## 📚 文档导航

### 入门文档

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [../README.md](../README.md) | 主文档，命令参考和工作流程 | 所有用户 |
| [../ARCHITECTURE.md](../ARCHITECTURE.md) | 项目架构总览 | 开发者/架构师 |
| [../DEVELOPER_GUIDE.md](../DEVELOPER_GUIDE.md) | 开发者指南 | 开发者 |
| [RPG_DSL_DOCUMENTATION_INDEX.md](RPG_DSL_DOCUMENTATION_INDEX.md) | RPG-DSL 系统文档索引 | RPG-DSL 用户 |

### RPG-DSL 系统

| 文档 | 说明 | 适合人群 |
|------|------|----------|
| [NOVELGEN_RPG_INTEGRATION.md](NOVELGEN_RPG_INTEGRATION.md) | RPG 集成指南，介绍基本概念和使用方法 | 新手用户 |
| [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) | RPG 增强写作使用指南 | 内容创作者 |
| [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) | DSL-RPG 集成规范，完整的架构设计和数据流 | 架构师/开发者 |
| [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) | DSL 规格文档，详细的语法定义和函数库 | DSL 开发者 |
| [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md) | 渐进式生成工作流，分阶段 DSL 生成策略 | 高级用户 |
| [RPG_CONSTRAINT_INTEGRATION.md](RPG_CONSTRAINT_INTEGRATION.md) | 约束系统集成方案，RPG 约束指导写作 | 开发者/内容创作者 |
| [rpg-compose-integration.md](rpg-compose-integration.md) | RPG 与 Compose 大纲系统集成 | 开发者 |

### 历史记录

| 文档 | 说明 |
|------|------|
| [../IMPROVEMENTS.md](../IMPROVEMENTS.md) | RPG-Compose 集成改进总结 |
| [../IMPROVEMENT_SUMMARY.md](../IMPROVEMENT_SUMMARY.md) | Novelgen + RPG 集成改进总结 |

---

## 🚀 快速开始

### 我是新用户，应该看什么？

1. 阅读 [../README.md](../README.md) 了解基本用法
2. 运行快速开始示例创建第一个项目
3. 根据需要查看相关文档

### 我想了解 RPG-DSL 系统，应该看什么？

1. 阅读 [RPG_DSL_DOCUMENTATION_INDEX.md](RPG_DSL_DOCUMENTATION_INDEX.md) 了解 RPG-DSL 概览
2. 阅读 [NOVELGEN_RPG_INTEGRATION.md](NOVELGEN_RPG_INTEGRATION.md) 了解基本概念
3. 阅读 [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) 学习 DSL 语法
4. 阅读 [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) 了解架构设计

### 我想参与开发，应该看什么？

1. 阅读 [../ARCHITECTURE.md](../ARCHITECTURE.md) 了解项目架构
2. 阅读 [../DEVELOPER_GUIDE.md](../DEVELOPER_GUIDE.md) 了解开发规范
3. 阅读 [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) 了解 DSL 系统设计
4. 查看源码 `internal/` 目录

---

## 📖 文档分类

### 用户文档

面向使用 Novelgen 创作内容的用户：

- [../README.md](../README.md) - 命令参考
- [NOVELGEN_RPG_INTEGRATION.md](NOVELGEN_RPG_INTEGRATION.md) - RPG 基础
- [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) - RPG 写作

### 架构文档

面向理解系统设计的架构师和高级开发者：

- [../ARCHITECTURE.md](../ARCHITECTURE.md) - 项目架构
- [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) - DSL-RPG 集成
- [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md) - 渐进式工作流

### 技术规范

面向实现和维护系统的开发者：

- [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) - DSL 规格
- [RPG_CONSTRAINT_INTEGRATION.md](RPG_CONSTRAINT_INTEGRATION.md) - 约束系统
- [rpg-compose-integration.md](rpg-compose-integration.md) - Compose 集成

### 开发指南

面向贡献代码的开发者：

- [../DEVELOPER_GUIDE.md](../DEVELOPER_GUIDE.md) - 开发规范
- 源码注释 - 各模块实现细节

---

## 🎯 按主题查找文档

### 命令行使用

- [../README.md](../README.md) - 完整命令列表和参数
- [../ARCHITECTURE.md](../ARCHITECTURE.md) §4.1 - CLI 命令系统

### AI Agent

- [../ARCHITECTURE.md](../ARCHITECTURE.md) §4.2 - Agent 系统
- `internal/agents/` - 源码和注释

### RPG-DSL

- [RPG_DSL_DOCUMENTATION_INDEX.md](RPG_DSL_DOCUMENTATION_INDEX.md) - 文档索引
- [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) - DSL 语法
- [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md) - 集成架构

### 约束系统

- [RPG_CONSTRAINT_INTEGRATION.md](RPG_CONSTRAINT_INTEGRATION.md) - 约束方案
- [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) - 约束使用指南
- `internal/rpg/constraint_system.go` - 源码

### 基准测试

- `internal/rpg/benchmark/` - 基准测试源码
- [../ARCHITECTURE.md](../ARCHITECTURE.md) §7.1 - 基准测试系统

### 连续性检查

- [../ARCHITECTURE.md](../ARCHITECTURE.md) §4.5 - 连续性系统
- `internal/logic/continuity/` - 源码

---

## 📝 文档更新指南

### 添加新文档

1. 在 `docs/` 目录创建 Markdown 文件
2. 更新本文档索引
3. 在 [../README.md](../README.md) 添加链接（如适用）

### 更新现有文档

1. 保持格式一致性
2. 更新版本号和日期
3. 确保链接有效

### 文档格式规范

- 使用 H1 (`#`) 作为标题
- 使用 H2 (`##`) 作为主要章节
- 使用 H3 (`###`) 作为子章节
- 代码块指定语言
- 表格对齐列宽

---

## 🔗 外部资源

- [Go 文档](https://golang.org/doc/) - Go 语言参考
- [Cobra 文档](https://github.com/spf13/cobra) - CLI 框架
- [OpenAI API](https://platform.openai.com/docs/) - LLM API 参考

---

## 📅 更新日志

### 2026-04-24
- 创建文档索引
- 整理文档分类
- 添加快速开始指南

### 2026-04-20
- 添加 DSL 相关文档
- 更新集成方案

### 2026-04-19
- 初始文档创建
- RPG-Compose 集成文档
