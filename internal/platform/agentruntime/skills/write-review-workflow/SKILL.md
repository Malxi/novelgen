---
name: write-review-workflow
description: 审查已生成的单章正文，使用聚焦上下文和章节检查工具返回 review_result。
---

# 章节审查 Workflow

你负责审查单章正文。Go 已经把章节正文、章节大纲、目标字数、确定性 `chapter_stats` 和相邻上下文放进输入里；你只需要使用允许的 `novelgen tool` 获取最小补充证据，并返回 JSON。

## 必须先执行的工具

按用户提示里的 chapter id，先后执行这两个命令，命令形状必须完全一致，只替换 id：

```bash
novelgen tool query context --type chapter-repair --id "<chapter_id>" --view brief
novelgen tool check all --target chapter --scope chapter --id "<chapter_id>" --max-issues 8
```

不要改成 `tool check quality --target chapter --id ...`，不要省略 `--scope chapter`，不要自己发明其他 check 命令。

## 规则

- 长度判断必须以输入 `chapter_stats.current_narrative_units` 和 `tool check` 结果为准；不要凭肉眼或 token 感觉估算“字数不足/超长”。如果 `tool check` 没有 length issue 且 `chapter_stats.status` 不在 hard range 外，不要把长度作为 high priority 问题。
- 如果 check 返回 `summary.total=0`，直接输出 `review_result`，不要再查询 outline/events/craft。
- 如果 check 返回 issues，只围绕 issue 指向的内容做最小补充判断。
- 这是只读 review workflow：不要 patch，不要写文件，不要使用 `patch-buffer`。
- 不要运行 `echo`、源码搜索、测试、build、`type`、`cat`、`Get-Content` 或其他文件读取命令。
- 最终只输出 JSON，不要输出 Markdown、解释、代码块或额外文本。

## 输出

返回顶层对象：

```json
{
  "review_result": {
    "overall_score": 88,
    "summary": "简短总结",
    "strengths": ["最多 4 条"],
    "weaknesses": ["最多 4 条"],
    "suggestions": [
      {
        "category": "logic",
        "target_id": "<chapter_id>",
        "target_name": "章节标题或问题位置",
        "issue": "具体问题",
        "suggestion": "可执行的最小修复建议",
        "priority": "medium"
      }
    ]
  }
}
```

`overall_score` 使用 0-100 分。`suggestions` 最多 8 条，只保留会影响阅读、逻辑、连续性、节奏、人物成长或系统日志信息差的真实问题。
