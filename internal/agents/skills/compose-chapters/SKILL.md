# Compose Chapters Skill

## Purpose
Generate detailed chapters for a specific volume, maintaining continuity with previous volumes and building toward the volume's goals.

## Input
- `setup`: StorySetup with premise, characters, storylines, etc.
- `part`: Current Part information (title, summary)
- `volume`: Current Volume to generate chapters for (title, summary)
- `volume_index`: Index of this volume (1-based)
- `total_volumes`: Total number of volumes in the outline
- `chapters_per_volume`: Number of chapters to generate
- `previous_volume`: Previous volume (if any) for continuity
- `outline_context`: Context from previous volumes

## Output
- `chapters`: Array of Chapter objects with complete details

## Chapter Requirements

### Required Fields
1. **title**: Chapter title (string)
2. **summary**: One-sentence summary (Character + Location + Event) (string)
3. **characters**: List of characters appearing (array of strings)
4. **location**: Primary location (string)
5. **events**: State change events (array of event objects)
6. **beats**: 3-5 plot beats (array of strings, NOT comma-separated)
7. **opening_beat**: Must equal beats[0] (string)
8. **closing_beat**: Must equal beats[last] (string)
9. **conflict**: Core conflict description (string)
10. **pacing**: slow/normal/fast (string)
11. **timeline**: Chapter timeline information (object) - ⚠️ REQUIRED for timeline consistency

### ⚠️ CRITICAL: Events Count Requirement
**Each chapter MUST have 3-5 events.** This is a HARD requirement.

Each event represents a distinct plot beat or state change in the chapter:
- Event 1: Opening/Discovery (enter location, meet character, discover item)
- Event 2: Development/Conflict (learn skill, face obstacle, transform status)
- Event 3: Climax/Resolution (combat, achieve goal, establish relationship)
- Event 4-5 (optional): Aftermath/Setup (move to new location, gain reward, set new goal)

**DO NOT compress all chapter content into a single event.** Each event should represent one clear action or state change.

### CRITICAL: Beats Format
The `beats` field MUST be an array of strings:
```json
"beats": [
  "Then, Lin Yan wakes up in the collapsed mine with no memory",
  "Therefore, he searches for survivors and finds Old Miner Wang trapped",
  "Then, a second collapse kills them both",
  "Therefore, Lin Yan resurrects and realizes he has a unique ability"
]
```

NOT a comma-separated string like:
```json
"beats": "beat1, beat2, beat3"  // WRONG!
```

### ⚠️ CRITICAL: Timeline Requirements (NEW)
**Each chapter MUST include timeline information** to ensure cross-chapter time consistency and detect timeline issues.

**Timeline Fields:**
1. **anchor** (string, REQUIRED): Time anchor relative to story start, e.g., "Day 3 evening", "three months later"
2. **start_time** (string, optional): Specific start time description within the chapter
3. **end_time** (string, optional): Specific end time description within the chapter  
4. **duration** (string, optional): Time elapsed within chapter: "same day", "lasts 3 days", "an instant"
5. **time_jump** (boolean, optional): Whether there's a time jump from previous chapter
6. **previous_gap** (string, optional): Time gap from previous chapter, e.g., "7 days passed", "next morning"
7. **transition** (string, REQUIRED if time_jump=true): Explanation of what happened during the time gap

