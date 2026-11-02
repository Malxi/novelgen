package rpg

import (
	"fmt"
	"strings"
)

func (ov *OutlineValidator) validateEnemies() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.Name == "" {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "enemy", Severity: "major", Location: chapter.ID,
							Description: "敌人清单中存在空名称",
							Fix:         "为每个敌人设置明确的名称",
						})
					}
					if enemy.Count <= 0 {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "enemy", Location: chapter.ID,
							Description: fmt.Sprintf("敌人 '%s' 数量为 %d，已自动设为1", enemy.Name, enemy.Count),
							Suggestion:  "明确敌人的出现数量",
						})
					}
				}
				// Check: if chapter has combat events but no enemies listed
				hasCombat := false
				for _, evt := range chapter.Events {
					if evt.Type == "combat" || evt.Action == "combat" {
						hasCombat = true
						break
					}
				}
				if hasCombat && len(chapter.Enemies) == 0 {
					ov.Warnings = append(ov.Warnings, OutlineWarning{
						Type: "enemy", Location: chapter.ID,
						Description: "章节有 combat 事件但未声明敌人清单",
						Suggestion:  "在 enemies 中列出本章出现的敌人及其数量",
					})
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateResourceLedger() {
	var prevLedger map[string]int // item → end count from previous chapter

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if len(chapter.ResourceLedger) == 0 {
					prevLedger = nil
					continue
				}
				if prevLedger == nil {
					prevLedger = make(map[string]int)
				}

				for _, entry := range chapter.ResourceLedger {
					// Arithmetic check: Start + Delta should equal End
					expected := entry.Start + entry.Delta
					if expected != entry.End {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "resource_ledger", Severity: "critical",
							Location:    chapter.ID,
							Description: fmt.Sprintf("资源 '%s' 账目不对: %d + %d = %d ≠ %d", entry.Item, entry.Start, entry.Delta, expected, entry.End),
							Fix:         fmt.Sprintf("修正 start/delta/end 使等式成立: %d + %d = %d", entry.Start, entry.Delta, entry.Start+entry.Delta),
						})
					}

					// Continuity: this chapter's Start should match previous chapter's End
					if prevEnd, ok := prevLedger[entry.Item]; ok && prevEnd != entry.Start {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "resource_ledger", Location: chapter.ID,
							Description: fmt.Sprintf("资源 '%s' 与上一章不一致: 上一章结束=%d, 本章开始=%d", entry.Item, prevEnd, entry.Start),
							Suggestion:  fmt.Sprintf("确保本章 start 等于上一章 end (%d)", prevEnd),
						})
					}
					prevLedger[entry.Item] = entry.End
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateScenes() {
	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if len(chapter.Scenes) == 0 {
					ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
						Type: "scenes", Location: chapter.ID,
						Current:   "章节未拆分为场景",
						Suggested: "将章节拆分为2-3个场景，每个场景指定POV/目标/地点/字数",
						Reason:    "场景拆分帮助写作者聚焦每个场景的目标，避免跑题或漏角色",
					})
					continue
				}

				// Check scene order continuity
				for i, scene := range chapter.Scenes {
					if scene.Order != i+1 {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景序号不连续: 期望 %d, 实际 %d", i+1, scene.Order),
							Suggestion:  "确保场景序号从1开始递增",
						})
					}
					if scene.POV == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景 %d 缺少POV角色", scene.Order),
							Suggestion:  "为每个场景指定视角角色",
						})
					}
					if scene.Goal == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "scenes", Location: chapter.ID,
							Description: fmt.Sprintf("场景 %d 缺少目标(goal)", scene.Order),
							Suggestion:  "明确这个场景要推进什么、达成什么",
						})
					}
				}

				// Scene characters should be a subset of chapter characters
				chapterChars := make(map[string]bool)
				for _, name := range chapter.Characters {
					chapterChars[name] = true
				}
				for _, scene := range chapter.Scenes {
					for _, name := range scene.Characters {
						if !chapterChars[name] {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "scenes", Location: chapter.ID,
								Description: fmt.Sprintf("场景角色 '%s' 不在章节角色列表中", name),
								Suggestion:  fmt.Sprintf("将 '%s' 添加到章节角色列表", name),
							})
						}
					}
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateMysteries() {
	planted := make(map[string]string) // id → chapter where planted
	resolved := make(map[string]bool)  // id → resolved

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, p := range chapter.Mysteries.Planted {
					if p.ID == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: "planted谜题缺少ID",
							Suggestion:  "为每个planted谜题设置唯一ID",
						})
						continue
					}
					if prevCh, ok := planted[p.ID]; ok {
						ov.Issues = append(ov.Issues, OutlineIssue{
							Type: "mysteries", Severity: "major", Location: chapter.ID,
							Description: fmt.Sprintf("谜题 '%s' 已在 %s 中planted，重复planted", p.ID, prevCh),
							Fix:         "确认是追加线索还是新谜题，追加线索应使用不同的ID",
						})
					}
					planted[p.ID] = chapter.ID
				}
				for _, r := range chapter.Mysteries.Resolved {
					if r.ID == "" {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: "resolved谜题缺少ID",
							Suggestion:  "为每个resolved谜题设置唯一ID",
						})
						continue
					}
					if _, wasPlanted := planted[r.ID]; !wasPlanted {
						ov.Warnings = append(ov.Warnings, OutlineWarning{
							Type: "mysteries", Location: chapter.ID,
							Description: fmt.Sprintf("谜题 '%s' 被resolved但之前从未被planted", r.ID),
							Suggestion:  "在前面章节中添加对应的planted，或修改ID",
						})
					}
					resolved[r.ID] = true
				}
			}
		}
	}

	if len(planted) > 0 {
		unresolved := make([]string, 0)
		for id := range planted {
			if !resolved[id] {
				unresolved = append(unresolved, id)
			}
		}
		if len(unresolved) > 0 {
			ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
				Type:      "mysteries",
				Current:   fmt.Sprintf("存在%d个未回收的伏笔", len(unresolved)),
				Suggested: fmt.Sprintf("确认这些伏笔是否计划在后续章节回收: %s", strings.Join(unresolved, ", ")),
				Reason:    "未回收的伏笔可能导致读者感觉故事不完整",
			})
		}
	}
}

