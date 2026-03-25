# 小说大纲重生成技能

## 文档信息
- 版本：V1.0
- 适用场景：重新生成 Part/Volume/Chapter 同时保持与整体故事的连续性
- 核心目标：在保持故事连续性的前提下，根据用户建议重新生成指定元素

---

## 一、核心执行铁则

1. **连续性至上**：重生成的内容必须直接承接前文事件
2. **设置下一章**：当前内容必须自然引入下一章
3. **保留事件**：保留或替换相同叙事目的的事件
4. **角色一致性**：角色行为必须与其已建立的状态一致
5. **结构稳定**：不改变 Part/Volume/Chapter 的数量，只做精准编辑

---

## 二、上下文信息

### 提供的上下文
1. **当前元素类型**：part / volume / chapter
2. **当前元素ID**：标识要重生成的元素
3. **上下文信息**：
   - Part 重生成：前后 Part 的信息
   - Volume 重生成：所属 Part 信息 + 前后 Volume 信息
   - Chapter 重生成：所属 Part 和 Volume 信息 + 前后 Chapter 详细信息
4. **用户建议**：用户的具体修改意见

### 故事设定信息
- Project Name：项目名称
- Genres：题材类型
- Theme：主题
- Tone：基调

---

## 三、重生成要求

### 连续性锚点（CRITICAL）
1. **承接前文**：必须直接承接前一章节的事件，引用具体事件、角色状态、情节发展
2. **设置后文**：必须自然引入下一章，创建叙事动力
3. **保留事件**：如果当前章节已有事件列表，确保重生成版本包含相同事件（或服务于相同叙事目的的替代事件）
4. **角色一致性**：角色必须与其已建立的情感状态和关系保持一致
5. **因果逻辑**：每个情节节拍必须是前一事件的逻辑后果

### 章节特定要求（Chapter Regeneration）
- **opening_beat**：必须等于 beats[0]，且直接承接前一章的 closing_beat
- **closing_beat**：必须等于 beats[last]，且设置下一章的 opening_beat
- **state_change**：必须与前后章节保持一致

---

## 四、事件类型参考

### 事件类型（Events）
1. **relationship**：角色关系变化
   - Characters: [characterA, characterB]
   - Subject: 关系名称/类型
   - Change: 关系变化描述

2. **goal**：角色目标更新
   - Characters: [character]
   - Subject: 目标名称/描述
   - Change: achieved / abandoned / new

3. **item**：物品获取/失去
   - Characters: [character]
   - Subject: 物品名称
   - Change: get / lost

4. **premise**：能力/系统升级
   - Characters: [character]
   - Subject: 系统名称
   - Change: level up / new ability / breakthrough

5. **storyline**：故事线进展
   - Characters: []
   - Subject: 故事线名称
   - Change: started / advanced / completed / twist

6. **gate**：障碍/代价引入
   - Characters: [character]
   - Subject: 障碍名称
   - Change: introduced / escalated / overcome

7. **status**：角色状态变化
   - Characters: [character]
   - Subject: 状态类型
   - Change: 当前状态

8. **memory**：信息/知识获取
   - Characters: [character]
   - Subject: 获得的信息
   - Change: info / secret / knowledge

---

## 五、输出要求

### Part 重生成输出
```json
{
  "id": "P1",
  "title": "Part标题",
  "summary": "Part摘要"
}
```

### Volume 重生成输出
```json
{
  "id": "P1-V1",
  "title": "Volume标题",
  "summary": "Volume摘要",
  "chapters": [...]
}
```

### Chapter 重生成输出
```json
{
  "id": "P1-V1-C1",
  "title": "章节标题",
  "summary": "一句话摘要",
  "characters": ["角色1", "角色2"],
  "location": "地点",
  "events": [...],
  "beats": ["节拍1", "节拍2", "节拍3"],
  "opening_beat": "节拍1",
  "closing_beat": "节拍3",
  "conflict": "核心冲突",
  "pacing": "normal"
}
```

---

## 六、质量自检清单

重生成完成后，必须检查：
- [ ] 严格承接前文 - 引用前一章节的具体事件
- [ ] 正确设置后文 - 与下一章自然衔接
- [ ] 保留或合理替换事件
- [ ] 角色行为一致
- [ ] 因果逻辑清晰
- [ ] 遵循用户建议
- [ ] opening_beat / closing_beat 与 beats 一致
- [ ] 所有内容使用指定语言
