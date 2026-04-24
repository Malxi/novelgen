package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"novelgen/internal/rpg/dsl"
)

type recapRecord struct {
	ChapterID  string   `json:"chapter_id"`
	Title      string   `json:"title"`
	Time       string   `json:"time"`
	Present    []string `json:"present"`
	PlotBeats  []string `json:"plot_beats"`
	Reveals    []string `json:"reveals"`
	Unresolved []string `json:"unresolved"`
	Status     []string `json:"status"`
	Items      []string `json:"items"`
}

type deathCostSnapshot struct {
	ChapterID    string
	HasDeath     bool
	HasLevelDrop bool
	HasLifespan  bool
	NoLifespan   bool
	FromLevel    int
	ToLevel      int
}

func buildCrossChapterConsistencyIssues(chapterFiles []string, rules *consistencyRules) []dsl.SimulationIssue {
	recaps := loadRecaps(chapterFiles)
	if len(recaps) == 0 {
		return nil
	}
	if rules == nil {
		rules = defaultConsistencyRules()
	}

	issues := make([]dsl.SimulationIssue, 0)
	if moduleEnabled(rules.Modules.TimeGapTransition, true) {
		issues = append(issues, checkTimeGapTransition(recaps, rules)...)
	}
	if moduleEnabled(rules.Modules.ResurrectionConsistency, true) {
		issues = append(issues, checkResurrectionConsistency(recaps, rules)...)
	}
	if moduleEnabled(rules.Modules.CultivationLifespan, true) {
		issues = append(issues, checkCultivationLifespanCoupling(recaps)...)
	}
	if moduleEnabled(rules.Modules.NPCIdentityConflict, true) {
		issues = append(issues, checkNPCIdentityConflict(recaps)...)
	}
	if moduleEnabled(rules.Modules.InjuryRecoveryContinuity, true) {
		issues = append(issues, checkInjuryRecoveryContinuity(recaps)...)
	}
	if moduleEnabled(rules.Modules.ResourceBudgetConsistency, true) {
		issues = append(issues, checkResourceBudgetConsistency(recaps, rules)...)
	}
	return issues
}

func loadRecaps(chapterFiles []string) []recapRecord {
	type item struct {
		file  string
		recap recapRecord
	}
	rows := make([]item, 0, len(chapterFiles))
	for _, f := range chapterFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var r recapRecord
		if err := json.Unmarshal(b, &r); err != nil {
			continue
		}
		if strings.TrimSpace(r.ChapterID) == "" {
			r.ChapterID = chapterIDFromFile(f)
		}
		rows = append(rows, item{file: f, recap: r})
	}

	sort.Slice(rows, func(i, j int) bool {
		a1, b1, c1 := parsePVC(rows[i].recap.ChapterID)
		a2, b2, c2 := parsePVC(rows[j].recap.ChapterID)
		if a1 != a2 {
			return a1 < a2
		}
		if b1 != b2 {
			return b1 < b2
		}
		if c1 != c2 {
			return c1 < c2
		}
		return rows[i].recap.ChapterID < rows[j].recap.ChapterID
	})

	out := make([]recapRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.recap)
	}
	return out
}

func chapterIDFromFile(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return base
}

func parsePVC(chID string) (int, int, int) {
	re := regexp.MustCompile(`P(\d+)-V(\d+)-C(\d+)`)
	m := re.FindStringSubmatch(chID)
	if len(m) != 4 {
		return 9999, 9999, 9999
	}
	p, _ := strconv.Atoi(m[1])
	v, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return p, v, c
}

func checkTimeGapTransition(recaps []recapRecord, rules *consistencyRules) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	for i := 1; i < len(recaps); i++ {
		prev := recaps[i-1]
		cur := recaps[i]
		days := inferElapsedDays(cur.Time)
		if days < rules.Thresholds.TimeGapDays {
			continue
		}

		joined := chapterText(cur)
		hasTransition := containsAny(joined, rules.Keywords.TransitionHints...)
		if hasTransition {
			continue
		}

		issues = append(issues, dsl.SimulationIssue{
			Type:        dsl.IssueContinuity,
			Severity:    dsl.SeverityWarning,
			Chapter:     cur.ChapterID,
			Description: fmt.Sprintf("章节时间跨度约 %d 天，但缺少明确过渡描写（上一章: %s）", days, prev.ChapterID),
			Suggestion:  "建议补 1-2 段过渡，说明这段时间的生存、修炼、关系变化",
		})
	}
	return issues
}

