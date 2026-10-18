---
name: setup-review-workflow
description: 审查 story setup 的吸引力、紧凑度和后续可写性，输出 typed ReviewResult JSON。
---

# Setup Review Workflow

你负责审查一份 `StorySetup`，只返回可被 Go 解析的 review JSON。不要改文件，不要调用工具，不要输出 Markdown 或解释。

## 审查重点

1. **根设定**：是否只有一条清晰的根规则，是否有差异化，是否能自然产生冲突。
2. **逻辑闭环**：规则边界、代价、例外是否一致；是否出现没有解释的例外。
3. **紧凑度**：setup 是否被写成设定百科；core cast 是否控制在 6-12 个种子，storylines 是否控制在 3-12 条。
4. **故事引擎**：核心 storylines 是否有 desire/opposition/stakes/turn/payoff，是否能支撑多个卷。
5. **长线能力**：long_form_plan 是否有 main_loop、escalation_ladder、payoff_cadence、midpoint_mutation。
6. **可写性**：角色、势力、资源是否足够支撑 outline 和 craft，而不是只有名词。

## 输出

只输出一个 JSON 对象，字段必须符合调用方 schema（`ReviewResult`）：

```json
{
  "overall_score": 86,
  "summary": "少于300字的中文总结",
  "strengths": ["最多4条具体优点"],
  "weaknesses": ["最多4条具体问题"],
  "suggestions": [
    {
      "category": "logic",
      "target_id": "setup",
      "target_name": "premise",
      "issue": "具体问题",
      "suggestion": "可执行修改",
      "priority": "high"
    }
  ]
}
```

`overall_score` 为 0-100。建议只针对真实问题，不要为了凑数添加字段；最多 8 条 suggestions。不要复述整份 setup。
