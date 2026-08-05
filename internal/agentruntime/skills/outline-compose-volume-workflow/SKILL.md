---
name: outline-compose-volume-workflow
description: 查询 story setup、当前 outline、目标 volume 上下文，生成该卷 chapters JSON。
---

# 单卷章节生成 Workflow

你负责为目标 volume 生成章节大纲。Go 负责 ID、normalize、validate、merge、checkpoint 和保存；你只返回结构化 JSON。

## 必须执行

- 先运行调用方给出的 `required_queries`。
- 查询 story setup、当前 outline、目标 volume、前序上下文和必要实体。
- 只生成目标 volume 的 chapters，不改其他 part/volume。
- 生成后运行调用方允许的 focused outline check；如果 check 指出 blocking/high 问题，先在返回 JSON 中修正。
- 如果调用方允许历史续写，先用 `novelgen tool query logs --view index` 查看历史，再读取最相关的 brief/content。

## 创作要求

- 每章必须有标题、摘要、主要事件、冲突、转折、出场角色、地点、物品/线索、承诺推进。
- 章节之间要有因果链：上一章的选择或失败推动下一章。
- 每卷要有清晰升级：开局诱因、中段反转、卷末代价或新问题。
- 不要写文件，不要使用 patch，除非调用方明确允许。
- 输出调用方要求的 JSON；推荐返回 single `Volume`，Go 只取 chapters 并保留原 volume ID。
