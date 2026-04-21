# 小说大纲生成技能

## 文档信息
- 版本：V1.0
- 适用场景：基于小说设定生成结构化大纲
- 核心目标：生成具有严格结构的故事大纲（Parts → Volumes → Chapters）

---

## 一、核心执行铁则

1. **严格结构原则**：必须严格按照指定的 Parts × Volumes × Chapters 结构生成
2. **连续性原则**：章节之间必须有直接的因果联系，上一章的结尾必须是下一章的开头
3. **事件追踪原则**：使用 Events 字段追踪所有状态变化，确保故事线连贯
4. **角色一致性原则**：角色的情感状态、目标、关系必须自然演变，不能重置

---

## 二、输出结构要求

### 严格三级结构
- **Parts**：故事的主要部分（如：开篇、发展、高潮、结局）
- **Volumes**：每个 Part 内的卷/册
- **Chapters**：每 Volume 内的章节

### ID 格式（代码自动生成，可留空）
- Part ID: P{number} (e.g., P1, P2)
- Volume ID: P{part}-V{volume} (e.g., P1-V1, P2-V3)
- Chapter ID: P{part}-V{volume}-C{chapter} (e.g., P1-V1-C1, P2-V3-C5)

---

## 三、章节字段要求

### 必填字段
1. **title**：章节标题
2. **summary**：一句话摘要，格式：人物 在 地点 发生了 事件
3. **characters**：出场角色列表（summary 中的角色必须在此列出）
4. **location**：发生地点（必须与 summary 中的地点匹配）
5. **events**：状态变化事件列表（⚠️ 每章 3-5 个事件，覆盖章节的主要情节变化）
6. **beats**：3-5 个关键情节点
7. **opening_beat**：开场情节点（必须等于 beats[0]）
8. **closing_beat**：结尾情节点（必须等于 beats[last]）
9. **conflict**：核心冲突（一句话描述）
10. **pacing**：节奏（slow/normal/fast）

### 事件类型（用于 Events 字段）

#### ⚠️ 推荐：新 Event 结构（语义更清晰，推荐优先使用）

新结构使用 **Actor → Action → Target** 的主谓宾格式，语义更明确：

```yaml
- actor: 执行动作的角色（主语）
  action: 动作类型（谓语）
  target: 动作目标（宾语）
  target_type: 目标类型
  context: 动作发生的上下文/地点（可选）
  result: 动作结果的描述（可选，用于AI理解）
  
  # 旧字段（向后兼容，可选填）
  type: 事件类型
  characters: [涉及的角色列表]
  subject: 目标对象
  change: 变化状态
  details: 详细描述
```

##### Action 动作类型（谓语）

**物品相关：**
- `acquire` - 获得物品（例：林跃 acquire 生存刀）
- `use` - 使用物品
- `lose` - 失去物品
- `craft` - 制造/合成物品

**移动相关：**
- `move` - 移动位置
- `enter` - 进入地点
- `leave` - 离开地点
- `teleport` - 传送/跃迁

**战斗相关：**
- `combat` - 进入战斗
- `defeat` - 击败目标
- `escape` - 逃离战斗

**能力相关：**
- `learn` - 学习技能/知识
- `awaken` - 觉醒能力
- `upgrade` - 升级能力
- `master` - 掌握能力

**发现相关：**
- `discover` - 发现/探索
- `reveal` - 揭示/揭露

**关系相关：**
- `meet` - 遇见角色
- `befriend` - 建立友谊

**状态相关：**
- `transform` - 转变/变身
- `recover` - 恢复

**目标相关：**
- `set` - 设定目标
- `progress` - 推进目标
- `achieve` - 达成目标

##### TargetType 目标类型
- `item` - 物品
- `character` - 角色
- `location` - 地点
- `skill` - 技能
- `status` - 状态
- `relationship` - 关系
- `knowledge` - 知识

##### 新格式示例
```yaml
# 获得物品
- actor: "林跃"
  action: "acquire"
  target: "生存刀"
  target_type: "item"
  context: "低温设施地下三层"
  result: "林跃从杂物间找到生锈的生存刀"

# 觉醒能力
- actor: "林跃"
  action: "awaken"
  target: "基因进化能力"
  target_type: "premise"
  context: "废弃动力室"
  result: "林跃发现自己的身体对虫族腐蚀液免疫"

# 遇见角色
- actor: "林跃"
  action: "meet"
  target: "拾荒者小队"
  target_type: "character"
  context: "废土地表"
  result: "林跃遇到正在搜寻物资的幸存者"
```

---

#### 旧 Event 结构（向后兼容）