func checkResurrectionConsistency(recaps []recapRecord, rules *consistencyRules) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	snaps := make([]deathCostSnapshot, 0)

	floorRuleSeen := false
	for _, r := range recaps {
		text := chapterText(r)
		if containsAny(text, rules.Keywords.FloorRuleHints...) {
			floorRuleSeen = true
		}
		snaps = append(snaps, extractDeathCostSnapshot(r))
	}

	kindSet := map[string]bool{}
	for _, s := range snaps {
		if !s.HasDeath {
			continue
		}
		kind := "none"
		switch {
		case s.HasLevelDrop && s.HasLifespan:
			kind = "both"
		case s.HasLevelDrop:
			kind = "level_only"
		case s.HasLifespan:
			kind = "lifespan_only"
		}
		kindSet[kind] = true
	}

	if len(kindSet) >= rules.Thresholds.ResurrectionDiversityMin {
		issues = append(issues, dsl.SimulationIssue{
			Type:        dsl.IssueLogic,
			Severity:    dsl.SeverityWarning,
			Description: "多次死亡代价模式不一致（修为掉级/寿元折损组合波动较大）",
			Suggestion:  "建议在设定中明确死亡代价分支条件，并在对应章节显式触发条件",
		})
	}

	if floorRuleSeen {
		for _, s := range snaps {
			if !s.HasDeath || !s.NoLifespan {
				continue
			}
			if s.FromLevel >= 2 && s.ToLevel == 1 {
				issues = append(issues, dsl.SimulationIssue{
					Type:        dsl.IssueLogic,
					Severity:    dsl.SeverityCritical,
					Chapter:     s.ChapterID,
					Description: "死亡后从练气二层降到一层但未出现寿元代价，和既有底线规则存在冲突",
					Suggestion:  "补充本次免寿元代价的触发条件，或修正为扣寿元版本",
				})
			}
		}
	}
	return issues
}

func checkCultivationLifespanCoupling(recaps []recapRecord) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	prevLevel := -1
	prevChapter := ""

	for _, r := range recaps {
		curLevel := inferChapterLevel(r)
		if prevLevel > 0 && curLevel > prevLevel {
			text := chapterText(r)
			hasRecovery := containsAny(text, "修炼", "吐纳", "打坐", "闭关", "突破", "恢复", "稳固")
			if !hasRecovery {
				issues = append(issues, dsl.SimulationIssue{
					Type:        dsl.IssueGrowth,
					Severity:    dsl.SeverityWarning,
					Chapter:     r.ChapterID,
					Description: fmt.Sprintf("修为从 %s 章的练气%d层提升到本章练气%d层，但缺少明确恢复/修炼过程", prevChapter, prevLevel, curLevel),
					Suggestion:  "补一段修炼恢复过程，或在 recap/status 中显式标注提升原因",
				})
			}
		}
		if curLevel > 0 {
			prevLevel = curLevel
			prevChapter = r.ChapterID
		}
	}
	return issues
}

