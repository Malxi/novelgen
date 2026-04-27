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
6. **scenes**: Scene breakdown (array, 2-3 entries). Each scene contains its own `beats` array.


7. **mysteries**: Planted and resolved mysteries (object) - ⚠️ REQUIRED for suspense tracking

8. **conflict**: Core conflict description (string)
9. **pacing**: slow/normal/fast (string)
10. **timeline**: Chapter timeline information (object) - ⚠️ REQUIRED for timeline consistency
11. **state_anchor**: Protagonist state at chapter START (object) - ⚠️ REQUIRED for cross-chapter state tracking
12. **enemies**: Enemy list for this chapter (array) - ⚠️ REQUIRED if chapter has combat
13. **resource_ledger**: Resource changes this chapter (array) - ⚠️ REQUIRED if resources change

### ⚠️ CRITICAL: Events Count Requirement
**Each chapter MUST have 3-5 events.** This is a HARD requirement.

Each event represents a distinct plot beat or state change in the chapter:
- Event 1: Opening/Discovery (enter location, meet character, discover item)
- Event 2: Development/Conflict (learn skill, face obstacle, transform status)
- Event 3: Climax/Resolution (combat, achieve goal, establish relationship)
- Event 4-5 (optional): Aftermath/Setup (move to new location, gain reward, set new goal)

**DO NOT compress all chapter content into a single event.** Each event should represent one clear action or state change.

### Beats Live Inside Scenes
Each scene has its own `beats` array (1-2 beats per scene). Do NOT output a chapter-level `beats` field — beats belong to scenes.

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

### ⚠️ CRITICAL: State Anchor Requirements (NEW)
**Each chapter MUST declare state_anchor** — the protagonist's expected state at chapter START. This creates a state baseline that the write agent follows and the validator checks across chapters.

**State Anchor Fields:**
1. **cultivation** (string): Realm/power level, e.g. "炼气三层", "S级进化者一阶"
2. **spirit_stones** (int): Resource count, e.g. 37. **Must be numeric — no ranges.**
3. **allies** (array): Current allies/companions at chapter start
4. **injuries** (array): Active injuries at chapter start. Empty array if none.
5. **location** (string): Chapter start location
6. **key_items** (array): Key items currently held
7. **notes** (string): Any other trackable state

**Cross-Chapter Consistency Rules:**
- Chapter 1/spirit_stones sets the baseline count
- If spirit_stones changes, there MUST be a corresponding event (acquire/lose item)
- If cultivation changes, there MUST be a breakthrough/evolution event
- If injuries are cleared, there MUST be a recovery event
- If allies are added, there MUST be a meet/relationship event

**State Anchor Example:**
```json
"state_anchor": {
  "cultivation": "炼气三层",
  "spirit_stones": 37,
  "allies": ["赵虎", "周明"],
  "injuries": [],
  "location": "黑风矿丙字三号矿道",
  "key_items": ["鹤嘴锄", "残破矿灯"],
  "notes": ""
}
```

### ⚠️ CRITICAL: Enemy List (NEW)
**Every chapter with combat events MUST declare enemies.** This gives the write agent exact enemy specs to use.

**Enemy Fields:**
- **name** (string, REQUIRED): 敌人名称，如 "虫族工蜂"
- **faction** (string, REQUIRED): 所属阵营ID，如 "zerg"、"ai_mech"。同一阵营的tier应有自然排序
- **tier** (string, REQUIRED): 阵营内等级标识，如 "drone"、"soldier"、"elite"、"queen"
- **count** (int, REQUIRED): 出现数量
- **level** (int, optional): 敌人等级（可从faction tier定义中查表）
- **is_boss** (bool, optional): 是否是boss
- **boss_id** (string, REQUIRED if is_boss=true): boss唯一ID，跨章追踪用，如 "boss_queen"
- **status** (string, REQUIRED if is_boss=true): new(首次出场)/engaged(战斗中)/defeated(击败)/escaped(逃脱)
- **context** (string, optional): 出现场景，如 "机甲库伏击"

