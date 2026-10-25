package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"novelgen/internal/platform/store"
)

const (
	defaultQueryLimit    = 6
	defaultQueryMaxChars = 4000
)

type graphNeighbor struct {
	Node       string
	Type       string
	Importance float64
}

type scoredScene struct {
	ID      string
	Score   float64
	Reasons []string
}

// Query selects the minimum prior-story context for one chapter using graph,
// lexical and optional vector retrieval. It builds an ephemeral equivalent
// when the persisted index is missing or stale, so an absent cache never blocks
// writing.
func Query(ctx context.Context, root *store.Root, opts QueryOptions) (*ContextResult, error) {
	startedAt := time.Now()
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	chapterID := strings.TrimSpace(opts.ChapterID)
	if chapterID == "" {
		return nil, fmt.Errorf("chapter id is required for narrative memory query")
	}
	intent := normalizeIntent(opts.Intent)
	if intent == "" {
		return nil, fmt.Errorf("unsupported narrative memory intent %q", opts.Intent)
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultQueryLimit
	}
	if opts.Limit > 32 {
		opts.Limit = 32
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = defaultQueryMaxChars
	}
	if opts.MaxChars < 400 {
		opts.MaxChars = 400
	}
	if opts.MaxChars > 20000 {
		opts.MaxChars = 20000
	}

	previous, previousVectors, warnings := loadPersisted(root)
	dirtyInvalidated := false
	if dirty, err := LoadNarrativeMemoryDirty(root); err != nil {
		warnings = append(warnings, "read narrative memory dirty marker: "+err.Error())
		dirtyInvalidated = true
	} else if dirty != nil {
		warnings = append(warnings, "narrative memory is marked stale by canonical writes; using an in-memory rebuild for this query")
		dirtyInvalidated = true
	}
	if previous != nil && previous.Vector.Enabled && previous.Vector.Count > len(previousVectors) {
		warnings = append(warnings, "persisted narrative memory vectors are incomplete; rebuilding an ephemeral index")
		previous = nil
		previousVectors = nil
	}
	buildResult, err := Build(ctx, root, BuildOptions{
		Embedder:         opts.Embedder,
		Previous:         previous,
		PreviousVectors:  previousVectors,
		SkipEmbedding:    true,
		SkipVectorErrors: true,
		Force:            dirtyInvalidated,
	})
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, buildResult.Warnings...)
	status := "ephemeral"
	if previous != nil && buildResult.UpToDate {
		status = "fresh"
	} else if previous != nil {
		warnings = append(warnings, "persisted narrative memory is stale; using an in-memory rebuild for this query")
	} else if !dirtyInvalidated {
		warnings = append(warnings, "narrative memory index is not built; using an in-memory index for this query")
	}
	index := buildResult.Index
	target, ok := chapterByID(index, chapterID)
	if !ok {
		return nil, fmt.Errorf("chapter %q not found in narrative memory outline", chapterID)
	}

	result := ContextResult{
		ChapterID:    chapterID,
		ChapterOrder: target.Order,
		Intent:       intent,
		MustInclude: MustIncludeFacts{
			ActiveCharacters: append([]string(nil), target.Characters...),
			Locations:        uniqueStrings(target.Location),
			Storylines:       append([]string(nil), target.Storylines...),
		},
		Promises:  []PromiseHit{},
		Retrieved: []RetrievedScene{},
		Citations: []Citation{},
		Meta: QueryMeta{
			Status:            status,
			IndexBuiltAt:      index.BuiltAt.Format(time.RFC3339),
			SourceFingerprint: fingerprintSummary(index.SourceFingerprints),
			MaxChars:          opts.MaxChars,
			SceneCount:        len(index.Scenes),
			NodeCount:         len(index.Nodes),
			EdgeCount:         len(index.Edges),
		},
		Warnings: warnings,
	}
	if previousChapter := previousChapterWithRecap(index, target.Order); previousChapter != nil {
		state := append([]string{}, previousChapter.RecapStatus...)
		state = append(state, previousChapter.RecapItems...)
		if previousChapter.RecapLastLine != "" {
			state = append(state, "last_line: "+previousChapter.RecapLastLine)
		}
		result.MustInclude.CurrentState = uniqueStrings(state...)
		if previousChapter.RecapNextHint != "" {
			result.MustInclude.PreviousRecap = previousChapter.RecapNextHint
		} else if previousChapter.RecapLastLine != "" {
			result.MustInclude.PreviousRecap = previousChapter.RecapLastLine
		}
	}

	result.Promises = selectPromises(index, target.Order, intent, 8)
	result.Meta.OpenPromises = countOpenPromises(index, target.Order)
	result.Meta.ElapsedMilliseconds = time.Since(startedAt).Milliseconds()
	if intent == "promise-audit" {
		result.Meta.ReturnedScenes = 0
		return &result, nil
	}

	allowedScenes := make(map[string]bool)
	for _, scene := range index.Scenes {
		if scene.ChapterOrder > target.Order {
			continue
		}
		if scene.ChapterOrder == target.Order && !opts.IncludeTarget {
			continue
		}
		allowedScenes[scene.ID] = true
	}
	sceneByID := make(map[string]SceneRecord, len(index.Scenes))
	for _, scene := range index.Scenes {
		sceneByID[scene.ID] = scene
	}
	queryText := buildQueryText(target, opts.Focus, result.Promises, intent)
	lexical := newLexicalIndex(index.Scenes)
	lexicalHits := lexical.Search(queryText, 0)
	lexicalByID := map[string]lexicalHit{}
	for _, hit := range lexicalHits {
		if allowedScenes[hit.ID] {
			lexicalByID[hit.ID] = hit
		}
	}
	result.Meta.LexicalCandidates = len(lexicalByID)

	graphScores, graphReasons := graphCandidates(index, target, result.MustInclude, result.Promises, allowedScenes)
	result.Meta.GraphCandidates = len(graphScores)

	vectorScores := map[string]vectorMatch{}
	if opts.Embedder != nil && len(buildResult.Vectors) > 0 && strings.TrimSpace(queryText) != "" {
		vectors, err := opts.Embedder.Embed(ctx, []string{queryText})
		if err != nil {
			result.Warnings = append(result.Warnings, "vector query embedding failed; continuing with graph and lexical retrieval: "+err.Error())
		} else if len(vectors) == 1 && len(vectors[0]) > 0 {
			for _, hit := range searchVectors(vectors[0], buildResult.Vectors, allowedScenes, 32) {
				vectorScores[hit.ID] = hit
			}
			result.Meta.VectorUsed = true
			result.Meta.VectorModel = opts.Embedder.Model()
		}
	}
	result.Meta.VectorCandidates = len(vectorScores)

	scores := map[string]*scoredScene{}
	get := func(id string) *scoredScene {
		item := scores[id]
		if item == nil {
			item = &scoredScene{ID: id}
			scores[id] = item
		}
		return item
	}
	lexicalRanked := rankedLexical(lexicalByID)
	for rank, hit := range lexicalRanked {
		item := get(hit.ID)
		item.Score += reciprocalRank(rank)
		item.Reasons = append(item.Reasons, "lexical")
	}
	graphIDs := make([]string, 0, len(graphScores))
	for id := range graphScores {
		graphIDs = append(graphIDs, id)
	}
	sort.SliceStable(graphIDs, func(i, j int) bool {
		if graphScores[graphIDs[i]] == graphScores[graphIDs[j]] {
			return graphIDs[i] < graphIDs[j]
		}
		return graphScores[graphIDs[i]] > graphScores[graphIDs[j]]
	})
	for rank, id := range graphIDs {
		item := get(id)
		item.Score += reciprocalRank(rank) * 0.9
		item.Score += graphScores[id] * 0.35
		reasons := graphReasons[id]
		if len(reasons) == 0 {
			reasons = []string{"graph"}
		}
		item.Reasons = append(item.Reasons, reasons...)
	}
	vectorRanked := rankedVectors(vectorScores)
	for rank, hit := range vectorRanked {
		item := get(hit.ID)
		item.Score += reciprocalRank(rank) * 1.1
		item.Reasons = append(item.Reasons, "vector")
	}
	for id, item := range scores {
		if !allowedScenes[id] {
			delete(scores, id)
			continue
		}
		scene := sceneByID[id]
		item.Score += scene.Importance * 0.02
		item.Score += recencyBoost(scene.ChapterOrder, target.Order)
		if opts.Focus != "" && containsNormalized(scene.Text, opts.Focus) {
			item.Score += 0.2
			item.Reasons = append(item.Reasons, "focus_exact")
		}
		item.Reasons = uniqueStrings(item.Reasons...)
	}
	if len(scores) == 0 {
		for rank, id := range fallbackSceneIDs(index, allowedScenes, opts.Limit) {
			scores[id] = &scoredScene{ID: id, Score: reciprocalRank(rank), Reasons: []string{"recent_fallback"}}
		}
	}

	ordered := make([]*scoredScene, 0, len(scores))
	for _, item := range scores {
		ordered = append(ordered, item)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Score == ordered[j].Score {
			left := sceneByID[ordered[i].ID]
			right := sceneByID[ordered[j].ID]
			if left.ChapterOrder == right.ChapterOrder {
				return left.ParagraphStart < right.ParagraphStart
			}
			return left.ChapterOrder > right.ChapterOrder
		}
		return ordered[i].Score > ordered[j].Score
	})
	remaining := opts.MaxChars
	seenParagraphs := map[string]bool{}
	for _, item := range ordered {
		if len(result.Retrieved) >= opts.Limit {
			break
		}
		scene := sceneByID[item.ID]
		paragraphKey := fmt.Sprintf("%s:%d-%d", scene.ChapterID, scene.ParagraphStart, scene.ParagraphEnd)
		if seenParagraphs[paragraphKey] {
			continue
		}
		text := scene.Text
		charCount := len([]rune(text))
		if charCount > remaining {
			if len(result.Retrieved) > 0 {
				continue
			}
			text = clipRunes(text, remaining)
			charCount = len([]rune(text))
		}
		if charCount == 0 {
			continue
		}
		remaining -= charCount
		seenParagraphs[paragraphKey] = true
		result.Retrieved = append(result.Retrieved, RetrievedScene{
			ID: scene.ID, ChapterID: scene.ChapterID, ChapterTitle: scene.ChapterTitle,
			ChapterOrder:   scene.ChapterOrder,
			ParagraphStart: scene.ParagraphStart, ParagraphEnd: scene.ParagraphEnd,
			Text: text, SourceSHA256: scene.TextSHA256, Score: roundScore(item.Score),
			Reasons: item.Reasons, Characters: scene.Characters, Locations: scene.Locations,
			Storylines: scene.Storylines,
		})
		result.Citations = append(result.Citations, Citation{
			ChapterID: scene.ChapterID, ParagraphStart: scene.ParagraphStart,
			ParagraphEnd: scene.ParagraphEnd, SHA256: scene.TextSHA256,
		})
	}
	result.Meta.ReturnedScenes = len(result.Retrieved)
	result.Meta.ReturnedChars = opts.MaxChars - remaining
	result.Meta.ElapsedMilliseconds = time.Since(startedAt).Milliseconds()
	return &result, nil
}