func (ov *OutlineValidator) validateBossContinuity() {
	bosses := make(map[string]string) // boss_id → last seen chapter + status

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.BossID == "" {
						continue
					}
					status := enemy.Status
					if status == "" {
						status = "engaged"
					}

					if prev, ok := bosses[enemy.BossID]; ok {
						parts := strings.SplitN(prev, "|", 2)
						prevCh, prevStatus := parts[0], ""
						if len(parts) > 1 {
							prevStatus = parts[1]
						}

						// defeated after engaged = normal progression
						if prevStatus == "defeated" || prevStatus == "escaped" {
							ov.Issues = append(ov.Issues, OutlineIssue{
								Type: "boss_continuity", Severity: "major",
								Location:    chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 在 %s 已 %s，本章不应再出现", enemy.Name, prevCh, prevStatus),
								Fix:         "如果boss未死，请将状态改为escaped并说明去向；如果死亡，请移除后续章节的出场",
							})
						}

						// engaged → engaged without notes = missing transition
						if prevStatus == "engaged" && status == "engaged" {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "boss_continuity", Location: chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 跨章持续战斗中 (%s → %s)，但state_anchor缺少残血说明", enemy.Name, prevCh, chapter.ID),
								Suggestion:  fmt.Sprintf("在 %s 的 state_anchor.notes 中添加boss当前状态，如：'%s残血，甲壳碎裂'", chapter.ID, enemy.Name),
							})
						}
					} else {
						if status == "defeated" || status == "escaped" {
							ov.Warnings = append(ov.Warnings, OutlineWarning{
								Type: "boss_continuity", Location: chapter.ID,
								Description: fmt.Sprintf("Boss '%s' 首次出场即 %s，缺少前置 engaged 章节", enemy.Name, status),
								Suggestion:  "如果是首次出场即被击败，请将status改为new并在前一章或本章前半部分铺垫",
							})
						}
					}

					bosses[enemy.BossID] = chapter.ID + "|" + status
				}
			}
		}
	}
}

