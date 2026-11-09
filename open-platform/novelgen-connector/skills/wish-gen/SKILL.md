---
name: wish-gen
display_name: 创作指导
display_name_en: Generation Guidance
description: 按阶段读取 Novelgen workflow、skill 和 schema reference，指导 setup、outline、craft、write 和 review。
description_zh: 按阶段读取 workflow、skill 和 schema reference，指导设定、大纲、craft、正文和审查。
description_en: Load Novelgen workflows, skills and schema references for setup, outline, craft, write and review.
version: 0.1.0
author: Malxi
---

# Wish Gen

这是只读指导；真正写作由 WorkBuddy 的模型完成。

```bash
novelgen gen workflow list
novelgen gen workflow read setup
novelgen gen workflow read outline
novelgen gen workflow read write
novelgen gen skill list
novelgen gen skill read outline-compose-volume-workflow
novelgen gen reference list
novelgen gen reference read setup-schema
novelgen gen reference read outline-schema
```

阶段对应：

| 阶段 | workflow | skill |
| --- | --- | --- |
| setup | `setup` | `setup-generate-workflow` / `setup-improve-workflow` |
| outline 骨架 | `outline` | `outline-compose-skeleton-workflow` |
| outline 单卷 | `outline` | `outline-improve-volume-workflow` |
| craft | — | `craft-character-workflow` / `craft-element-workflow` |
| write | `write` | `write-chapter-workflow` / `write-improve-workflow` |
| review | `review` | `write-review-workflow` |
| refresh | `refresh` | — |

第一次写 setup 前必须读 `setup-schema`；第一次写 outline 前必须读
`outline-schema`。重点：

```text
premise            string
premises           []object
writing_style      object
world_resources    []object
outline.parts      []object，唯一顶层字段
chapter.events     []object
```

不要运行 Novelgen 的 `setup gen` / `compose pipeline` / `write pipeline`；不要为了
找字段去读 Go 源码，先读 schema reference。
