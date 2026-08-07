---
name: outline-review-workflow
description: 只读审查整本或单卷大纲，使用查询工具返回 review_result，不修改任何项目文件。
---

# 大纲审查 Workflow

你负责审查调用方指定范围的大纲（整本或单个 volume）。这是**只读 review**：不 patch、不写文件、不运行 check（确定性检查由调用方单独运行）。你的价值在于发现确定性规则覆盖不到的开放性问题：节奏、人物动机、主题呼应、信息差博弈，以及用户 prompt 指定的关注点。

## 必须先执行的工具

按调用方输入中的 `required_queries` 原样执行第一条。整本审查通常是 `novelgen tool query outline --type all --view index`，单卷审查是目标卷的 context index。它是后续所有判断的起点，不要跳过它直接查细节。

## 规则

- 先通过 index 了解整体结构，再按需查询 volume brief、events、story-setup；不要一次性拉取全部章节详情，不要查询章节正文。
- 只允许调用允许列表内的命令。不要运行 `tool check`、`tool patch`、`tool patch-buffer`、`tool refresh`，不要读取源码、故事文件、RPG 文件或 Claude 临时 `tool-results`。
- 章节级细节（如 `storyline_advances`、`chapter_payoff`、`conflict`、章节事件）用 `novelgen tool query outline --type chapter --id "<id>" --view brief` 或 `novelgen tool query outline --type events --chapter-id "<id>" --view brief` 查询；不要使用 `context --type outline-chapter` 或 `context --type outline-events`（不支持的类型）。
- 每个 suggestion 的 `target_id` 必须是指定范围内真实存在的 volume/chapter ID（如 `P1-V1`、`P1-V1-C2`）；不确定就不要写 `target_id`。
- 不要凭空断言"伏笔未回收/设定矛盾"——除非从查询结果中看到了对应事实。宁可少列，不要编造。
- 用户 prompt 是审查任务的最高优先级；没有用户 prompt 时，按结构、节奏、连贯性、人物、情节、信息差等维度自由审查。
- 保持克制：`suggestions` 最多 8 条，只列真正影响阅读质量的问题；`strengths` 最多 4 条；`weaknesses` 最多 4 条。
- 最终只输出 JSON，不要输出 Markdown、解释、代码块或额外文本。

## 输出

直接返回 `review_result` 的字段作为顶层 JSON 对象（不要包一层 `review_result` 键，也不要加 Markdown 代码围栏）：

```json
{
  "overall_score": 82,
  "summary": "简短总结",
  "dimensions": [
    {"name": "结构", "score": 8.5, "max": 10},
    {"name": "节奏", "score": 8.0, "max": 10}
  ],
  "strengths": ["最多 4 条"],
  "weaknesses": ["最多 4 条"],
  "suggestions": [
    {
      "category": "character",
      "target_id": "P1-V2",
      "target_name": "卷标题或章节标题",
      "issue": "具体问题",
      "suggestion": "可执行的最小修改建议",
      "priority": "high"
    }
  ]
}
```

`overall_score` 使用 0-100 分。`priority` 取值 `critical`、`high`、`medium`、`low`。