**Example:**
```json
"enemies": [
  {"name": "虫族工蜂", "faction": "zerg", "tier": "drone", "count": 3, "level": 1, "context": "机甲库伏击"},
  {"name": "虫族母虫", "faction": "zerg", "tier": "queen", "count": 1, "level": 5, "is_boss": true, "boss_id": "boss_queen", "status": "engaged"}
]
```

### ⚠️ CRITICAL: Resource Ledger (NEW)
**Track numeric resource changes per chapter.** Validator checks start+delta=end and cross-chapter continuity.

**Ledger Fields:**
- **item** (string): 资源名称
- **start** (int): 本章开始时的数量
- **delta** (int): 变化量（正=获得，负=消耗）
- **end** (int): 本章结束时的数量（必须 = start + delta）
- **reason** (string): 变化原因

**Example:**
```json
"resource_ledger": [
  {"item": "灵石", "start": 37, "delta": 3, "end": 40, "reason": "矿道挖掘"},
  {"item": "灵石", "start": 40, "delta": -2, "end": 38, "reason": "购买丹药"}
]
```

### ⚠️ CRITICAL: Scene Splitting (NEW)
**Each chapter should split into 2-3 scenes.** Scenes give the write agent focused targets.

**Scene Fields:**
- **order** (int): 序号，从1开始
- **pov** (string, REQUIRED): 视角角色
- **goal** (string, REQUIRED): 场景目标（这个场景要推进什么）
- **location** (string): 场景地点
- **characters** (array): 本场景角色
- **beats** (array, REQUIRED): 本场景的1-2个plot beats（自然中文，不要连接词前缀）
- **words** (int, optional): 建议字数
- **tone** (string, optional): 情绪基调

**Example:**
```json
"scenes": [
  {"order": 1, "pov": "林野", "goal": "潜入矿道深处挖灵石", "location": "丙字三号矿道", "characters": ["林野", "老陈"], "beats": ["用鹤嘴锄敲开矿壁发现三块灵石", "老陈提醒巡逻监工即将经过"], "words": 1200, "tone": "紧张"},
  {"order": 2, "pov": "林野", "goal": "与赵虎密谋逃跑计划", "location": "矿奴大通铺", "characters": ["林野", "赵虎"], "beats": ["赵虎透露废矿道深处有暗河通往矿场外", "两人约好三天后趁夜行动"], "words": 1800, "tone": "压抑"}
]
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
      ],
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
      },
      "state_anchor": {
        "cultivation": "凡人/未觉醒",
        "spirit_stones": 0,
        "allies": [],
        "injuries": ["矿难擦伤"],
        "location": "黑风矿坍塌矿道",
        "key_items": ["碎矿镐"],
        "notes": "刚穿越，身体虚弱，记忆模糊"
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
      ],
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
      },
      "state_anchor": {
        "cultivation": "觉醒·初级复活能力",
        "spirit_stones": 5,
        "allies": ["赵虎", "周明"],
        "injuries": [],
        "location": "黑石聚落防线",
        "key_items": ["虫壳碎片", "矿镐"],
        "notes": "已掌握复活能力基本用法，与赵虎周明建立信任"
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
7. **⚠️ EVENT COUNT**: Each chapter MUST have 3-5 events. This is a HARD requirement.

### ⚠️ CRITICAL: Mysteries / Suspense Tracking (NEW)
**Track planted and resolved mysteries across chapters.** Validator checks: unresolved at end, resolved-without-planted, same-chapter plant+resolve.

**Fields:**
```json
"mysteries": {
  "planted": [
    {"id": "myst_why_woken", "clue": "星核日志显示三天前有人远程发送了激活指令，唤醒不是意外"}
  ],
  "resolved": [
    {"id": "myst_origin_gene", "resolution": "解密休眠基地主控记录：林越的基因适配是联邦秘密计划预设的"}
  ]
}
```

**Rules:**
- `id` must use `myst_` prefix, unique across the entire volume
- A mystery resolved in chapter N must have been planted in some earlier chapter
- Do NOT plant and resolve in the same chapter (unless it's a minor beat)
- Unresolved mysteries at the volume's end produce an info-level suggestion
