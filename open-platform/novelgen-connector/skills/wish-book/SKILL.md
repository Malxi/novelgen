---
name: wish-book
display_name: 小说管理
display_name_en: Novel Management
description: 管理 Novelgen 小说项目：创建、状态、读取、写入和刷新。
description_zh: 管理小说项目：创建、状态、读取、写入和刷新。
description_en: Manage Novelgen projects: create, status, read, write and refresh.
version: 0.1.0
author: Malxi
---

# Wish Book

WorkBuddy 负责创作；Novelgen 负责项目状态和受控写入。不要直接用编辑器修改
`story/`、`chapters/`、`drafts/` 或 `novel.json`。

## 工具

在项目根目录执行，或给命令加 `--dir <project-dir>`：

```bash
novelgen book status --json
novelgen book read setup --view index
novelgen book read outline --view index
```

创建项目：

```bash
novelgen book init "我的第一本书" --dir my-first-novel --genre 科幻 --parts 1 --volumes 3 --chapter 5 --language zh
```

`--chapter` 是每卷章节数。

## 写入循环

先 dry-run，检查返回的 `changed`、`check`、`warnings`，再 apply：

```bash
novelgen book write setup --mode replace --apply=false < setup.json
novelgen book write setup --mode replace --apply < setup.json

novelgen book write outline --mode replace --apply=false < outline.json
novelgen book write outline --mode replace --apply < outline.json

novelgen book write outline --mode patch --target volume --id P1-V1 --apply < volume-patch.json
novelgen book write chapter --id P1-V1-C1 --mode replace --apply < chapter.md
```

- 完整 setup/outline/chapter/recap 用 `--mode replace`。
- 局部修复用 `--mode patch`。
- `--apply` 才写盘；默认 dry-run。
- 整本 outline replace 会报告将被移除的章节；apply 前先报告给用户。
- 不要运行 `novelgen setup gen`、`compose pipeline`、`write pipeline`，它们会消耗 Novelgen 的 LLM 配置。
- `novelgen book refresh chapter-dsl` 会调用 Novelgen LLM，除非用户明确要求，否则跳过。

## 生成前

先用：

```bash
novelgen gen workflow read setup
novelgen gen reference read setup-schema
novelgen gen workflow read outline
novelgen gen reference read outline-schema
```

然后加载对应的 `gen skill`。