如果无法确定新格式的字段，可以使用旧格式：

```yaml
- type: 事件类型
  characters: [涉及的角色列表]
  subject: 目标对象（角色/物品/故事线名称）
  change: 变化状态（必须填写，见下表）
  details: 详细描述
```

##### 各类型事件的 Change 值规范

1. **relationship** - 角色关系变化
   - `established` - 建立关系
   - `improved` - 关系改善
   - `deteriorated` - 关系恶化
   - `broken` - 关系破裂

2. **goal** - 角色目标更新
   - `set` - 设定目标
   - `progressed` - 目标有进展
   - `abandoned` - 放弃目标
   - `achieved` - 达成目标

3. **item** - 物品获取/失去
   - `acquired` - 获得物品
   - `lost` - 失去物品
   - `used` - 使用物品
   - `upgraded` - 物品升级

4. **premise** - 能力/系统升级
   - `awakened` - 初次觉醒
   - `upgraded` - 能力升级
   - `mastered` - 掌握能力
   - `degraded` - 能力退化

5. **storyline** - 故事线进展（⚠️ 重要：必须填写 change）
   - `started` - 故事线开始
   - `progressed` - 故事线推进
   - `escalated` - 冲突升级
   - `climax` - 高潮阶段
   - `completed` - 故事线完成
   - `twisted` - 意外转折

6. **gate** - 障碍/代价引入
   - `introduced` - 引入障碍
   - `escalated` - 障碍升级
   - `overcome` - 克服障碍

7. **status** - 角色状态变化
   - `afflicted` - 受到状态影响
   - `worsened` - 状态恶化
   - `improved` - 状态好转
   - `resolved` - 状态解除

8. **memory** - 信息/知识获取
   - `learned` - 获得信息
   - `forgotten` - 遗忘信息
   - `shared` - 分享信息

##### 旧格式示例
```yaml
# storyline 事件示例（正确格式）
- type: storyline
  characters: [林砚, 缉厄司]
  subject: "矿场死循环"
  change: "started"  # 必须填写！
  details: "矿场的人为压迫成为林砚生存的直接威胁..."

# premise 事件示例
- type: premise
  characters: [林砚]
  subject: "附矿场复活"
  change: "upgraded"
  details: "累计死亡五次后，复活能力达到Lv2"
```

---

## 四、连续性要求（极其重要）

### 1. 因果链
- 每一章必须是前一章事件的直接后果
- 问自己："因为前一章发生了什么，所以这一章发生了什么？"

### 2. 节拍连续性（BEAT CONTINUITY）
- 第 N 章的最后一个节拍和第 N+1 章的第一个节拍必须直接相连
- 示例：如果第1章结尾是"主角修炼到天亮"，第2章必须以"天亮了，主角结束修炼"开始
- 禁止在章节之间有时间跳跃或场景切换

### 3. 角色状态追踪
- 角色的情感状态、目标、关系必须随章节自然演变
- 不能在新的章节中"重置"角色发展

### 4. 情节线索
- 故事线应该逐步推进，第1章开始的故事线应该在后续章节中显示进展
- 不能消失或遗忘已建立的情节线索

---

## 五、节拍编写指南

### 节拍结构
- 每个章节 3-5 个节拍
- 节拍必须交替：意图 → 障碍 → 后果
- 不能连续三个节拍没有冲突/复杂情况

### 节拍连续性标记
- 使用 "Therefore," 或 "Then," 开头来强制因果流
- 确保每个节拍都是前一个节拍的逻辑结果

### 章节钩子
- 每章的最后一个节拍必须引入一个具体的障碍或决定
- 下一章必须解决或升级这个障碍

---

## 六、质量自检清单

生成完成后，必须检查：
- [ ] 结构数量完全符合要求
- [ ] 所有章节都有完整的必填字段
- [ ] opening_beat == beats[0]
- [ ] closing_beat == beats[last]
- [ ] 章节之间有明确的连续性
- [ ] **⚠️ 每章必须有 3-5 个 events（硬性要求！）**
  - 不要将整章内容压缩为单个事件
  - 每个 event 应该代表一个清晰的动作或状态变化
  - 事件流程建议：进入/遇见 → 发现/学习 → 战斗/达成
- [ ] **所有 Events 都有有效的 `change` 字段（⚠️ 关键！）**
  - storyline 事件必须有 change: started/progressed/completed 等
  - premise 事件必须有 change: awakened/upgraded/mastered 等
  - relationship 事件必须有 change: established/improved/deteriorated 等
- [ ] 事件追踪完整
- [ ] 角色行为一致
- [ ] 所有内容使用指定语言
