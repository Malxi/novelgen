---
name: recap-extract-workflow
description: 从章节正文抽取 recap，并在 apply_patches=true 时通过 novelgen tool patch recap 受控写入。
---

# Recap 抽取 Workflow

你负责从当前章节正文抽取结构化 recap。只使用章节上下文 query 和 recap check/patch 工具。

## 规则

- `apply_patches=false` 时不要写入，只返回 JSON。
- `apply_patches=true` 时，必须先 dry-run `novelgen tool patch recap`，成功后再用同一 patch 加 `--apply`。
- apply 后必须运行 recap/schema check。
- 不要查询 outline/events/craft 之外的无关内容；如果 workflow 已给章节正文，不要重复查询全文。
- 不要运行 shell 编码、文件读写或源码搜索。
- `last_line` 必须来自章节最后一句或最后一幕；`next_opening_hint` 必须直接承接它，并复用至少一个具体名词、角色、地点或意象，避免写成新的剧情建议。
- 尽量一次通过 minimal gate；consistency warning 不会自动触发第二次抽取。
- 保持 recap 紧凑：`plot_beats` 不超过 8 条，`decisions` 不超过 5 条，`reveals` 不超过 6 条，`unresolved` 不超过 5 条，`promises` 不超过 4 条，`items`/`status` 不超过 6 条。

## 输出

只返回调用方 schema 要求的 JSON。不要输出 Markdown、解释、章节概览、验证清单、命令记录或额外文本。recap 必须包含章节 ID、标题、地点、时间、出场人物、关键情节、决定、揭示、未解决事项、承诺、物品状态、结尾句和下一章提示。