func (ov *OutlineValidator) validateFactionTiers() {
	// Collect faction→tier→first_appearance mapping
	type tierInfo struct {
		chapter   string
		enemyName string
	}
	factions := make(map[string]map[string]tierInfo)

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				for _, enemy := range chapter.Enemies {
					if enemy.Faction == "" {
						continue
					}
					if factions[enemy.Faction] == nil {
						factions[enemy.Faction] = make(map[string]tierInfo)
					}
					if _, seen := factions[enemy.Faction][enemy.Tier]; !seen {
						factions[enemy.Faction][enemy.Tier] = tierInfo{chapter: chapter.ID, enemyName: enemy.Name}
					}
				}
			}
		}
	}

	for faction, tiers := range factions {
		if len(tiers) == 0 {
			continue
		}

		// Suggest defining faction tiers in story setup
		var tierNames []string
		for t := range tiers {
			tierNames = append(tierNames, t)
		}
		if len(tierNames) > 1 {
			ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
				Type:      "faction_tier",
				Location:  faction,
				Current:   fmt.Sprintf("阵营 '%s' 使用了以下tier: %s", faction, strings.Join(tierNames, ", ")),
				Suggested: fmt.Sprintf("在 story_setup.json 的 premises 中定义 '%s' 阵营的完整等级体系", faction),
				Reason:    "统一管理阵营等级体系，便于validator做跨章一致性检查",
			})
		}

		// Check: enemy name should match [faction]_[tier] pattern for consistency
		for tier, info := range tiers {
			if tier == "" {
				ov.Warnings = append(ov.Warnings, OutlineWarning{
					Type: "faction_tier", Location: info.chapter,
					Description: fmt.Sprintf("敌人 '%s' 有faction但没有tier", info.enemyName),
					Suggestion:  fmt.Sprintf("为该敌人指定tier，如：%s_drone", info.enemyName),
				})
			}
		}
	}
}

// validateStorylineTexture adds soft hints for chapters that would benefit from
// clearer arc movement. It never creates issues or warnings.
func (ov *OutlineValidator) validateStorylineTexture() {
	if ov.Outline == nil {
		return
	}

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				hasStorylineEvent := false
				for _, event := range chapter.Events {
					if event.Type == "storyline" || event.GetTargetType() == "storyline" {
						hasStorylineEvent = true
						break
					}
				}

				if hasStorylineEvent && len(chapter.StorylineAdvances) == 0 {
					ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
						Type:      "storyline_texture",
						Location:  chapter.ID,
						Current:   "chapter has storyline event but no storyline_advances",
						Suggested: "Optionally add one storyline_advances entry if the chapter creates a real reveal, pressure, choice, or consequence.",
						Reason:    "This keeps important story arcs from becoming thin while staying optional for chapters where the event is self-evident.",
					})
				}

				for _, advance := range chapter.StorylineAdvances {
					if strings.TrimSpace(advance.StorylineName) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   "storyline_advances entry has no storyline_name",
							Suggested: "Name the setup storyline being moved, or remove the entry if it does not map to a meaningful arc.",
							Reason:    "A named arc helps later agents carry pressure and payoff forward without forcing a rigid template.",
						})
					}
					if strings.TrimSpace(advance.Change) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   "storyline_advances entry has no change",
							Suggested: "Describe the concrete shift in the arc, such as a reveal, setback, choice, escalation, or payoff.",
							Reason:    "A storyline note is useful only when it records an actual dramatic movement.",
						})
						continue
					}
					if strings.TrimSpace(advance.Consequence) == "" && strings.TrimSpace(advance.Pressure) == "" {
						ov.Suggestions = append(ov.Suggestions, OutlineSuggestion{
							Type:      "storyline_texture",
							Location:  chapter.ID,
							Current:   advance.Change,
							Suggested: "If useful, add either consequence or pressure so the change creates forward energy.",
							Reason:    "Storyline advancement is stronger when it changes what characters risk, know, want, or must do next.",
						})
					}
				}
			}
		}
	}
}

