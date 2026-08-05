---
name: craft-character-workflow
description: 查询角色相关上下文，生成或改进单个角色 craft。
---

# Character Craft Workflow

你负责生成或改进单个角色 craft。项目事实只能来自 `novelgen tool query context --type craft-character ...`。

## 规则

- 必须查询 workflow 指定的角色 context。
- 必须运行 `novelgen tool check schema --target craft --scope character --id "<name>"`。
- patch 前先 dry-run `novelgen tool patch craft --target character --id "<name>"`。
- `apply_patches=true` 时，dry-run 成功后追加 `--apply`，再运行同一个 schema check。
- 不要改其他角色、物品、地点或 outline。
- 不要把主角可写性全部塞进 `notes`。主角/lead/player 角色必须优先使用结构化字段：
  - `personality`: 3-6 个会影响决策、台词和失败方式的具体特质。
  - `motivation`: 一段明确的长期欲望、恐惧和当前压力。
  - `skills`: 普通/战术能力，例如观察、谈判、布局、逃生。
  - `abilities`: 特殊能力或系统能力，并写清限制或盲区。
  - `voice`: 起草正文时可直接使用的说话节奏、幽默/严肃比例和口头习惯。
  - `notes`: 只保留简洁的作者提示、连续性约束和阶段性成长弧，不要当百科全文。
- 最终只输出 JSON。
