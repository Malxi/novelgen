---
name: "novel-entity-extractor"
description: "Uses LLM to intelligently extract RPG entities (characters, items, skills, locations, events) from novel text. Invoke when user needs to extract structured RPG data from unstructured novel content with high accuracy and semantic understanding."
---

# Novel Entity Extractor

This skill uses LLM to intelligently extract RPG entities from novel text with semantic understanding.

## When to Invoke

- User wants to extract RPG data from novel text
- User needs high-accuracy entity extraction
- User wants to understand character relationships, plot events, world building
- Current regex-based extraction is insufficient
- User asks for AI/LLM-based extraction

## Extraction Capabilities

### 1. Characters (角色)
Extract with attributes:
- Name (名字)
- Role type: protagonist/antagonist/supporting (角色类型)
- Personality traits (性格特征)
- Background (背景)
- Goals/Motivations (目标/动机)
- Relationships (关系)
- Power level/Cultivation realm (修为等级)

### 2. Items (物品)
Extract with attributes:
- Name (名字)
- Type: weapon/armor/consumable/treasure (类型)
- Rarity: common/uncommon/rare/epic/legendary (稀有度)
- Description/Effects (描述/效果)
- Owner (拥有者)

### 3. Skills/Abilities (技能)
Extract with attributes:
- Name (名字)
- Type: combat/cultivation/support/passive (类型)
- Description (描述)
- Power level (威力等级)
- Owner (拥有者)

### 4. Locations (地点)
Extract with attributes:
- Name (名字)
- Type: city/dungeon/sect/wilderness (类型)
- Description (描述)
- Connected locations (关联地点)
- Important events (重要事件)

### 5. Events/Timeline (事件/时间线)
Extract with attributes:
- Event type: battle/breakthrough/death/resurrection (事件类型)
- Participants (参与者)
- Location (地点)
- Time/Chapter (时间/章节)
- Consequences (后果)

## Output Format

Return structured JSON:

```json
{
  "characters": [
    {
      "name": "张三",
      "type": "protagonist",
      "personality": "坚毅、聪明",
      "background": "穿越者",
      "goals": "成为最强者",
      "relationships": [{"name": "李四", "relation": "friend"}],
      "power_level": "金丹期"
    }
  ],
  "items": [...],
  "skills": [...],
  "locations": [...],
  "events": [...],
  "timeline": [...],
  "analysis": {
    "plot_summary": "故事主线概述",
    "power_system": "战力体系分析",
    "potential_issues": ["可能的问题"]
  }
}
```

## Guidelines

1. **Context Awareness**: Consider context to disambiguate entities
2. **Relationship Extraction**: Identify connections between characters
3. **Temporal Understanding**: Track event sequence and timeline
4. **Consistency Check**: Flag potential contradictions
5. **Confidence Scoring**: Rate extraction confidence (0.0-1.0)

## Example Usage

Input: "林凡穿越到修仙世界，获得了一个签到系统。他在青云宗修炼，结识了师妹苏婉儿。"

Output should identify:
- Character: 林凡 (protagonist, 穿越者, has 签到系统)
- Character: 苏婉儿 (supporting, 师妹)
- Location: 青云宗 (sect)
- Skill: 签到系统 (system/cheat)
- Event: 穿越 (transmigration)