func checkNPCIdentityConflict(recaps []recapRecord) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	descriptorOwners := map[string]map[string]bool{}
	deadChapter := map[string]string{}

	for _, r := range recaps {
		for _, p := range r.Present {
			name, desc := splitNameDesc(p)
			if name == "" {
				continue
			}
			if strings.Contains(desc, "已故") {
				deadChapter[name] = r.ChapterID
			}
			for _, k := range descriptorKeys(desc) {
				if descriptorOwners[k] == nil {
					descriptorOwners[k] = map[string]bool{}
				}
				descriptorOwners[k][name] = true
			}
			if deadAt, ok := deadChapter[name]; ok && deadAt != r.ChapterID && !containsAny(chapterText(r), "复活", "复生") {
				issues = append(issues, dsl.SimulationIssue{
					Type:        dsl.IssueCharacter,
					Severity:    dsl.SeverityWarning,
					Chapter:     r.ChapterID,
					Description: fmt.Sprintf("角色 '%s' 在 %s 已标记已故，但本章再次出现且无复活说明", name, deadAt),
					Suggestion:  "确认是否同名不同人；若是同一人，请补充存活/复活解释",
				})
			}
		}
	}

	for desc, owners := range descriptorOwners {
		if len(owners) <= 1 {
			continue
		}
		names := make([]string, 0, len(owners))
		for n := range owners {
			names = append(names, n)
		}
		sort.Strings(names)
		issues = append(issues, dsl.SimulationIssue{
			Type:        dsl.IssueCharacter,
			Severity:    dsl.SeverityWarning,
			Description: fmt.Sprintf("人物特征 '%s' 同时指向多个角色：%s，可能存在身份混淆", desc, strings.Join(names, " / ")),
			Suggestion:  "统一角色ID和特征归属，必要时增加一句区分（同名/绰号/误认）",
		})
	}
	return issues
}

func checkInjuryRecoveryContinuity(recaps []recapRecord) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	type inj struct{ chapter string }
	severe := map[string]inj{}

	for i, r := range recaps {
		text := chapterText(r)
		for _, s := range r.Status {
			name, desc := splitStatusLine(s)
			if name == "" {
				continue
			}
			if containsAny(desc, "重伤", "血肉模糊", "骨折", "无法行动", "奄奄一息", "濒死") {
				severe[name] = inj{chapter: r.ChapterID}
			}
		}

		if i == 0 {
			continue
		}
		for _, p := range r.Present {
			name, _ := splitNameDesc(p)
			if name == "" {
				continue
			}
			last, ok := severe[name]
			if !ok {
				continue
			}

			hasRecovery := containsAny(text, "疗伤", "包扎", "养伤", "恢复", "痊愈", "丹药")
			normalAct := containsAny(text, "干活", "挖矿", "行动自如", "奔跑", "出手")
			if normalAct && !hasRecovery {
				issues = append(issues, dsl.SimulationIssue{
					Type:        dsl.IssueContinuity,
					Severity:    dsl.SeverityWarning,
					Chapter:     r.ChapterID,
					Description: fmt.Sprintf("角色 '%s' 在 %s 重伤后，本章恢复正常行动但缺少疗伤过程", name, last.chapter),
					Suggestion:  "补充伤势恢复时间/手段（药物、休养、治疗者）",
				})
				delete(severe, name)
			}
		}
	}
	return issues
}

func checkResourceBudgetConsistency(recaps []recapRecord, rules *consistencyRules) []dsl.SimulationIssue {
	issues := make([]dsl.SimulationIssue, 0)
	halfStoneSeen := false
	useCount := 0
	lastChapter := ""

	for _, r := range recaps {
		text := chapterText(r)
		if containsAny(text, rules.Keywords.ResourceNames...) {
			halfStoneSeen = true
			lastChapter = r.ChapterID
		}
		if halfStoneSeen && containsAny(text, rules.Keywords.ResourceConsume...) && containsAny(text, rules.Keywords.ResourceNames...) {
			useCount++
		}
	}

	if halfStoneSeen && useCount >= rules.Thresholds.ResourceMinReuseCount {
		issues = append(issues, dsl.SimulationIssue{
			Type:        dsl.IssueBalance,
			Severity:    dsl.SeverityInfo,
			Chapter:     lastChapter,
			Description: "“半块下品灵石”多次承担修炼消耗，但持续性与消耗上限未交代",
			Suggestion:  "补充半块灵石可支持次数、衰减机制，或补写中途替代资源来源",
		})
	}
	return issues
}