**Timeline Continuity Rules:**
- Chapter 1 anchor should be "Day 1" or story start time
- Each subsequent chapter's anchor must be chronologically after previous chapter
- If time_jump=true, MUST provide transition explaining the gap
- Duration should match the actual content (don't say "same day" if chapter spans weeks)

**Timeline Example:**
```json
"timeline": {
  "anchor": "第15天黎明",
  "start_time": "黎明时分",
  "end_time": "当夜子时",
  "duration": "当天",
  "time_jump": true,
  "previous_gap": "距离上章过了7天",
  "transition": "林砚在这7天里一直在暗中准备逃亡物资，同时观察监工巡逻规律"
}
```

### Event Types (⚠️ 推荐新格式)

#### 推荐新格式（语义更清晰）
使用 **Actor → Action → Target** 的主谓宾格式：

```json
{
  "actor": "Lin Yan",
  "action": "awaken",
  "target": "death-resurrection ability",
  "target_type": "premise",
  "context": "Abandoned Mine",
  "result": "Lin Yan discovers he can resurrect after death"
}
```

**Action 类型：**
- `acquire` - 获得物品
- `use` - 使用物品  
- `move` / `enter` / `leave` - 移动
- `combat` / `defeat` / `escape` - 战斗
- `learn` / `awaken` / `upgrade` / `master` - 能力
- `discover` / `reveal` - 发现
- `meet` / `befriend` - 关系
- `set` / `progress` / `achieve` - 目标

**TargetType 类型：**
- `item`, `character`, `location`, `skill`, `status`, `relationship`, `knowledge`

#### ⚠️ 战斗事件特殊格式（重要）

对于战斗类型的事件（action 为 `combat` 或 `defeat`），**必须**使用以下结构化格式：

```json
{
  "actor": "林跃",
  "action": "combat",
  "enemies": [
    {"name": "低阶工蜂虫族", "count": 3, "level": 1},
    {"name": "高阶螳螂虫族", "count": 1, "level": 5, "is_boss": true}
  ],
  "allies": ["赵虎", "周明"],
  "context": "黑石聚落防线",
  "result": "全歼虫族潮，成功守住黑石聚落"
}
```

**战斗事件格式要求：**
- `enemies`: 敌人列表（必须）
  - `name`: 敌人名称（字符串）
  - `count`: 敌人数量（数字）
  - `level`: 敌人等级（数字，相对于玩家等级）
  - `is_boss`: 是否是Boss（布尔值，可选）
- `allies`: 盟友列表（可选，如果有NPC参与战斗）
- 不要使用 `target` 和 `target_type` 字段

**非战斗事件**继续使用标准格式（actor/action/target/target_type）。

#### 旧格式（向后兼容）
如果无法确定新格式，可以使用：
- `type`: Event type - one of: relationship, goal, item, premise, storyline, gate, status, memory
- `subject`: Who/what is affected
- `change`: What changed

Example:
```json
{
  "type": "premise",
  "subject": "Lin Yan",
  "change": "Discovers death-resurrection ability"
}
```

## Continuity Requirements

### Volume-Level Continuity
- Chapter 1 must follow from previous volume's ending (if any)
- Each chapter must lead logically to the next
- Last chapter must fulfill the volume's summary

### Beat Continuity
- Chapter N's closing beat must connect to Chapter N+1's opening
- Use "Therefore," or "Then," to show causality
- No time jumps or scene breaks between chapters

## Output Format Example

```json
{
  "chapters": [
    {
      "title": "The Awakening",
      "summary": "Lin Yan discovers his ability in the collapsed mine",
      "characters": ["Lin Yan", "Old Miner Wang"],
      "location": "Abandoned Mine",
      "events": [
        {
          "actor": "Lin Yan",
          "action": "enter",
          "target": "collapsed mine",
          "target_type": "location",
          "context": "after the explosion",
          "result": "Lin Yan enters the dark, unstable mine"
        },
        {
          "actor": "Lin Yan",
          "action": "meet",
          "target": "Old Miner Wang",
          "target_type": "character",
          "context": "deep in the mine",
          "result": "Lin Yan finds Old Miner Wang trapped under debris"
        },
        {
          "actor": "Lin Yan",
          "action": "awaken",
          "target": "death-resurrection ability",
          "target_type": "premise",
          "context": "after the second collapse",
          "result": "Lin Yan discovers he can resurrect after death"
        },
        {
          "actor": "Lin Yan",
          "action": "achieve",
          "target": "escape the mine",
          "target_type": "goal",
          "context": "using his new ability",
          "result": "Lin Yan successfully escapes the mine with his new understanding"
        }
      ],
      "beats": [
        "Then, Lin Yan wakes up in the collapsed mine with no memory",
        "Therefore, he searches for survivors and finds Old Miner Wang trapped",
        "Then, a second collapse kills them both",
        "Therefore, Lin Yan resurrects and realizes he has a unique ability"
      ],
      "opening_beat": "Then, Lin Yan wakes up in the collapsed mine with no memory",
      "closing_beat": "Therefore, Lin Yan resurrects and realizes he has a unique ability",
      "conflict": "Survival in the collapsed mine while discovering his ability",
      "pacing": "fast",
      "timeline": {
        "anchor": "第1天清晨",
        "start_time": "矿难发生后不久",
        "end_time": "当日下午",
        "duration": "当天",
        "time_jump": false,
        "previous_gap": "",
        "transition": ""
      }
    },
    {
      "title": "First Battle",
      "summary": "Lin Yan fights insect swarm with allies",
      "characters": ["Lin Yan", "Zhao Hu", "Zhou Ming"],
      "location": "Black Rock Settlement",
      "events": [
        {
          "actor": "Lin Yan",
          "action": "combat",
          "enemies": [
            {"name": "低阶工蜂虫族", "count": 10, "level": 1},
            {"name": "高阶螳螂虫族", "count": 1, "level": 3, "is_boss": true}
          ],
          "allies": ["赵虎", "周明"],
          "context": "黑石聚落防线",
          "result": "全歼虫族潮，成功守住聚落"
        }
      ],
      "beats": [
        "Then, insect swarm approaches the settlement",
        "Therefore, Lin Yan organizes defense with allies",
        "Then, fierce battle begins with Lin Yan facing the boss insect",
        "Therefore, they defeat the swarm and secure the settlement"
      ],
      "opening_beat": "Then, insect swarm approaches the settlement",
      "closing_beat": "Therefore, they defeat the swarm and secure the settlement",
      "conflict": "Defending settlement from insect swarm",
      "pacing": "fast",
      "timeline": {
        "anchor": "第3天黄昏",
        "start_time": "日落时分",
        "end_time": "深夜",
        "duration": "数个时辰",
        "time_jump": true,
        "previous_gap": "过了2天",
        "transition": "林砚在这两天里熟悉了复活能力，并与赵虎、周明建立了信任关系"
      }
    }
  ]
}
```

## Guidelines

1. **Volume Focus**: All chapters must serve the volume's summary
2. **Progressive Tension**: Tension should build throughout the volume
3. **Character Consistency**: Characters must act consistently with their established traits
4. **Event Tracking**: Use events to track all state changes
5. **Beat Quality**: Each beat must advance the plot or reveal character
6. **First Chapter Hook**: Chapter 1 must immediately engage the reader
7. **Last Chapter Cliffhanger**: Final chapter should set up the next volume
8. **Event Progression**: Events should follow a logical flow: enter/meet → discover/learn → combat/achieve
9. **⚠️ EVENT COUNT**: Each chapter MUST have 3-5 events. This is a HARD requirement.