// deathInfo 角色死亡记录
type deathInfo struct {
	chapterID string
	how       string
}

// 死亡关键词（用于 defeat/combat 结果与 kill/die/devour/swallow 动作）
var outlineDeathKeywords = []string{"吞噬", "死亡", "击杀", "杀死", "陨落", "毙命", "灰飞烟灭", "身亡"}

// 文本死亡关键词（details/result 中提到角色名 + 以下关键词判定死亡）
var outlineDeathTextKeywords = []string{"吞噬成功", "已死", "死亡", "陨落", "灰飞烟灭"}

// 非真实死亡语境：模拟/推演/预演/幻境/梦境/测试中的"死亡"不算死亡
var outlineNonDeathContexts = []string{"模拟死亡", "推演死亡", "预演死亡", "幻境中死亡", "梦里死亡", "梦境死亡", "模拟器", "推演中", "预演中", "幻境", "梦境", "模拟场景", "测试死亡"}

// 吞噬类能力/系统词：出现这些词时"吞噬"是能力名而非死亡动作
var outlineDevourAbilityWords = []string{"吞噬系统", "吞噬能力", "吞噬外挂", "吞噬属性", "吞噬之力", "吞噬功法", "吞噬能量", "吞噬气运"}

// outlineTextHasNonDeathContext 判断文本是否处于非真实死亡语境（模拟/推演/幻境等）
func outlineTextHasNonDeathContext(text string) bool {
	for _, ctx := range outlineNonDeathContexts {
		if strings.Contains(text, ctx) {
			return true
		}
	}
	return false
}

// outlineTextHasDevourAbility 判断文本中的"吞噬"是否只是能力/系统名（非死亡动作）
func outlineTextHasDevourAbility(text string) bool {
	for _, w := range outlineDevourAbilityWords {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

// validateCharacterDeathContinuity 角色死亡/复活连续性检查
// 扫描全部章节，记录已死亡角色；后续章节中该角色再次出场即报 major 问题；
// 若后续事件明确复活该角色，则清除死亡标记。
func (ov *OutlineValidator) validateCharacterDeathContinuity() {
	if ov.Outline == nil {
		return
	}

	deaths := make(map[string]deathInfo) // 规范化角色名 → 死亡信息
	flagged := make(map[string]bool)     // 角色名|章节 → 已报过

	for _, part := range ov.Outline.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				chapterChars := outlineChapterCharacterNames(chapter)

				// 先处理本章复活事件：复活后同章出场不报问题
				for _, evt := range chapter.Events {
					// 时间线重置/回档 = 群体复活，清空全部死亡标记
					evtText := normalizeOutlineValidatorText(evt.Details + " " + evt.Result + " " + evt.Change)
					if strings.Contains(evtText, "时间线重置") || strings.Contains(evtText, "回档") {
						deaths = make(map[string]deathInfo)
						break
					}
					for _, revived := range outlineEventRevivedCharacters(evt, chapterChars) {
						delete(deaths, revived)
					}
				}

				// 检查本章出场的已死亡角色
				for _, name := range chapterChars {
					info, ok := deaths[name]
					if !ok {
						continue
					}
					key := name + "\x00" + chapter.ID
					if flagged[key] {
						continue
					}
					flagged[key] = true
					ov.Issues = append(ov.Issues, OutlineIssue{
						Type:        "death_continuity",
						Severity:    "major",
						Location:    chapter.ID,
						Description: fmt.Sprintf("角色 %s 已在 %s 死亡/被吞噬，本章不应再出场", name, info.chapterID),
						Impact:      "死亡角色再次出场会破坏死亡事件的严肃性与剧情连贯性",
						Fix:         "为该角色补充复活/重生事件，或移除本章中的出场",
					})
				}

				// 按事件顺序更新死亡/复活标记
				for _, evt := range chapter.Events {
					for _, victim := range outlineEventDeadVictims(evt, chapterChars) {
						deaths[victim] = deathInfo{chapterID: chapter.ID, how: normalizeOutlineValidatorText(evt.GetAction())}
					}
					for _, revived := range outlineEventRevivedCharacters(evt, chapterChars) {
						delete(deaths, revived)
					}
				}
			}
		}
	}
}

