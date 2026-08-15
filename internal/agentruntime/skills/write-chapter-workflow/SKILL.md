---
name: write-chapter-workflow
description: 使用 chapter-write 上下文生成单章正文 JSON。
---

# Chapter Write Workflow

你负责根据 `chapter-write` context 写当前章节正文。生成模式只返回 JSON 给 Go 保存，不直接写项目文件。

## 必须执行

- 先查询调用方指定的 `chapter-write` 小型确认包：
  `novelgen tool query context --type chapter-write --id "<chapter_id>" --view brief --fields path,navigation,stats,warnings`
- 章节事实、recap、前后文和 next-chapter hook 以调用方 typed input 为准；这个小型确认包只用于确认项目路径、相邻章节、实体名和工具策略。
- 只写当前章节，不修改 outline、setup、craft、recap、draft 或其它章节。
- 生成模式没有 patch 权限。不要运行 `tool patch`、`patch-buffer`、`tool check`、`tool refresh`。
- 不要运行 `novelgen tool query chapter --content`。`chapter-write` context 已经是生成正文所需的事实包。
- 工具返回结果已经显示在对话里，不要再读 Claude 临时 `tool-results`。不要直接读写章节文件、日志文件、源码；不要使用 `Get-Content`、`cat`、`type`、`grep/findstr`、`head`、`Select-String`、`--help`、管道、重定向、`2>&1`、临时文件、`echo/printf` 输出 JSON 或 shell 探测命令。只有只读 query/check 结果明显乱码时，才可重试一次 runtime 允许的 UTF-8 PowerShell 包装；不得用于正文或最终 JSON。
- 正文只能放在最终 assistant JSON 的 `content` 字段里；禁止用 Bash、`printf`、`echo`、PowerShell 字符串、临时文件或管道来承载、计数、截断、暂存、展示正文。
- 如果某个额外工具调用被拒绝，立即停止工具探索，使用已经取得的 `chapter-write` context 返回最终 JSON。
- 成功拿到 `chapter-write` 小型确认包后，不要再次查询、截断、过滤或 shell 摘要这个结果；不要改用普通 `--view brief` 或 `--view full`。直接用 typed input 和对话中的工具结果写作。
- 如果 context 中有 `next_actions`，只执行其中允许的最小查询；如果没有明确要求，不要扩大上下文。
- 只有输入里的 `history_mode` 非空，才查询历史 logs。此时先运行：
  `novelgen tool query logs --view index --limit 5`
  默认只使用 index 里的摘要；如果没有 agent-live，优先参考 prompts/responses 的创作历史。只有索引明确显示某条已完成旧运行高度相关时，最多读取 1 条 exact brief，不要读取 content。把历史作为创作意图、风格、失败教训和工具策略参考，不要复制旧输出，不要把命令记录写进正文，不要因为历史而扩写目标篇幅。

## 写作要求

- 正文必须体现本章事件、冲突、转折和结尾承诺。
- 对 system log / 信息差题材，主角成长可以是“获得日志线索、建立可执行判断、把信息差转化为行动优势”；不必每章都写修炼突破、道具或盟友。
- 如果 `history_mode` 为空，不要查询 logs；`chapter-write` context 已经足够。
- 如果输出 markdown 标题，只能有一个 `# <章节标题>`；正文第一段不要再次重复章节标题。
- 目标篇幅以调用方输入 `target_words` 为硬预算；不要为了默认项目字数、历史日志、未来伏笔或世界观解释自行扩写。
- 按调用方给出的段落预算写作：用紧凑段落合并事件，不要把每个 beat 展开成独立长场景。宁可减少环境描写、心理分析和解释性对白，也要先满足目标篇幅。

### AI 味预防（生成时逐项自查，写出来之前先检查）
> 以下每一项命中，说明这段文字"太像 AI 写的"，写出前先修掉：
1.  **金句密度**：对仗句、排比句、总结性警句每章最多 1-2 处，必须自然出自人物处境；禁止段落结尾都塞金句、禁止"前世老师说过"这类万能金句触发器反复使用。
2.  **模板化节奏**：禁止每章完美走完"目标-冲突-解决-钩子"无意外、无粗糙感；允许失败的尝试、无意义的闲笔、没解决的小事。
3.  **情绪过于正确**：角色禁止永远"合理反应"——允许因偏见、冲动、面子、误会做蠢事；主角失误后禁止每次都立刻总结升华成哲理（偶尔让失误就只是失误）。
4.  **过度工整**：伏笔回收禁止两章内完美闭环；让部分伏笔断一阵线、让部分小事没有下文。
5.  **设计感过重**：禁止每场戏都精准服务于剧情；可以有一次漫无目的的闲笔（看路人、闻气味、发呆）。
6.  **世界观说明书**：设定信息禁止以说明文倾倒，必须通过事件、对话、动作自然透出；一段内连续两条以上"系统提示/规则解释"即视为说明书。
7.  **强加限制**：主角金手指的"不无敌"来自对手反制和剧情冲突（反派克制、策略变化、信息被利用），**禁止**给金手指本身编造冷却/权限/距离惩罚/使用代价等自缚机制。
8.  **元语言**：禁止"他心想，这是……"式的自我总结、禁止"仿佛/宛如/像极了"过度修辞、禁止章节标题式总结感。
9.  **破折号克制（铁律）**：破折号"——"每千字不超过 3 处。解释性/补充性破折号改用逗号、冒号、句号或直接拆句；只保留真正需要停顿的强调性破折号（插入语、转折、心理中断）。整段连续两处以上"——"即为滥用。

## 输出

最终只作为 assistant response 输出调用方要求的 JSON，且只保留 schema 允许的字段，通常就是 `content`。不要输出 `chapter_id`、`title`、`word_count`、`notes` 或其它元数据；不要用 Bash、`echo`、`printf`、PowerShell 或文件写入来输出 JSON/正文。不要输出命令记录、解释、Markdown 代码块或额外字段。
