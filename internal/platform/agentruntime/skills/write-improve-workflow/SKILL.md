---
name: write-improve-workflow
description: 使用 chapter-repair 上下文、章节检查和受控 patch 工具改进单章正文。
---

# Chapter Improve Workflow

你负责修复单章正文。项目事实只能来自 `novelgen tool ...`，最终只输出调用方要求的 JSON。

## 必须执行的入口

把 `<chapter_id>` 替换为当前任务章节 ID，例如 `P1-V1-C1`。

1. 先查询修复上下文：
   `novelgen tool query context --type chapter-repair --id "<chapter_id>" --view brief`
2. 再运行章节检查：
   `novelgen tool check all --target chapter --scope chapter --id "<chapter_id>" --min-priority low --max-issues 12`
3. 如果检查或 context 要求刷新派生 DSL，且你还没有执行过带 `--refresh-derived` 的 apply，运行：
   `novelgen tool refresh chapter-dsl --id "<chapter_id>"`
4. 需要更多事实时，只使用 context 返回的 `navigation`、`workflow`、`next_actions` 中的命令，或精确的 craft 查询：
   `novelgen tool query context --type chapter-write --id "<chapter_id>" --view brief`
   `novelgen tool query chapter --id "<chapter_id>" --content --view brief`
   `novelgen tool query context --type craft-character --name "<name>" --view brief`
   `novelgen tool query context --type craft-item --name "<name>" --view brief`
   `novelgen tool query context --type craft-location --name "<name>" --view brief`
   不存在 `craft-ability` 查询；能力/体质/技能只能从 chapter-repair、chapter-write、角色 craft、物品 craft 或地点 craft 的已返回事实中核验。
5. 只有输入里的 `history_mode` 非空，才查询历史 logs。此时先运行：
   `novelgen tool query logs --view index --limit 5`
   默认只使用 index 里的摘要；如果没有 agent-live，优先参考 prompts/responses 的创作历史。只有索引明确显示某条已完成旧运行高度相关时，最多读取 1 条 exact brief，不要读取 content。历史只用于理解上一次创作尝试、风格约束和失败教训；不要复制旧输出，不要把命令记录写进正文，不要因为历史而扩写目标篇幅。

## 禁止猜命令

不要运行这些错误形态：

- `novelgen tool chapter-context ...`
- `novelgen tool query --target chapter-context ...`
- `novelgen tool check --target chapter ...`
- `novelgen tool check chapter ...`
- `type chapters\chapter-...md`
- `cat chapters/chapter-...md`
- `Get-Content chapters\chapter-...md`

`tool check` 必须带 check kind：`quality`、`simulation` 或 `all`。

## Patch 规则

