# 小说整卷评审技能
对小说整卷内容进行严格的卷级审核，本次评审将从整卷视角审视：
- **剧情Bug检测（情节矛盾、因果断裂、设定冲突）**
- **逻辑Bug检测（时间线、空间关系、能力值一致性）**

## 强制参考输入项
- `story_setup`：故事核心设定，包含可选的`writing_style`写作风格要求；参考片段只用于评估文风一致性，不作为剧情事实
- `volume`：卷信息（ID、标题、摘要）
- `chapters`：卷内所有章节数组，每章包含：
  - `chapter_id`：章节ID
  - `chapter_title`：章节标题
  - `chapter_summary`：章节摘要
  - `chapter_content`：章节完整内容
  - `beats`：情节节点
- `target_words_per_chapter`：每章目标字数

### 关键原则：问题分配到具体章节
发现跨章节矛盾时，**必须把问题分配到具体的章节 issues 中**，不能只放在卷级 issues：
- 如果第8章写"小石头11岁"，第9章写"小石头12岁"
- 那么第8章的 issues 要包含："年龄描述：本章写小石头11岁，但第9章写12岁，建议统一为11岁"
- 第9章的 issues 要包含："年龄描述：本章写小石头12岁，但第8章写11岁，建议改为11岁"
- **禁止**只在 volume_level_issues 中写"小石头年龄矛盾"而不分配到具体章节

### 好的报告示例
> "数值矛盾：第3章写'练气一层阳寿六十载'，第6章写'练气一层阳寿七十年'，建议统一为六十载"

> "事件矛盾：第5章老郑被搜出下品灵石、挨鞭后还活着，第7章又被搜出中品灵石、被铁锤砸死，等于死了两次。建议明确是一场搜身还是两场，统一灵石品级"

### 差的报告示例
> "阳寿矛盾" - 缺少具体章节和描述
> "见剧情Bug2" - 引用外部编号
> "小石头年龄前后不一致" - 未分配到具体章节的 issues 中

## 修改建议生成原则

### 关键原则：可执行性
所有建议必须是当前章节可以直接执行的修改，**禁止**要求修改其他章节。

### 关键原则：独立性
每个章节的 `issues` 和 `suggestions` 必须是**独立自包含**的：
- 完整描述问题，不要假设读者能看到其他章节
- 禁止引用 Bug 编号或其他章节

### 建议示例
- ❌ "在C9中增加伏笔"（要求修改其他章节）
- ❌ "见剧情Bug2"（引用外部编号）
- ✅ "在本章开头简要回顾C9交付灵石时的细节，暗示主角已经做了手脚"
- ✅ "矿脉图描述为'细筒'但与后文'半张麻纸'的物理形态不符，建议统一描述"

## 强制输出结构
- `volume_review_result`：整卷评审结果
  - `overall_score`：综合质量评分（0-100分）
  - `volume_structure_analysis`：卷结构分析
    - `opening_hook`：开篇钩子评价
    - `rising_action`：上升情节分布评价
    - `climax`：卷终高潮评价
    - `resolution`：收尾评价
  - `character_arcs`：人物成长弧线分析数组
    - `character_name`：人物名称
    - `arc_evaluation`：成长弧线评价
    - `continuity_issues`：连续性问题
  - `chapter_reviews`：各章评审数组
    - `chapter_id`：章节ID
    - `chapter_score`：章节评分（0-10分，可带小数如8.5）
    - `chapter_role`：本章在卷中的定位（setup/development/turning_point/climax/resolution）
    - `continuity_with_previous`：与前章衔接评价
    - `continuity_with_next`：与后章衔接评价
    - `issues`：本章问题列表（字符串数组，独立自包含）
    - `suggestions`：本章改进建议列表（字符串数组，本章可执行）
  - `volume_level_issues`：卷级整体问题（字符串数组）
  - `volume_level_suggestions`：卷级整体改进建议（字符串数组）

## 评分权重调整
- 卷级结构完整性：25%
- 人物成长连贯性：20%
- 剧情连贯性：20%
- 剧情Bug（无Bug满分，有Bug扣分）：15%
- 逻辑Bug（无Bug满分，有Bug扣分）：10%
- 文笔质量：10%

> **重要**：每发现一个critical级别的Bug，总分扣减10分；major级别扣5分；minor级别扣2分。
