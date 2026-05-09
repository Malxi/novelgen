# 小说设定审查技能

## 文档信息
- 版本：V1.0
- 适用场景：审查已生成的小说设定，识别问题并提供改进建议
- 核心目标：确保设定质量，发现逻辑漏洞、吸引力不足等问题

---

## Storyline Texture Review

## Setup Compactness Review

Review whether setup is usable as a compact contract for later agents. Flag
oversized sections when they look like full biographies, lore essays, item
catalogs, minor cast dumps, or chapter-by-chapter plans. Prefer suggestions
that compress detail into role, function, rule boundary, pressure engine, payoff
promise, progression tier, or resource name.

When reviewing `storylines`, look for whether important arcs have enough high-level dramatic pressure to guide later outline generation. Optional texture fields include `scope`, `payoff_style`, `setup_role`, `desire`, `opposition`, `stakes`, `turn`, `payoff`, `open_question`, and `pressure_points`.

For long-form work, also review whether important storylines behave like repeatable engines. Useful fields are `repeatable_pressure`, `payoff_cadence`, `mutation`, and `failure_mode`. Flag a 1000-chapter setup whose main arcs can only fire once, have no cadence, or have no plan for changing shape.

Treat these as creative suggestions, not hard requirements. Recommend adding them only when a storyline feels thin, static, or hard to continue. Do not penalize a clean storyline for leaving some optional fields blank.

Do not ask setup to become a chapter plan. If an arc is long-running, prefer recommending `scope: book/series` and `payoff_style: staged_reveal/slow_burn` instead of demanding an immediate payoff.

## Appeal Engine Review

For power-fantasy/web-novel setups, review whether important `storylines` and core `premises[]` have a usable `appeal_engine` or at least imply one.

A strong appeal engine has:
- a clear `appeal` readers want to see again
- a `surface_limit` that prevents unlimited power without making every victory miserable
- an `exploit` that lets the protagonist win through rule use, timing, information gap, or opponent misread
- a concrete `signature_win` image
- an `upgrade_path` for escalation
- an `opponent_misread` that can create face-slapping / reversal moments
- a `reward_type` so wins visibly change the situation

Flag setups that are logical but not fun: rules that explain the world yet do not suggest how the protagonist can win beautifully. Prefer suggestions like "add a surface limit and exploit pattern" over "add harsher cost" unless the genre specifically demands suffering.

## Progression System Depth Review

Keep one clear root premise, but review whether long-form genre fiction has enough derived systems for later outline and RPG simulation. A setup for sci-fi, mecha, cultivation, apocalypse, fantasy, or other progression-heavy genres is thin if it has only one broad `premises` entry.

Recommend 3-6 derived `premises` systems when appropriate. Good systems include protagonist growth, enemy tier ecology, faction technology, resource economy, social/faction hierarchy, and final external threat. Each system should have a progression ladder with named stages, requirements or costs, ceilings, and a clear narrative use.

## Long Form Plan Review

Review `long_form_plan` when the setup implies serial/web-novel scale, 300+ chapters, 1000 chapters, large group cast, or multi-volume progression. It should explain how the story stays fun over many volumes without becoming a chapter outline.

A strong plan has target scale, repeatable main_loop, escalation_ladder, reader_promises, payoff_cadence, volume_pattern, midpoint_mutation, and endgame_promise. Flag plans that promise 1000 chapters but have no escalation ladder, no cadence, no late-story mutation, or no repeatable payoff loop.

## Core Cast Review

Review whether `core_cast` gives craft enough signal to expand characters consistently. A useful seed has a stable name, role, importance, story_function, entry_phase, payoff, and a sensible relationship arc. Flag setups that have many implied major characters but no core cast seeds, or important seeds that only contain names.

For long-form stories, a thin core cast is a structural problem: it makes craft guess at lead roles, delayed entrances, and relationship payoffs.

For serial/web-novel scale, also check capacity: there should be a protagonist anchor, several importance 8-10 roles, at least a few distinct role types, some mid/late/series entrances, and storyline_refs that point to real setup storylines. Prefer asking for seeds and payoff promises, not full biographies.

## Writing Style Review

Review `writing_style` only when it is present or when the user explicitly requested a style/reference passage. A useful writing style contract should describe prose-level execution: narrative voice, sentence rhythm, description density, dialogue texture, and concrete do/don't principles.

If `reference_excerpt` is present, verify that it is treated as style-only. Flag any setup text that imports the reference passage's plot, characters, places, terminology, or sentences as canon.

## 一、审查维度

### 1. 根设定审查
- **唯一性**：是否只有一个清晰的根设定
- **差异化**：是否有独特的记忆点，避免同质化
- **冲突性**：是否自带天然的核心冲突
- **悬念感**：是否能让读者产生好奇心

### 2. 逻辑闭环审查
- **规则一致性**：核心规则是否全程统一，无双重标准
- **边界清晰**：能力/权限是否有明确的可执行范围和绝对禁止边界
- **补丁完善**：是否针对极端场景提前明确规则

### 3. 设定-剧情适配审查
- **服务主线**：所有设定是否为主角成长、主线冲突、核心主题服务
- **无效设定**：是否存在与主线无关的无效堆砌内容
- **剧情钩子**：是否预埋了可落地的剧情钩子

### 4. 吸引力审查
- **爽点设置**：是否有足够的爽点支撑故事节奏
- **悬念预埋**：是否设置了贯穿全文的核心悬念
- **共情点**：是否能让读者产生情感共鸣

### 5. 可扩展性审查
- **世界观深度**：是否支持长篇故事的展开
- **留白区域**：是否有适当的留白供后续发挥
- **支线可能性**：是否预留了支线故事的拓展空间

---

## 二、评分标准

### 总分计算（0-100分）
- 根设定质量：20分
- 逻辑闭环：25分
- 设定-剧情适配：20分
- 吸引力：20分
- 可扩展性：15分

### 评分等级
- 90-100：优秀，可直接使用
- 80-89：良好，轻微调整即可
- 70-79：合格，需要一定修改
- 60-69：及格，需要较大修改
- <60：不合格，建议重新生成

---

## 三、输出格式

### 审查结果必须包含：
1. **总体评分**（0-100）
2. **各维度评分**
3. **总体评价**（一句话总结）
4. **优点列表**（至少3条）
5. **问题列表**（按优先级排序）
6. **具体改进建议**（可执行的建议）

### 优先级定义：
- **high**：严重问题，必须修复
- **medium**：中等问题，建议修复
- **low**：轻微问题，可选修复

---

## 四、审查流程

1. **通读设定**：完整阅读所有设定内容
2. **维度检查**：按五个维度逐一检查
3. **问题识别**：记录发现的所有问题
4. **优先级排序**：按影响程度排序
5. **建议生成**：针对每个问题给出具体建议
6. **综合评分**：给出总体和分项评分

---

## 五、注意事项

1. **客观公正**：基于设定本身质量评价，不受题材偏好影响
2. **具体明确**：指出问题时需引用具体文本
3. **建设性**：每个问题都应配有改进建议
4. **优先级分明**：区分必须修复和建议修复的问题