- 只修复当前章节，不改 outline、craft、setup 或其他章节。
- 如果 `tool check all` 返回干净，且用户请求是条件式、探索式，或明确说“无需修复则保持原文”，立即返回最终 JSON，不要 patch。
- 如果用户请求包含“如果人物/能力/物品/设定不属于当前项目事实则不要修改”、或等价条件，必须先用 `chapter-repair`/`chapter-write` 上下文和必要的精确 craft 查询核验这些实体。若核验失败，先确认已经完成入口要求的 `tool check all`，然后返回当前正文，不要 patch；不要为了 length/style/check 中的无关问题顺手修稿。
- 只有用户/建议明确点名具体缺陷、错词、替换目标或必须改的段落时，才可以在 clean check 后继续做最小 patch。
- 默认做最小修复：只改能解决当前 check/review 问题的段落，不为凑目标字数扩写新场景。只有问题明确是“过短、字数不足、需要扩写”时，才补足篇幅。
- 对非 length 类问题（如 pacing、style、character、structure、logic）做 patch 时，保持旧正文主体规模，通常保留旧正文 narrative units 的 85% 以上；不要用大幅删减来换取检查通过。
- 对 system log / 信息差题材，“获得日志线索、形成可执行判断、确认策略优势”也是主角成长；不要为了满足成长检查硬塞修炼突破、道具或盟友。
- 工具返回结果已经显示在对话里，不要再读 Claude 临时 `tool-results`。不要直接读写章节文件，不要使用编辑器、管道、重定向、`2>&1`、临时文件、源码搜索、`type`、`cat`、`Get-Content`、`head`、`Select-String`、`echo/printf` 输出 JSON 或 shell 包装。只有只读 query/check 结果明显乱码时，才可重试一次 runtime 允许的 UTF-8 PowerShell 包装；不得用于 patch、正文或最终 JSON。
- 成功拿到 `chapter-repair`、`chapter-write` 或 check 结果后，不要再次查询、截断、过滤或 shell 摘要这个结果；直接用对话中的工具结果修复。
- 没有先完成 `chapter-repair` 查询和 `tool check all` 时，不要 patch，也不要输出最终正文。
- 如果调用方要求了 `target_words`，必须先完成包含 `--target-words <N>` 的精确章节检查；之后每一次 `tool patch chapter` dry-run 和 apply 也都必须带同一个 `--target-words <N>`。`patch-buffer clear/append` 不带这个参数。
- patch 前先 dry-run；无 `target_words` 时用 `novelgen tool patch chapter --id "<chapter_id>"`，有 `target_words=<N>` 时用 `novelgen tool patch chapter --id "<chapter_id>" --target-words <N>`。
- 长章节正文使用固定 patch buffer ID：`<chapter_id>-draft`。
- patch buffer 的可用命令是：
  `novelgen tool patch-buffer clear --id "<chapter_id>-draft"`
  `novelgen tool patch-buffer append --id "<chapter_id>-draft" --stdin`
  `novelgen tool patch chapter --id "<chapter_id>" --patch-buffer "<chapter_id>-draft"`
- `patch-buffer append --stdin` 的正文块应作为该工具调用的 stdin/input 提交；不要用 `printf`、`echo`、PowerShell 字符串、管道或临时文件来承载正文块、计划 JSON 或状态信息。
- 每个 content chunk 尽量控制在 900-1600 个中文字符；长正文分块 append，目标是 4-8 个 chunk 内完成整章 patch，避免过多工具回合。不要把整章塞进一个 `--text` 参数；`--text` 只适合很短且不含引号、竖线、分号、换行的片段。
- buffer 内容不要重复 markdown 标题。章节工具会自动保留一个 `# <章节标题>`，正文第一段直接从场景 prose 开始。
- `apply_patches=true` 时，dry-run 成功后才能用同一 patch 加 `--apply --refresh-derived`；如果有 `target_words=<N>`，apply 命令同样必须带 `--target-words <N>`。该命令会写入章节、刷新目标章节 RPG DSL，并返回 post-refresh all check。检查仍有 blocking/high 问题时继续修复；检查干净后再输出最终 JSON。
- 成功执行 `--apply --refresh-derived` 后，先读取该工具结果里的 `check` 和 `next_actions`。如果 `next_actions.action` 是 `return_final_json` 或 check 已干净，立刻输出最终 JSON；不要再单独运行 `tool refresh chapter-dsl`、不要再查章节正文、不要读 Claude 临时 tool-results。只有工具结果明确返回 `repair_remaining_issues` 时才继续修复。
- 不要用 Bash/printf/echo 输出步骤计划、状态 JSON、解释或最终 JSON；这些内容只应作为最终 assistant response 返回。
- 不要使用 `--patch-json` 传中文长正文；不要运行 `--help` 猜参数。
- 目标篇幅以调用方输入 `target_words` 为硬预算；历史日志、未来伏笔或世界观解释不能扩大本次修复范围。
- 按调用方给出的段落预算控制改稿长度：用紧凑段落合并事件，不要把每个 beat 展开成独立长场景。只有明确修复“过短”时才扩写。

## 输出

最终只作为 assistant response 输出 JSON，且只保留 schema 允许的字段，通常就是 `content`。不要输出 `chapter_id`、`title`、`word_count`、`notes` 或其它元数据；不要用 Bash、`echo`、`printf` 或文件写入来输出 JSON。不要输出 Markdown、解释、命令记录或代码块。
## Apply Patch Override

In `apply_patches=true` mode, a clean check does not cancel an explicit user edit request. If suggestions or user instructions ask for prose changes, perform a minimal patch dry-run and then the matching `--apply --refresh-derived` command before final JSON.