// outlineChapterCharacterNames 收集章节中所有出场角色名（结构化字段，不含 summary）
func outlineChapterCharacterNames(chapter StoryChapter) []string {
	var names []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range chapter.Characters {
		add(name)
	}
	for _, evt := range chapter.Events {
		for _, name := range evt.Characters {
			add(name)
		}
		add(evt.GetActor())
		add(evt.GetTarget())
		for _, enemy := range evt.Enemies {
			add(enemy.Name)
		}
	}
	for _, enemy := range chapter.Enemies {
		add(enemy.Name)
	}
	return names
}

// outlineEventDeadVictims 判定事件中的死亡角色
// 概念/机制名：出现在 events 里但不是真实角色（不应参与死亡追踪）
var outlineNonCharacterConcepts = []string{"死亡回档", "回档", "时间线", "日志系统", "系统", "天道", "规则", "世界意志"}

// outlineIsConceptName 判断名字是否为概念/机制名（非真实角色）
func outlineIsConceptName(name string) bool {
	for _, c := range outlineNonCharacterConcepts {
		if name == c {
			return true
		}
	}
	return false
}

func outlineEventDeadVictims(evt StoryEvent, chapterChars []string) []string {
	var victims []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !outlineIsConceptName(name) && !contains(victims, name) {
			victims = append(victims, name)
		}
	}

	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	target := normalizeOutlineName(evt.GetTarget())
	change := normalizeOutlineName(evt.Change)
	text := normalizeOutlineValidatorText(evt.Details + " " + evt.Result)

	// 文本层守卫：非真实死亡语境（模拟/推演/幻境/梦境）中的"死亡"不算死亡。
	// 结构化字段（action/change）是主信号，因此该守卫现在很少触发。
	if outlineTextHasNonDeathContext(text) {
		return victims
	}
	// 文本层守卫："吞噬"仅是能力/系统名（吞噬系统/吞噬能力/吞噬外挂）时不算死亡动作。
	if outlineTextHasDevourAbility(text) {
		return victims
	}

	// 1. 结构化动作：kill/die/devour/swallow/defeat 直接判定死亡。
	//    die 类动作死亡的是执行者，其余动作死亡的是目标。
	switch {
	case strings.Contains(action, "die"):
		add(evt.GetActor())
	case strings.Contains(action, "kill"), strings.Contains(action, "devour"),
		strings.Contains(action, "swallow"), strings.Contains(action, "defeat"):
		add(target)
	}

	// 2. 结构化 change 明确为死亡结果：目标死亡（若 actor==target 且为自我死亡则记执行者）。
	if outlineChangeImpliesDeath(change) {
		if outlineChangeImpliesSelfDeath(change) && target != "" && normalizeOutlineName(evt.GetActor()) == target {
			add(evt.GetActor())
		} else {
			add(target)
		}
	}

	// 3. change 含"吞噬"且目标是真实角色名（且文本未把它当能力名）→ 目标被吞噬死亡。
	if strings.Contains(change, "吞噬") && target != "" && !outlineTextHasDevourAbility(text) {
		add(target)
	}

	// 4. 仅作为兜底：combat/defeat 动作 + 结果文本含死亡关键词。
	//    文本路径必须通过上面的非真实死亡语境守卫；不再扫描章节角色名共现。
	if (action == "combat" || action == "defeat") &&
		containsAnyOutlineValidatorText(text, outlineDeathTextKeywords...) {
		if outlineEventActorDied(evt, text) {
			add(evt.GetActor())
		} else {
			add(target)
		}
	}

	return victims
}

