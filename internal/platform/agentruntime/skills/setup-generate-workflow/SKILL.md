---
name: setup-generate-workflow
description: 根据用户 idea 和可选的 creative_provocation 生成 typed story setup JSON。
---

# Setup Generate Workflow

你负责生成一份可被 Go 解析和校验的 `StorySetup` JSON。Go 负责 ID、normalize、quality gate、checkpoint 和保存；你只返回 JSON。

## 输出

- 只输出纯 JSON 对象，不要 Markdown、代码块、解释或命令日志。
- 顶层字段必须符合调用方给出的 output schema。
- 必填字段：`project_name`、`genres`、`premise`、`theme`、`rules`、`target_audience`、`tone`、`tense`、`pov_style`。
- 适合长篇时再填 `long_form_plan`、`core_cast`、`storylines`、`premises`、`world_resources`、`world_timeline`。
- setup 是紧凑契约，不是设定百科：core cast 保持 6-12 个种子，storylines 保持 3-12 条，rules 只保留真正有生成力的边界。

## 创作约束

- 用户 `idea` 的明确要求优先。随机句只能增加结构性压力，不能替换用户题材、主角或核心关系。
- 如果输入有 `creative_provocation`：
  - 严格遵守 `axiom_budget=1`。
  - 按 `derivation_steps` 的顺序推导：公理 → 机制 → 第一层后果 → 制度 → 激励 → 失败模式 → 三类冲突 → 反转与线索。
  - 任何 `reject_if` 成立时，重构设定；不要靠第二条魔法规则圆场。
  - 拒绝 `forbidden_explanations`：梦、幻觉、传说、隐喻、巧合、远古神秘力量。
  - 把随机句变成世界内可执行的规则、制度、交换或历史事件，而不是名字、气氛或预言。
- 如果输入有 `creative_scenario`：
  - 把 `arena`、`scarcity`、`hazard`、`objective`、`enforcer`、`mobility`、`threat`、`cost`、`twist` 当作固定因果角色，不得替换成新的魔法规则。
  - `mecha_role` 是机甲独立承担的结构角色，不能把机甲替换成无人机或其他载具。
  - 六个元素必须分别落在 `element_roles` 指定的结构位置，不能只是名词装饰。
  - 扩写名称、历史、势力、角色、资源和故事线，但保持 `causal_chain` 成立。

## 自检

输出前逐项检查：

1. 是否只有一个不可解释的公理？
2. 每个机构是否有受益者、预算或执行能力？
3. 普通人是否有可理解的方式利用或绕过规则？绕过是否有成本？
4. 规则是否能产生至少三种不同性质的卷级冲突？
5. 最重要的反转是否有 2-3 个可提前埋下的线索？

不要写 patch，不要调用工具，不要读取项目文件；直接返回 setup JSON。
