---
name: outline-improve-volume-workflow
description: 单卷大纲改进 workflow，通过 query/check/patch 工具局部修复目标卷，不直接写项目文件。
---

# 单卷大纲改进 Workflow

你负责改进调用方指定的一个 volume。目标是像改代码一样做小步可验证修改：先读工具给出的上下文或检查结果，再 dry-run，再 apply，再跑同一目标的检查。

## 必须遵守

- 先运行调用方给出的 `required_queries`。
- 如果没有明确的用户改进请求或 focused issue，先运行目标卷 outline check；如果 `summary.total=0`，立即返回 `applied_patches=false`，不要继续查 brief、章节、事件或源码。
- 如果 check 或 context 给出了 `next_actions`、`patch_query`、`patch_shape`，只处理第一条 patchable issue。
- 对 mysteries 这类可能跨卷的问题，优先信任 `tool check` 或 context；不要只因为目标卷内看不到 plant 就自行断定“未 planted”。如果 scoped check 为 clean，立刻返回。
- patch 只能使用 `novelgen tool patch outline --target volume --id "<volume_id>"`。
- 不要使用 `tool patch outline --target chapter`，也不要直接写 `story/compose/outline.json`、`outline.md` 或任何项目文件。
- 每次 invocation 最多一个 patch cycle：一次 dry-run，同一份 JSON 一次 apply，随后一次对应的 check。
- 中文或复杂 JSON 必须用 stdin pipe 传给 patch 命令；不要使用 `--patch-json`、`<json>`、`<compact-json>`、临时文件、`Get-Content`、`type`、`findstr`、`echo test`、Python/Node/PowerShell helper，或 shell redirection `2>&1`。
- 不要读取源码、story 文件、RPG 文件、Claude 临时 `tool-results`。
- 跨卷连续性事实只允许读取相邻卷的 `payoff_contract/summary`（调用方已把对应命令加入允许列表）；禁止查询非相邻卷，禁止 patch 目标卷以外的任何卷。

## 没有可修复问题时

- 如果检查没有问题，返回 `review_result.score=100` 或调用方要求的等价 clean 结果。
- `suggestions` 保持简短，说明本轮没有需要 patch 的目标卷问题。
- 不要为了“显得有工作量”继续探索其它卷或全局上下文。

## 输出

最终只输出调用方要求的 JSON。

- apply 模式下输出 `review_result`、`applied_patches`、`applied_patch_count`、`final_check`。
- 非 apply 模式下按调用方 schema 返回审阅结果。
- 最终 JSON 不要返回完整 `volume_patch`，也不要返回完整 volume 或 outline。