// outlineChangeImpliesDeath 判断结构化的 change 是否为死亡结果。
func outlineChangeImpliesDeath(change string) bool {
	switch strings.ToLower(strings.TrimSpace(change)) {
	case "temporary_death", "defeated", "died", "dead", "killed",
		"死亡", "战死", "身亡", "陨落", "被吞噬", "吞噬成功":
		return true
	}
	return false
}

// outlineChangeImpliesSelfDeath 判断该 change 是否表示执行者自身的死亡（actor==target 时适用）。
func outlineChangeImpliesSelfDeath(change string) bool {
	switch strings.ToLower(strings.TrimSpace(change)) {
	case "temporary_death", "died", "dead", "killed",
		"死亡", "战死", "身亡", "陨落", "被吞噬", "吞噬成功":
		return true
	}
	return false
}

// outlineEventActorDied 判断事件文本是否明确表明执行者死亡
func outlineEventActorDied(evt StoryEvent, text string) bool {
	actor := normalizeOutlineName(evt.GetActor())
	if actor == "" {
		return false
	}
	actorLower := normalizeOutlineValidatorText(actor)
	if strings.Contains(text, actorLower+"被") && containsAnyOutlineValidatorText(text, outlineDeathKeywords...) {
		return true
	}
	return containsAnyOutlineValidatorText(text, actorLower+"已死", actorLower+"身亡", actorLower+"陨落")
}

var outlineReviveKeywords = []string{"revive", "复活", "重生", "时间线重置", "回档"}

// outlineEventRevivedCharacters 返回复活事件中被复活的角色
// 判定依据：action 含 revive/复活/重生，或事件文本（change/details/result）含复活/重生标记
// 且文本中出现角色名（结构化字段优先，其次按文本共现归因）。
func outlineEventRevivedCharacters(evt StoryEvent, chapterChars []string) []string {
	action := strings.ToLower(normalizeOutlineName(evt.GetAction()))
	text := normalizeOutlineValidatorText(strings.Join([]string{evt.Change, evt.Details, evt.Result}, " "))
	if !containsAnyOutlineValidatorText(action, outlineReviveKeywords...) &&
		!containsAnyOutlineValidatorText(text, "复活", "重生") {
		return nil
	}
	var revived []string
	add := func(name string) {
		name = normalizeOutlineName(name)
		if name != "" && !contains(revived, name) {
			revived = append(revived, name)
		}
	}
	add(evt.GetTarget())
	add(evt.GetActor())
	for _, name := range evt.Characters {
		add(name)
	}
	// 事件文本中同现角色名与复活/重生标记
	if containsAnyOutlineValidatorText(text, "复活", "重生") {
		for _, name := range chapterChars {
			if strings.Contains(text, normalizeOutlineValidatorText(name)) {
				add(name)
			}
		}
	}
	return revived
}

// cultivationSighting 角色修为记录
type cultivationSighting struct {
	raw     string
	tier    int
	layer   int
	phase   int
	parsed  bool
	chapter string
	index   int
}

// 修为大境界（由低到高）与小境界（由低到高）
var outlineCultivationTiers = []string{"练气", "筑基", "金丹", "元婴", "化神", "炼虚", "合体", "大乘", "渡劫"}
var outlineCultivationPhases = []string{"大圆满", "巅峰", "后期", "中期", "初期"}

var outlineBreakthroughKeywords = []string{"突破", "breakthrough", "进阶", "晋升", "升级", "upgrade", "觉醒", "awaken", "进化", "evolution"}
var outlineDowngradeKeywords = []string{"跌境", "跌落", "修为倒退", "修为被废", "反噬修为受损", "掉到", "跌至", "降至", "压制", "被废", "反噬损失修为"}

// validateCultivationContinuity 全角色修为单调性检查
// 来源：chapter.StateAnchor.Cultivation（仅主角）与 chapter.StateChange 自然语言（任意角色）。
// 修为下降且缺少跌境事件报 major；修为上升且缺少突破事件报 minor 警告。
