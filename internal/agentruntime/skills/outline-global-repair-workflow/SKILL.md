---
name: outline-global-repair-workflow
description: 查询 outline/setup 的全局检查结果，按受限 patch task 小步修复，并只输出 JSON。
---

# 大纲全局修复 Workflow

你负责修复跨卷或全局一致性问题。目标是像改代码一样做一个小 diff：先读工具给出的路线，再 dry-run，再 apply，再跑指定检查。

## 必须遵守

- 先运行调用方给出的 required query。
- 如果返回结果里有 `patch_task`，优先使用它；不要再查询 `outline-volume`、`outline-repair`、`chapter-repair`、`query outline`、`query chapter`、源码、RPG 文件、完整 setup、完整 outline 或 Claude 临时 `tool-results`。
- 每次 invocation 最多处理一个 `patch_task`。
- 如果 `patch_task` 提供 `dry_run_command` 和 `apply_command`，直接运行这些命令；不要自己重组 patch 命令。
- dry-run 只做一次：运行 `patch_task.dry_run_command`。
- dry-run 成功后，只运行一次 `patch_task.apply_command`。
- apply 成功后，必须运行 `patch_task.post_patch_check_query` 的原始命令，不要换成 volume check、medium-only check 或其它 check。
- 跑完 post-check 后立刻返回最终 JSON，不要开始第二轮 patch，也不要继续探索上下文。
- 不要使用 `<json>`、`<compact-json>`、空 patch、临时文件、`Get-Content`、`type`、`findstr`、`echo test`、Python/Node/PowerShell 编码辅助命令。
- 对中文或复杂 JSON，不要使用 `--patch-json`；使用 stdin pipe。

## 没有 patch_task 时

- 只处理 `issue_context` 中同时有 `patch_query` 和 `patch_shape` 的问题。
- 如果没有可 patch 的问题，返回 `applied_patches=false`，并在 `review_result.suggestions` 中说明剩余问题为什么需要更小的工作流。

## 输出

最终只输出调用方要求的 JSON 字段：

- `review_result`
- `applied_patches`
- `applied_patch_count`
- `final_check`