func chapterByID(index *Index, chapterID string) (ChapterRecord, bool) {
	for _, chapter := range index.Chapters {
		if chapter.ID == chapterID {
			return chapter, true
		}
	}
	return ChapterRecord{}, false
}

func previousChapterWithRecap(index *Index, targetOrder int) *ChapterRecord {
	var selected *ChapterRecord
	for i := range index.Chapters {
		chapter := &index.Chapters[i]
		if chapter.Order >= targetOrder || chapter.RecapSHA256 == "" || chapter.RecapSHA256 == "missing" {
			continue
		}
		if selected == nil || chapter.Order > selected.Order {
			selected = chapter
		}
	}
	return selected
}

func selectPromises(index *Index, targetOrder int, intent string, limit int) []PromiseHit {
	hits := make([]PromiseHit, 0)
	for _, promise := range index.Promises {
		if promise.ChapterOrder >= targetOrder {
			continue
		}
		if promise.Status == "resolved" && promise.ResolvedChapter > 0 && promise.ResolvedChapter <= targetOrder {
			continue
		}
		reason := "open"
		if promise.DueChapterOrder > 0 {
			switch {
			case promise.DueChapterOrder < targetOrder:
				reason = "overdue"
			case promise.DueChapterOrder == targetOrder:
				reason = "due_this_chapter"
			default:
				reason = "upcoming_due"
			}
		}
		if intent == "chapter-write" && promise.DueChapterOrder > targetOrder+1 && promise.Kind != "mystery" {
			continue
		}
		status := promise.Status
		if status == "resolved" {
			status = "open"
		}
		hits = append(hits, PromiseHit{
			ID: promise.ID, Kind: promise.Kind, Text: promise.Text,
			ChapterID: promise.ChapterID, DueChapterID: promise.DueChapterID,
			DueChapterOrder: promise.DueChapterOrder, Horizon: promise.Horizon,
			Status: status, Reason: reason,
		})
	}
	priority := map[string]int{"overdue": 0, "due_this_chapter": 1, "open": 2, "upcoming_due": 3}
	sort.SliceStable(hits, func(i, j int) bool {
		leftPriority := priority[hits[i].Reason]
		rightPriority := priority[hits[j].Reason]
		if leftPriority == rightPriority {
			return hits[i].ChapterID < hits[j].ChapterID
		}
		return leftPriority < rightPriority
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func countOpenPromises(index *Index, targetOrder int) int {
	count := 0
	for _, promise := range index.Promises {
		if promise.ChapterOrder >= targetOrder {
			continue
		}
		if promise.Status == "resolved" && promise.ResolvedChapter > 0 && promise.ResolvedChapter <= targetOrder {
			continue
		}
		if promise.Status != "resolved" || promise.ResolvedChapter > targetOrder {
			count++
		}
	}
	return count
}

func buildQueryText(target ChapterRecord, focus string, promises []PromiseHit, intent string) string {
	parts := []string{
		focus, target.Title, target.Summary, target.Conflict,
		strings.Join(target.Characters, " "),
		target.Location,
		strings.Join(target.Storylines, " "),
	}
	if intent == "voice-echo" {
		parts = append(parts, "character voice dialogue interiority rhythm")
	}
	for _, promise := range promises {
		parts = append(parts, promise.Text)
	}
	return strings.Join(uniqueStrings(parts...), " ")
}

func graphCandidates(index *Index, target ChapterRecord, must MustIncludeFacts, promises []PromiseHit, allowed map[string]bool) (map[string]float64, map[string][]string) {
	adjacency := graphAdjacency(index, target.Order)
	seeds := make([]string, 0)
	for _, name := range must.ActiveCharacters {
		seeds = append(seeds, entityNodeID("character", name))
	}
	for _, name := range must.Locations {
		seeds = append(seeds, entityNodeID("location", name))
	}
	for _, name := range must.Storylines {
		seeds = append(seeds, entityNodeID("storyline", name))
	}
	for _, promise := range promises {
		seeds = append(seeds, promise.ID)
	}
	nodeNames := map[string]string{}
	for _, node := range index.Nodes {
		nodeNames[node.ID] = node.Name
	}
	current := map[string]float64{}
	reasons := map[string][]int{}
	for _, seed := range uniqueStrings(seeds...) {
		if _, ok := nodeNames[seed]; ok {
			current[seed] = 1
		}
	}
	for depth := 0; depth < 2; depth++ {
		next := map[string]float64{}
		decay := 0.55 - float64(depth)*0.1
		ids := make([]string, 0, len(current))
		for id := range current {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, nodeID := range ids {
			for _, neighbor := range adjacency[nodeID] {
				score := current[nodeID] * decay
				if neighbor.Importance > 0 {
					score *= 1 + neighbor.Importance*0.05
				}
				if score > next[neighbor.Node] {
					next[neighbor.Node] = score
					reasons[neighbor.Node] = append(reasons[neighbor.Node], depth+1)
				}
			}
		}
		for id, score := range next {
			if score > current[id] {
				current[id] = score
			}
		}
	}
	out := map[string]float64{}
	reasonText := map[string][]string{}
	for id, score := range current {
		if !allowed[id] {
			continue
		}
		out[id] = score
		depths := reasons[id]
		if len(depths) == 0 {
			reasonText[id] = []string{"graph_seed"}
		} else {
			reasonText[id] = []string{fmt.Sprintf("graph_depth_%d", int(depths[0]))}
		}
	}
	return out, reasonText
}

func graphAdjacency(index *Index, targetOrder int) map[string][]graphNeighbor {
	adjacency := map[string][]graphNeighbor{}
	for _, edge := range index.Edges {
		if edge.ValidFromChapter > targetOrder {
			continue
		}
		if edge.ValidToChapter > 0 && targetOrder > edge.ValidToChapter {
			continue
		}
		importance := edge.Importance
		if importance <= 0 {
			importance = 1
		}
		adjacency[edge.From] = append(adjacency[edge.From], graphNeighbor{Node: edge.To, Type: edge.Type, Importance: importance})
		adjacency[edge.To] = append(adjacency[edge.To], graphNeighbor{Node: edge.From, Type: "reverse:" + edge.Type, Importance: importance})
	}
	return adjacency
}

func rankedLexical(values map[string]lexicalHit) []lexicalHit {
	hits := make([]lexicalHit, 0, len(values))
	for _, hit := range values {
		hits = append(hits, hit)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	return hits
}

func rankedVectors(values map[string]vectorMatch) []vectorMatch {
	hits := make([]vectorMatch, 0, len(values))
	for _, hit := range values {
		hits = append(hits, hit)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	return hits
}

func reciprocalRank(rank int) float64 {
	return 1 / (60 + float64(rank))
}

func recencyBoost(sceneOrder, targetOrder int) float64 {
	distance := targetOrder - sceneOrder
	if distance < 0 {
		return 0
	}
	boost := 0.08 - float64(distance)*0.004
	if boost < 0 {
		return 0
	}
	return boost
}

func fallbackSceneIDs(index *Index, allowed map[string]bool, limit int) []string {
	type candidate struct {
		id         string
		order      int
		importance float64
	}
	values := make([]candidate, 0)
	for _, scene := range index.Scenes {
		if allowed[scene.ID] {
			values = append(values, candidate{id: scene.ID, order: scene.ChapterOrder, importance: scene.Importance})
		}
	}
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].order == values[j].order {
			return values[i].importance > values[j].importance
		}
		return values[i].order > values[j].order
	})
	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.id)
	}
	return out
}

func fingerprintSummary(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(values[key])
		builder.WriteByte('\n')
	}
	return hashString(builder.String())
}

func normalizeIntent(value string) string {
	switch normalizeEntityName(value) {
	case "", "chapter-write", "chapter_write", "write":
		return "chapter-write"
	case "promise-audit", "promise_audit", "promises":
		return "promise-audit"
	case "voice-echo", "voice_echo", "voice":
		return "voice-echo"
	case "continuity":
		return "continuity"
	case "all":
		return "all"
	default:
		return ""
	}
}

func clipRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit == 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func roundScore(value float64) float64 {
	return float64(int(value*10000+0.5)) / 10000
}
