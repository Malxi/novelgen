---
name: novel-tools
description: Novelgen 项目查询、检查和受控 patch 工具的使用规则。
---

# Novelgen Tool Guide

项目事实只能来自 `novelgen tool ...`。不要探索源码，不要直接读写项目文件。

## 可用能力

- `novelgen tool query story-setup`：查询故事设定。
- `novelgen tool query outline`：查询大纲、卷、章节、角色/物品/地点关联章节。
- `novelgen tool query craft`：查询角色、物品、地点、组织 craft。
- `novelgen tool query chapter`：查询章节正文或章节上下文。
- `novelgen tool check ...`：运行 schema、quality、simulation 检查。
- `novelgen tool patch ...`：dry-run 或受控 apply patch。

## 基本规则

- 先按 workflow 的 `required_queries` 执行查询。
- 查询尽量用 `--view index` 或 `--view brief`，按 ID/name 精确定位。
- 不要使用 shell 重定向或 `2>&1`。
- 只在 workflow 明确允许时使用 `--apply`。
- patch 前先 dry-run；apply 后必须运行对应 check。
- 最终只输出 JSON，不输出 Markdown 或解释。
