---
name: cross-volume-review-workflow
description: 只读跨卷审查多卷大纲的连续性，使用查询工具返回 review_result，不修改任何项目文件。
---

# 跨卷大纲审查 Workflow

你负责审查调用方指定的**多卷范围**（如卷 1-5、卷 6-10）之间的连续性问题。这是**只读 review**：不 patch、不写文件、不运行 check。你的价值在于发现单卷审查看不到的**跨卷问题**：伏笔生命周期、设定一致性漂移、跨卷重复套路、宏观节奏、卷间钩子衔接。

## 必须先执行的工具

按调用方输入中的 `required_queries` 原样执行第一条（通常是 `novelgen tool query outline --type all --view index`）。它是后续所有判断的起点。

## 审查维度（跨卷专属）

**1. 伏笔/线索生命周期**
- 每条主要伏笔（魔门渗透、系统来源、角色身世、金莲、回档等）埋在哪一卷、回收没回收、是否跨卷断线
- 用 `novelgen tool query outline --type refs --entity-type storyline --name "<线名>" --view brief` 查一条线在全书/范围内的所有推进章节，核对埋设-推进-回收节奏

**2. 设定一致性漂移**
- 金手指规则、代价机制、能力边界是否跨卷漂移（如某卷说"消耗不可逆"、后面某卷又随意使用）
- 修为/境界、资源、伤势等状态锚点跨卷是否连贯
- 用 story-setup 查询对照设定书承诺 vs 各卷实际兑现

**3. 跨卷重复套路**
- 不同卷是否反复用同构桥段（如"截胡""三招速胜""反派送头""金句念主题"）
- 用 refs 查同类事件在不同卷的分布，识别换皮重复

**4. 宏观节奏**
- 卷与卷之间的情绪曲线：是否连续多卷压抑、高潮分布是否合理、卷间是否有节奏断裂
- 单卷内部可能是好的，但卷与卷连起来读是否有"平均化"或"节拍器"感

**5. 卷间钩子衔接**
- 上卷卷末钩子在下卷是否兑现、兑现是否及时
- 上卷承诺（payoff_contract、setup 的 reader_promises、escalation_ladder）下卷是否跟进

**6. 角色跨卷弧线**
- 角色跨卷成长/黑化/关系变化是否连贯，有没有跨卷人设漂移

## 规则

- 先通过 index 了解范围结构，再用 refs 按"线"查询；不要逐卷逐章拉取全部详情，聚焦跨卷证据。
- 只允许调用允许列表内的命令。不要运行 `tool check`、`tool patch`、`tool patch-buffer`、`tool refresh`，不要读取源码、故事文件、RPG 文件或 Claude 临时 `tool-results`。
- **每次只能执行一条命令**。严禁用 `&&`、`||`、`;`、`|` 或 `echo` 把多条命令串联或拼接输出分隔符；需要查多个目标就分多次独立调用。链式命令会被门禁拒绝并导致整个审查失败。
- 每条建议必须指出**跨卷证据**：至少引用两个不同卷/章节的事实对比。只针对单卷内部的问题不要列（那是单卷 review 的职责）。
- 每个 suggestion 的 `target_id` 必须是范围内真实存在的 volume/chapter ID；跨卷问题如果无法绑定单个 target_id，可以省略或指向最相关的卷。
- 不要凭空断言"伏笔未回收/设定矛盾"——除非从查询结果中看到了对应事实。宁可少列，不要编造。
- 用户 prompt 是审查任务的最高优先级。
- 保持克制：`suggestions` 最多 10 条，只列真正影响阅读质量的跨卷问题；`strengths` 最多 4 条；`weaknesses` 最多 4 条。
- 最终只输出 JSON，不要输出 Markdown、解释、代码块或额外文本。

## 输出

直接返回 `review_result` 的字段作为顶层 JSON 对象（不要包一层 `review_result` 键，也不要加 Markdown 代码围栏）：

```json
{
  "overall_score": 75,
  "summary": "简短总结（300字内）",
  "dimensions": [
    {"name": "跨卷伏笔", "score": 7.0, "max": 10},
    {"name": "设定一致性", "score": 8.0, "max": 10},
    {"name": "重复套路", "score": 6.5, "max": 10},
    {"name": "宏观节奏", "score": 7.5, "max": 10}
  ],
  "strengths": ["最多 4 条"],
  "weaknesses": ["最多 4 条"],
  "suggestions": [
    {
      "category": "cross-volume",
      "target_id": "P1-V1-C1",
      "target_name": "可选，相关章节",
      "issue": "跨卷问题描述，引用不同卷的事实",
      "suggestion": "具体修改方向",
      "priority": "high|medium|low"
    }
  ],
  "iteration": 0,
  "continuity_issues": []
}
```
