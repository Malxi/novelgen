# 小说大纲评审技能

## 技能目标

对一份故事大纲进行系统评审，从结构、节奏、人物弧光、剧情逻辑和吸引力等维度提供全面的反馈与改进建议。

## 输入参数

- `existing_outline`：待评审的故事大纲，包含篇章、卷次、章节等完整内容
- `setup`（可选）：故事的设定信息，如前提、类型、主题、世界观规则、主线脉络、核心 premise 等
- `user_prompt`（可选）：用户指定的评审关注点。**若提供此参数，请优先围绕用户的具体问题生成建议**

## 输出格式

- `result`：评审结果对象，包含以下字段：
  - `overall_score`：综合评分（0-100）
  - `dimensions`：各维度详细得分
  - `summary`：整体评价
  - `strengths`：亮点与优势
  - `weaknesses`：存在的问题与不足
  - `suggestions`：具体的改进建议

## 评审维度与权重

### 1. 结构（25 分）
- 篇章、卷次、章节的层级划分清晰且合理
- 每一层级的内容范围与篇幅控制得当
- 卷与卷、章与章之间的衔接过渡自然

### 2. 节奏（20 分）
- 章节长度与信息密度适中
- 冲突与张力呈渐进式上升
- 不存在明显的拖沓或过于仓促的段落

### 3. 人物弧光（20 分）
- 角色有清晰可见的成长或变化轨迹
- 行为动机前后一致、令人信服
- 人物之间的互动能够有效推动剧情

### 4. 剧情逻辑（20 分）
- 事件发展符合因果逻辑
- 前后设定不矛盾，伏笔回收合理
- 无重大剧情漏洞或强行降智

### 5. 吸引力（15 分）
- 开篇能够勾起读者兴趣
- 悬念、反转、高潮设置有效
- 情感张力到位，能引发读者共鸣

## 输出示例（JSON）

```json
{
  "overall_score": 75,
  "dimensions": [
    {"name": "structure", "score": 20, "max": 25},
    {"name": "pacing", "score": 15, "max": 20},
    {"name": "character_arcs", "score": 18, "max": 20},
    {"name": "plot_coherence", "score": 17, "max": 20},
    {"name": "engagement", "score": 12, "max": 15}
  ],
  "summary": "大纲基础扎实，但节奏和吸引力方面有提升空间。",
  "strengths": [
    "角色动机清晰有力",
    "三幕结构分明"
  ],
  "weaknesses": [
    "中部卷次略显拖沓",
    "部分章节目的性不强"
  ],
  "suggestions": [
    {
      "category": "节奏",
      "target_id": "P1-V1-C5",
      "target_name": "示例章节名",
      "issue": "本章缺乏张力，略显注水",
      "suggestion": "增加一次关键转折或冲突升级",
      "priority": "high"
    }
  ]
}
```

## 评审原则

1. 建议要具体：必须指明对应的章节或卷次，避免空泛评价
2. 按影响排序：高风险问题优先提出，次要问题后置
3. 考虑类型预期：考虑目标类型与读者群体的预期
4. 建议可操作：建议应具备可操作性，而非空泛的评价

---

## Storyline Texture Review

When setup storylines are available, check whether major chapters clearly move at least some arcs forward. Useful optional signals include chapter `storyline_advances` entries with a concrete `change`, plus `pressure` or `consequence` when that makes the arc more dramatic.

Treat this as a low-pressure craft suggestion. Do not require every chapter to include `storyline_advances`; recommend it only where the arc movement would otherwise feel thin, vague, or easy for later agents to forget.