func extractDeathCostSnapshot(r recapRecord) deathCostSnapshot {
	text := chapterText(r)
	s := deathCostSnapshot{ChapterID: r.ChapterID}
	s.HasDeath = containsAny(text, "死亡", "死后", "被砸死", "当场死", "身亡", "殒命")
	s.HasLifespan = regexp.MustCompile(`(扣|折损|损失|减少).{0,6}(寿元|寿命)|(寿元|寿命).{0,6}(扣|折损|损失|减少)`).MatchString(text)
	s.NoLifespan = regexp.MustCompile(`(未|无).{0,4}(折损|扣).{0,4}(寿元|寿命)|无寿元`).MatchString(text)

	re := regexp.MustCompile(`从练气([一二三四五六七八九十0-9]+)层.{0,12}(掉到|降到|跌到|回到|降至|至).{0,2}练气([一二三四五六七八九十0-9]+)层`)
	m := re.FindStringSubmatch(text)
	if len(m) == 4 {
		s.FromLevel = parseLevelToken(m[1])
		s.ToLevel = parseLevelToken(m[3])
		s.HasLevelDrop = s.FromLevel > s.ToLevel
	} else {
		s.HasLevelDrop = regexp.MustCompile(`修为.{0,6}(跌|降|掉|回落)`).MatchString(text)
	}
	return s
}

func inferChapterLevel(r recapRecord) int {
	text := strings.Join(r.Status, "\n") + "\n" + strings.Join(r.Reveals, "\n") + "\n" + strings.Join(r.PlotBeats, "\n")
	re := regexp.MustCompile(`练气([一二三四五六七八九十0-9]+)层`)
	all := re.FindAllStringSubmatch(text, -1)
	if len(all) == 0 {
		return -1
	}
	last := all[len(all)-1]
	return parseLevelToken(last[1])
}

func inferElapsedDays(timeText string) int {
	s := strings.TrimSpace(timeText)
	if s == "" {
		return 0
	}
	if strings.Contains(s, "半个月") {
		return 15
	}
	reDay := regexp.MustCompile(`([0-9]+)天`)
	if m := reDay.FindStringSubmatch(s); len(m) == 2 {
		d, _ := strconv.Atoi(m[1])
		return d
	}
	chDay := regexp.MustCompile(`([一二三四五六七八九十两]+)天`)
	if m := chDay.FindStringSubmatch(s); len(m) == 2 {
		return chineseNumberToInt(m[1])
	}
	if strings.Contains(s, "当日") || strings.Contains(s, "当天") {
		return 0
	}
	return 0
}

func splitNameDesc(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ""
	}
	for _, sep := range []string{"（", "(", "：", ":", "，", ","} {
		if idx := strings.Index(s, sep); idx > 0 {
			return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(sep):])
		}
	}
	return s, ""
}

func splitStatusLine(raw string) (string, string) {
	for _, sep := range []string{"：", ":"} {
		if idx := strings.Index(raw, sep); idx > 0 {
			return strings.TrimSpace(raw[:idx]), strings.TrimSpace(raw[idx+len(sep):])
		}
	}
	return splitNameDesc(raw)
}

func descriptorKeys(desc string) []string {
	keys := make([]string, 0)
	candidates := []string{"缺半块左耳", "左耳", "铜钱胎记", "左脸胎记", "胎记"}
	for _, c := range candidates {
		if strings.Contains(desc, c) {
			keys = append(keys, c)
		}
	}
	return keys
}

func chapterText(r recapRecord) string {
	var b strings.Builder
	b.WriteString(r.Title)
	b.WriteString("\n")
	b.WriteString(r.Time)
	b.WriteString("\n")
	for _, arr := range [][]string{r.Present, r.PlotBeats, r.Reveals, r.Unresolved, r.Status, r.Items} {
		for _, x := range arr {
			b.WriteString(x)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func containsAny(text string, tokens ...string) bool {
	for _, t := range tokens {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

func parseLevelToken(s string) int {
	if d, err := strconv.Atoi(s); err == nil {
		return d
	}
	return chineseNumberToInt(s)
}

func chineseNumberToInt(s string) int {
	val := map[rune]int{
		'零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
		'五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	if s == "" {
		return 0
	}
	if s == "十" {
		return 10
	}
	if strings.Contains(s, "十") {
		parts := strings.Split(s, "十")
		hi := 1
		if parts[0] != "" {
			r := []rune(parts[0])[0]
			hi = val[r]
		}
		lo := 0
		if len(parts) > 1 && parts[1] != "" {
			r := []rune(parts[1])[0]
			lo = val[r]
		}
		return hi*10 + lo
	}
	r := []rune(s)
	if len(r) == 1 {
		return val[r[0]]
	}
	return 0
}

func moduleEnabled(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}
