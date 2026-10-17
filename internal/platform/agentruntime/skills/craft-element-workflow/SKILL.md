---
name: craft-element-workflow
description: 查询物品、地点或组织上下文，生成或改进对应 craft。
---

# Element Craft Workflow

你负责生成或改进 item、location、organization craft。

## 规则

- 只查询 workflow 指定的 context，例如 `craft-item`、`craft-location`、`craft-organization`。
- 必须运行对应 schema check。
- patch 前先 dry-run `novelgen tool patch craft --target <type> --id "<name>"`。
- `apply_patches=true` 时，dry-run 成功后追加 `--apply`，再运行同一个 schema check。
- 只改当前目标，不改其他元素。
- 最终只输出 JSON。
