---
name: novel-tools-core
description: Common Novelgen Agent SDK tool rules. Project facts may only be queried, checked, or patched through novelgen tool commands.
---

# Novelgen Core Tool Rules

你在 novelgen 项目目录中工作。项目事实只能通过 `novelgen tool ...` 获取或修改；不要读取源码，也不要直接读写项目文件。

## Command Rules

- 只运行 workflow、`required_queries`、`navigation`、`workflow` 或 `next_actions` 指定或允许的 `novelgen tool ...` 命令。
- 默认命令必须以 `novelgen tool` 开始；不要使用绝对路径、`cd`、源码搜索、测试、build、临时文件、重定向或 shell 包装。
- 如果 Windows 下只读 `query`/`check` 结果明显乱码，可重试一次 runtime 允许的 UTF-8 PowerShell 包装；不要把 PowerShell 用于 patch、正文、JSON、文件读取或编码辅助。
- 不要添加 `2>&1`、`>`、`<`、`| grep`、`Out-File`、`Get-Content`、`Select-String`、`cat` 或 `type`。
- `tool query context` 的形态是：`novelgen tool query context --type <context-type> ...`。
- `tool query ideas` 生成随机的“结构化废话”，用于 setup/outline 构思；它是只读的，不依赖已有 setup。把每条结果当作必须合理化的约束，而不是成品设定。
- 需要成套相容设定时优先使用 `--type worldpack`：同一结果的 premise、storyline、cast、resource 共享 `pack_id`、同一个 Engine 和同一个 `reference_id`。也可用相同 genre/seed/count/source 分别抽 `premise|storyline|cast|resource`，再按 `pack_id` 对齐；不要把不同 pack_id 或不同 reference 的卡片直接拼接。
- `tool check` 的形态是：`novelgen tool check <quality|simulation|all> --target <target> ...`。不要省略 check kind。
- 中文或非 ASCII patch JSON 优先走 stdin；允许的管道只有两种：把 compact JSON 传给 patch 工具：`printf '%s' '<compact-json>' | novelgen tool patch ...`；或把长章节正文分块传给 patch buffer：`printf '%s' '<content chunk>' | novelgen tool patch-buffer append --id "<chapter_id>-draft" --stdin`。
- 输出过大时使用 `--view index|brief`、`--limit`、`--fields` 或精确 ID/name；默认不要使用 `--view full`。

## Query And Check

- `tool query` 和 `tool check` 是只读操作；优先使用 context bundle。
- 需要打破套路时可运行 `novelgen tool query ideas --type axiom|event|law|name --genre <genre> --count <n> --mode wild --seed <seed>`。`law` 生成“什么被守恒/转移”的系统性世界法则，优先用于 setup。逐条回答 `must_answer`，拒绝 `forbidden_explanations`，并遵守 `output_contract`。
- `engine` 返回制度引擎；`worldpack` 返回完整相容包。需要世界前提、剧情线、人物与资源时，优先抽取 `worldpack` 而不是分别无约束抽词。
- `--type reference` 返回带来源链接与访问日期的研究型参考卡；`--source` 支持群星、战锤、异形、星战、无人深空。参考卡只提供结构机制和语气，生成时必须使用原创专名、阵营和情节。
- `--type namecard` 返回“先原型、后名字”的角色卡：先抽 archetype、性格矛盾、恐惧与说话方式，再生成姓名；用 `speech_style`、`body_language` 和 `backstory_hook` 做角色扮演，不要只取姓名。
- `--type itemcard` 返回资源、商品、装备、消耗品、剧情道具、载具或凭证；`--category` 支持这些类别及中文别名。每张卡必须检查 function、use_cost、limit、counterplay 和 failure_mode，避免只抽到装饰性道具。
- `--view index` 只给路由信息（id/title/count），不会携带章节 summary；需要事实时再升级到 `--view brief`。`--limit` 只作用于列表结果，对象结果会返回 warning。
- `tool check schema --target craft --id "<name>"` 会从 craft 文件中推断类型；不传 scope/id 时检查全部四类 craft。`tool check --target recap --id "<chapter_id>"` 默认使用 chapter scope。
- 有 `navigation`、`workflow`、`next_actions` 时按最小下一步执行。
- 单章修复任务优先使用 `novelgen tool query context --type chapter-repair --id "<chapter_id>" --view brief`。
- 章节生成任务只能使用 `novelgen tool query context --type chapter-write --id "<chapter_id>" --fields path,navigation,stats,warnings --view brief`；正文事实来自调用方 typed input，不要再查询普通 brief/full 或已保存章节正文。
- 只有章节 review/improve/repair 等明确需要检查已有草稿的任务，才使用 `novelgen tool query chapter --id "<chapter_id>" --content --view brief`；brief 现在会返回 `content` 和 `content_path`，不要用 shell 读 `chapters/` 文件。`tool query chapter --type events --id "<chapter_id>"` 也按章节过滤事件。
- 不要一次扫描完整 setup、outline 或所有章节。
- 有 `meta.context_budget` 或 `check_budget` 时必须遵守；不要违反 `avoid`。
- `tool check all` 合并 deterministic quality 和 simulation；outline/setup simulation 由 Go 内存模型执行，不调用 LLM，不依赖 `story/rpg/01_outline.rpg`。

## Log Context

- 如果任务要求从历史创作记录继续，先使用 `novelgen tool query logs --view index`。
- 使用 `novelgen tool query logs --type prompts|responses|agent-live --name <agent> --view brief --limit 5` 获取小型历史预览。
- Agent SDK 运行优先看 `novelgen tool query logs --view index --limit 5`。`agent-live` 的 index/brief 会返回结构化 `summary`，包含 `model`、`final_model`、`sdk_skills`、tool 调用数、`query_calls`、`check_calls`、`patch_calls`、`patch_applies`，以及脱敏后的 `allowed_tool_commands` / `denied_tool_commands`。
- 只有从 index 选中一个精确日志时，才使用 `novelgen tool query logs --id <relative_log_path> --content --view brief`；内容会被工具截断。
- 不要用 shell 命令读取 `logs/`，包括 `Get-Content`、`cat`、`grep`、`Select-String`、重定向或临时文件。

## Patch And Apply

- `tool patch ...` 默认 dry-run；Go 负责 merge、保留 ID、normalize、schema/quality/simulation 校验，失败即拒绝。typed patch 会拒绝未知字段；空对象会被拒绝；`changed` 为空时会返回 no-change warning，不能当作写入成功。
- 不要运行 Python、Node、PowerShell 或 help 命令来编码 JSON；`--patch-json` 只用于很短的 ASCII-only patch。
- 长章节正文不要放进一个巨大的 `--text` 参数；优先使用 `patch-buffer clear`，再用 `printf '%s' '<content chunk>' | novelgen tool patch-buffer append --id "<chapter_id>-draft" --stdin` 分块追加，最后用 `tool patch chapter --patch-buffer` dry-run/apply。
- 只有当前 workflow 明确允许 apply，例如 `apply_patches=true` 且 allowlist 允许时，才可以使用 `--apply`。
- apply 前必须先对同一目标、同一 patch 成功执行 dry-run；`--apply` 命令必须复用上一轮 dry-run 的 patch 内容，只能追加 `--apply`。
- 不要直接写项目文件。唯一落盘入口是 workflow 授权的 `novelgen tool patch ... --apply`。

## Output

最终只输出调用方 schema 要求的 JSON；不要输出 Markdown、解释、代码块、命令记录或额外文本。

