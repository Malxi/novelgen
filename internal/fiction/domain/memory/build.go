package memory

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

const (
	defaultSceneWindow = 4
	defaultSceneStride = 3
)

type flatMemoryChapter struct {
	Chapter *models.Chapter
	Volume  *models.Volume
	Order   int
}

type canonicalSources struct {
	Setup         *models.StorySetup
	Outline       *models.Outline
	Characters    map[string]*models.Character
	Locations     map[string]*models.Location
	Items         map[string]*models.Item
	Organizations map[string]*models.Organization
	Chapters      []flatMemoryChapter
	ChapterByID   map[string]flatMemoryChapter
	Content       map[string]string
	Recaps        map[string]*models.ChapterRecap
	Fingerprints  map[string]string
}

type entityCatalog struct {
	Names map[string]map[string]bool
	Text  map[string]map[string]string
}

// Build constructs the index from canonical project artifacts. It returns the
// previous index unchanged when all source fingerprints and vector settings
// still match.
func Build(ctx context.Context, root *store.Root, opts BuildOptions) (*BuildResult, error) {
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	sources, err := loadCanonicalSources(root, embeddingModel(opts.Embedder))
	if err != nil {
		return nil, err
	}
	if opts.Previous != nil && !opts.Force {
		changed := changedFingerprints(opts.Previous.SourceFingerprints, sources.Fingerprints)
		vectorComplete := embeddingModel(opts.Embedder) == "" || opts.Previous.Vector.Count >= len(opts.Previous.Scenes)
		if opts.Previous.Version == IndexVersion && len(changed) == 0 && vectorComplete {
			return &BuildResult{
				Index:          opts.Previous,
				Vectors:        opts.PreviousVectors,
				Stats:          opts.Previous.Stats,
				UpToDate:       true,
				ChangedSources: nil,
			}, nil
		}
	}

	window := opts.SceneWindow
	if window <= 0 {
		window = defaultSceneWindow
	}
	stride := opts.SceneStride
	if stride <= 0 {
		stride = defaultSceneStride
	}
	if stride > window {
		stride = window
	}

	builder := indexBuilder{
		sources:         sources,
		index:           newIndex(sources.Fingerprints),
		entityCatalog:   buildEntityCatalog(sources),
		promiseByID:     map[string]int{},
		nodeByID:        map[string]int{},
		edgeSeen:        map[string]bool{},
		previousVectors: opts.PreviousVectors,
		embedder:        opts.Embedder,
		vectorWarnings:  nil,
	}
	builder.buildGraphBase()
	builder.buildChapterScenes(window, stride)
	builder.finishPromises()
	if err := builder.buildVectors(ctx, opts.Previous, opts.SkipVectorErrors, opts.SkipEmbedding); err != nil {
		return nil, err
	}
	builder.finishStats()

	return &BuildResult{
		Index:          builder.index,
		Vectors:        builder.vectors,
		Stats:          builder.index.Stats,
		ChangedSources: changedFingerprints(previousFingerprints(opts.Previous), sources.Fingerprints),
		Warnings:       builder.vectorWarnings,
	}, nil
}

// Refresh builds and atomically persists the derived index and optional vector
// sidecar. It never writes canonical story artifacts.
func Refresh(ctx context.Context, root *store.Root, embedder Embedder) (*RefreshResult, error) {
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	previous, previousVectors, loadWarnings := loadPersisted(root)
	if previous != nil && previous.Vector.Enabled && previous.Vector.Count > len(previousVectors) {
		loadWarnings = append(loadWarnings, "persisted narrative memory vectors are incomplete; rebuilding them")
		previous = nil
		previousVectors = nil
	}
	result, err := Build(ctx, root, BuildOptions{
		Embedder:         embedder,
		Previous:         previous,
		PreviousVectors:  previousVectors,
		SkipVectorErrors: true,
	})
	if err != nil {
		return nil, err
	}
	result.Warnings = append(loadWarnings, result.Warnings...)
	if result.UpToDate {
		if err := clearNarrativeMemoryDirty(root); err != nil {
			return nil, fmt.Errorf("clear narrative memory dirty marker: %w", err)
		}
		return &RefreshResult{
			OK:          true,
			Status:      "up_to_date",
			IndexPath:   root.NarrativeMemoryIndexFile(),
			VectorsPath: vectorPathIfPresent(root, result.Index.Vector),
			BuiltAt:     result.Index.BuiltAt,
			Stats:       result.Index.Stats,
			Vector:      result.Index.Vector,
			Warnings:    result.Warnings,
		}, nil
	}

	vectorData, err := encodeVectors(result.Vectors)
	if err != nil {
		return nil, fmt.Errorf("encode narrative memory vectors: %w", err)
	}
	if len(result.Vectors) > 0 {
		result.Index.Vector.FileSHA256 = vectorFileSHA256(vectorData)
	}
	if err := root.WriteJSON(root.NarrativeMemoryIndexFile(), result.Index); err != nil {
		return nil, fmt.Errorf("save narrative memory index: %w", err)
	}
	if len(result.Vectors) > 0 {
		if err := root.Write(root.NarrativeMemoryVectorsFile(), vectorData, store.FileMode); err != nil {
			return nil, fmt.Errorf("save narrative memory vectors: %w", err)
		}
	}
	if err := clearNarrativeMemoryDirty(root); err != nil {
		return nil, fmt.Errorf("clear narrative memory dirty marker: %w", err)
	}
	return &RefreshResult{
		OK:             true,
		Status:         "rebuilt",
		IndexPath:      root.NarrativeMemoryIndexFile(),
		VectorsPath:    vectorPathIfPresent(root, result.Index.Vector),
		BuiltAt:        result.Index.BuiltAt,
		ChangedSources: result.ChangedSources,
		Stats:          result.Stats,
		Vector:         result.Index.Vector,
		Warnings:       result.Warnings,
	}, nil
}

func previousFingerprints(index *Index) map[string]string {
	if index == nil {
		return nil
	}
	return index.SourceFingerprints
}

// Inspect reports whether a persisted index exists and still matches the
// current canonical source files.
func Inspect(root *store.Root, embeddingModelName string) InspectResult {
	result := InspectResult{IndexPath: root.NarrativeMemoryIndexFile()}
	index, vectors, warnings := loadPersisted(root)
	if index == nil {
		result.Status = "missing"
		return result
	}
	dirty, err := LoadNarrativeMemoryDirty(root)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	if dirty != nil {
		result.Status = "stale"
		result.BuiltAt = &index.BuiltAt
		result.Stats = index.Stats
		result.Vector = index.Vector
		result.VectorsPath = vectorPathIfPresent(root, index.Vector)
		for _, change := range dirty.Changes {
			result.ChangedSources = append(result.ChangedSources, "dirty:"+change)
		}
		return result
	}
	result.BuiltAt = &index.BuiltAt
	result.Stats = index.Stats
	result.Vector = index.Vector
	result.VectorsPath = vectorPathIfPresent(root, index.Vector)
	sources, err := loadCanonicalSources(root, embeddingModelName)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	changed := changedFingerprints(index.SourceFingerprints, sources.Fingerprints)
	if index.Version != IndexVersion {
		changed = append(changed, "index_version")
	}
	if index.Vector.Enabled && index.Vector.Count > len(vectors) {
		changed = append(changed, "vectors")
	}
	changed = uniqueStrings(changed...)
	if len(changed) > 0 {
		result.Status = "stale"
		result.ChangedSources = changed
		return result
	}
	result.Status = "fresh"
	if len(warnings) > 0 {
		result.Status = "error"
		result.Error = strings.Join(warnings, "; ")
	}
	return result
}

func loadPersisted(root *store.Root) (*Index, map[string][]float32, []string) {
	var warnings []string
	index := &Index{}
	if err := root.ReadJSON(root.NarrativeMemoryIndexFile(), index); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			warnings = append(warnings, "read narrative memory index: "+err.Error())
		}
		return nil, nil, warnings
	}
	if index.Version != IndexVersion {
		warnings = append(warnings, fmt.Sprintf("narrative memory index version %d, want %d", index.Version, IndexVersion))
	}
	vectors := map[string][]float32{}
	data, err := root.Read(root.NarrativeMemoryVectorsFile())
	if err == nil {
		if expected := strings.TrimSpace(index.Vector.FileSHA256); expected != "" && expected != vectorFileSHA256(data) {
			warnings = append(warnings, "narrative memory vector file hash does not match index")
		} else {
			decoded, decodeErr := decodeVectors(data)
			if decodeErr != nil {
				warnings = append(warnings, "read narrative memory vectors: "+decodeErr.Error())
			} else {
				vectors = decoded
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, "read narrative memory vectors: "+err.Error())
	}
	return index, vectors, warnings
}

func vectorPathIfPresent(root *store.Root, info VectorInfo) string {
	if !info.Enabled || info.Count == 0 {
		return ""
	}
	return root.NarrativeMemoryVectorsFile()
}

func newIndex(fingerprints map[string]string) *Index {
	return &Index{
		Version:            IndexVersion,
		BuiltAt:            time.Now().UTC(),
		SourceFingerprints: fingerprints,
		Chapters:           []ChapterRecord{},
		Scenes:             []SceneRecord{},
		Nodes:              []NodeRecord{},
		Edges:              []EdgeRecord{},
		Promises:           []PromiseRecord{},
	}
}

func loadCanonicalSources(root *store.Root, embeddingModelName string) (*canonicalSources, error) {
	setup, err := root.LoadSetup()
	if err != nil {
		return nil, fmt.Errorf("load setup for narrative memory: %w", err)
	}
	outline, err := root.LoadOutline()
	if err != nil {
		return nil, fmt.Errorf("load outline for narrative memory: %w", err)
	}
	characters, locations, items, organizations, err := root.LoadElements()
	if err != nil {
		return nil, fmt.Errorf("load craft for narrative memory: %w", err)
	}
	sources := &canonicalSources{
		Setup:         setup,
		Outline:       outline,
		Characters:    characters,
		Locations:     locations,
		Items:         items,
		Organizations: organizations,
		ChapterByID:   map[string]flatMemoryChapter{},
		Content:       map[string]string{},
		Recaps:        map[string]*models.ChapterRecap{},
		Fingerprints:  map[string]string{},
	}
	sources.Fingerprints["index_version"] = strconv.Itoa(IndexVersion)
	sources.Fingerprints["embedding_model"] = strings.TrimSpace(embeddingModelName)
	if err := addRootFileFingerprint(root, sources.Fingerprints, "setup", root.SetupJSON()); err != nil {
		return nil, err
	}
	if err := addRootFileFingerprint(root, sources.Fingerprints, "outline", root.OutlineJSON()); err != nil {
		return nil, err
	}
	for _, kind := range []store.CraftKind{
		store.CraftCharacters, store.CraftLocations, store.CraftItems, store.CraftOrganizations,
	} {
		if err := addRootFileFingerprint(root, sources.Fingerprints, "craft:"+string(kind), root.CraftFile(kind)); err != nil {
			return nil, err
		}
	}

	order := 0
	for partIndex := range outline.Parts {
		part := &outline.Parts[partIndex]
		for volumeIndex := range part.Volumes {
			volume := &part.Volumes[volumeIndex]
			for chapterIndex := range volume.Chapters {
				order++
				chapter := &volume.Chapters[chapterIndex]
				flat := flatMemoryChapter{Chapter: chapter, Volume: volume, Order: order}
				sources.Chapters = append(sources.Chapters, flat)
				sources.ChapterByID[chapter.ID] = flat

				content, path, loadErr := root.LoadChapterContent(chapter.ID)
				if loadErr != nil {
					return nil, fmt.Errorf("load chapter %s for narrative memory: %w", chapter.ID, loadErr)
				}
				sources.Content[chapter.ID] = content
				if path != "" {
					sources.Fingerprints["chapter:"+chapter.ID] = hashString(content)
				} else {
					sources.Fingerprints["chapter:"+chapter.ID] = "missing"
				}

				recap, recapErr := root.LoadRecap(chapter.ID)
				switch {
				case recapErr == nil:
					sources.Recaps[chapter.ID] = recap
					if data, readErr := root.Read(root.RecapFile(chapter.ID)); readErr == nil {
						sources.Fingerprints["recap:"+chapter.ID] = hashBytes(data)
					} else {
						return nil, fmt.Errorf("read recap %s for narrative memory: %w", chapter.ID, readErr)
					}
				case errors.Is(recapErr, fs.ErrNotExist):
					sources.Fingerprints["recap:"+chapter.ID] = "missing"
				default:
					return nil, fmt.Errorf("load recap %s for narrative memory: %w", chapter.ID, recapErr)
				}
			}
		}
	}
	return sources, nil
}

func addRootFileFingerprint(root *store.Root, out map[string]string, key, path string) error {
	data, err := root.Read(path)
	if err == nil {
		out[key] = hashBytes(data)
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		out[key] = "missing"
		return nil
	}
	return fmt.Errorf("read %s for narrative memory: %w", key, err)
}

func changedFingerprints(previous, current map[string]string) []string {
	keys := map[string]bool{}
	for key := range previous {
		keys[key] = true
	}
	for key := range current {
		keys[key] = true
	}
	changed := make([]string, 0)
	for key := range keys {
		if previous[key] != current[key] {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

func hashString(value string) string {
	return hashBytes([]byte(value))
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return fmt.Sprintf("%x", sum[:])
}

func embeddingModel(embedder Embedder) string {
	if embedder == nil {
		return ""
	}
	return strings.TrimSpace(embedder.Model())
}

func buildEntityCatalog(sources *canonicalSources) entityCatalog {
	catalog := entityCatalog{
		Names: map[string]map[string]bool{
			"character": {}, "item": {}, "location": {}, "organization": {}, "storyline": {},
		},
		Text: map[string]map[string]string{
			"character": {}, "item": {}, "location": {}, "organization": {}, "storyline": {},
		},
	}
	add := func(kind, name string, aliases ...string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := normalizeEntityName(name)
		catalog.Names[kind][key] = true
		catalog.Text[kind][key] = name
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			catalog.Names[kind][normalizeEntityName(alias)] = true
			catalog.Text[kind][normalizeEntityName(alias)] = name
		}
	}
	for key, character := range sources.Characters {
		add("character", character.Name, append([]string{key}, character.Aliases...)...)
	}
	for key, location := range sources.Locations {
		add("location", location.Name, key)
	}
	for key, item := range sources.Items {
		add("item", item.Name, key)
	}
	for key, organization := range sources.Organizations {
		add("organization", organization.Name, key)
	}
	for _, storyline := range sources.Setup.Storylines {
		add("storyline", storyline.Name)
	}
	for _, seed := range sources.Setup.CoreCast {
		add("character", seed.Name)
	}
	for _, chapter := range sources.Chapters {
		for _, name := range chapter.Chapter.Characters {
			add("character", name)
		}
		add("location", chapter.Chapter.Location)
		for _, scene := range chapter.Chapter.Scenes {
			add("character", scene.POV)
			for _, name := range scene.Characters {
				add("character", name)
			}
			add("location", scene.Location)
		}
		for _, event := range chapter.Chapter.Events {
			add("character", event.GetActor())
			switch event.GetTargetType() {
			case models.TargetTypeCharacter:
				add("character", event.GetTarget())
			case models.TargetTypeItem:
				add("item", event.GetTarget())
			case models.TargetTypeLocation:
				add("location", event.GetTarget())
			case models.TargetTypeStoryline:
				add("storyline", event.GetTarget())
			}
		}
	}
	return catalog
}

type indexBuilder struct {
	sources         *canonicalSources
	index           *Index
	entityCatalog   entityCatalog
	promiseByID     map[string]int
	nodeByID        map[string]int
	edgeSeen        map[string]bool
	vectors         map[string][]float32
	previousVectors map[string][]float32
	embedder        Embedder
	vectorWarnings  []string
}

func (b *indexBuilder) buildGraphBase() {
	ownershipSpans, itemsWithTimeline := itemOwnershipSpans(b.sources.Chapters)
	for _, chapter := range b.sources.Chapters {
		chapterNodeID := chapterNodeID(chapter.Chapter.ID)
		b.addNode(NodeRecord{
			ID:           chapterNodeID,
			Type:         "chapter",
			Name:         chapter.Chapter.Title,
			ChapterID:    chapter.Chapter.ID,
			ChapterOrder: chapter.Order,
			Scope:        "planned",
			Importance:   2,
			Text:         chapter.Chapter.Summary,
		})
		recap := b.sources.Recaps[chapter.Chapter.ID]
		recapStatus := []string(nil)
		recapItems := []string(nil)
		recapLastLine := ""
		recapNextHint := ""
		if recap != nil {
			recapStatus = uniqueStrings(append(recap.Status, recap.Unresolved...)...)
			recapItems = uniqueStrings(recap.Items...)
			recapLastLine = strings.TrimSpace(recap.LastLine)
			recapNextHint = strings.TrimSpace(recap.NextOpeningHint)
		}
		b.index.Chapters = append(b.index.Chapters, ChapterRecord{
			ID:            chapter.Chapter.ID,
			Title:         chapter.Chapter.Title,
			VolumeID:      chapter.Volume.ID,
			Order:         chapter.Order,
			Summary:       chapter.Chapter.Summary,
			Conflict:      chapter.Chapter.Conflict,
			Characters:    uniqueStrings(chapter.Chapter.Characters...),
			Location:      chapter.Chapter.Location,
			Timeline:      chapter.Chapter.Timeline.Anchor,
			POV:           chapterPOV(chapter.Chapter),
			Storylines:    uniqueStrings(chapterStorylines(chapter.Chapter)...),
			HasManuscript: strings.TrimSpace(b.sources.Content[chapter.Chapter.ID]) != "",
			ContentSHA256: hashString(b.sources.Content[chapter.Chapter.ID]),
			RecapSHA256:   b.sources.Fingerprints["recap:"+chapter.Chapter.ID],
			RecapStatus:   recapStatus,
			RecapItems:    recapItems,
			RecapLastLine: recapLastLine,
			RecapNextHint: recapNextHint,
		})
		for _, name := range chapter.Chapter.Characters {
			b.addCharacterNode(name, "", "", 1)
			b.addEntityEdge(chapterNodeID, "mentions_character", entityNodeID("character", name), "planned", chapter, 0, 0, chapter.Chapter.Summary, 1)
		}
		if location := strings.TrimSpace(chapter.Chapter.Location); location != "" {
			b.addLocationNode(location)
			b.addEntityEdge(chapterNodeID, "located_at", entityNodeID("location", location), "planned", chapter, 0, 0, chapter.Chapter.Summary, 1)
		}
		for _, advance := range chapter.Chapter.StorylineAdvances {
			name := strings.TrimSpace(advance.StorylineName)
			if name == "" {
				continue
			}
			b.addStorylineNode(name, 1)
			b.addEntityEdge(chapterNodeID, "advances_storyline", entityNodeID("storyline", name), "planned", chapter, 0, 0, firstNonEmpty(advance.Change, advance.Consequence, advance.Pressure), 2)
		}
		for eventIndex, event := range chapter.Chapter.Events {
			eventID := "event:" + chapter.Chapter.ID + ":" + strconv.Itoa(eventIndex)
			text := eventSearchText(event)
			b.addNode(NodeRecord{
				ID: eventID, Type: "event", Name: event.GetAction(), ChapterID: chapter.Chapter.ID,
				ChapterOrder: chapter.Order, Scope: "planned", Importance: 1, Text: text,
			})
			b.addEntityEdge(chapterNodeID, "has_event", eventID, "planned", chapter, 0, 0, text, 1)
			if actor := strings.TrimSpace(event.GetActor()); actor != "" {
				b.addCharacterNode(actor, "", "", 1)
				b.addEntityEdge(eventID, "has_actor", entityNodeID("character", actor), "planned", chapter, 0, 0, text, 1)
			}
			target := strings.TrimSpace(event.GetTarget())
			switch event.GetTargetType() {
			case models.TargetTypeCharacter:
				b.addCharacterNode(target, "", "", 1)
				b.addEntityEdge(eventID, "targets_character", entityNodeID("character", target), "planned", chapter, 0, 0, text, 1)
			case models.TargetTypeItem:
				b.addItemNode(target, "", 1)
				b.addEntityEdge(eventID, "targets_item", entityNodeID("item", target), "planned", chapter, 0, 0, text, 1)
			case models.TargetTypeLocation:
				b.addLocationNode(target)
				b.addEntityEdge(eventID, "targets_location", entityNodeID("location", target), "planned", chapter, 0, 0, text, 1)
			case models.TargetTypeStoryline:
				b.addStorylineNode(target, 1)
				b.addEntityEdge(eventID, "targets_storyline", entityNodeID("storyline", target), "planned", chapter, 0, 0, text, 1)
			}
		}
		b.addMysteryNodes(chapter)
		b.addRecapEdges(chapter)
	}
	b.addOwnershipEdges(ownershipSpans)
	b.addCraftEdges(itemsWithTimeline)
}

func (b *indexBuilder) buildChapterScenes(window, stride int) {
	chapterOrder := map[string]int{}
	for _, chapter := range b.sources.Chapters {
		chapterOrder[chapter.Chapter.ID] = chapter.Order
	}
	for _, chapter := range b.sources.Chapters {
		content := b.sources.Content[chapter.Chapter.ID]
		if strings.TrimSpace(content) == "" {
			continue
		}
		paragraphs := project.SplitChapterParagraphs(content)
		prose := make([]project.ChapterParagraph, 0, len(paragraphs))
		for _, paragraph := range paragraphs {
			if paragraph.Kind != "heading" {
				prose = append(prose, paragraph)
			}
		}
		if len(prose) == 0 {
			continue
		}
		windows := paragraphWindows(prose, window, stride)
		for windowIndex, block := range windows {
			text := joinParagraphText(block)
			if strings.TrimSpace(text) == "" {
				continue
			}
			start := block[0].Index
			end := block[len(block)-1].Index
			sceneID := "scene:" + chapter.Chapter.ID + ":" + strconv.Itoa(start) + "-" + strconv.Itoa(end)
			characters, locations, items, organizations, storylines := b.mentionedEntities(text)
			pov, outlineLocation, outlineCharacters := outlineSceneMetadata(chapter.Chapter, windowIndex, len(windows))
			if pov != "" {
				characters = appendUnique(pov, characters)
			}
			for _, name := range outlineCharacters {
				if containsNormalized(text, name) {
					characters = appendUnique(name, characters)
				}
			}
			if outlineLocation != "" && containsNormalized(text, outlineLocation) {
				locations = appendUnique(outlineLocation, locations)
			}
			storylines = uniqueStrings(append(storylines, chapterStorylines(chapter.Chapter)...)...)
			recapSignals := recapSignalTexts(b.sources.Recaps[chapter.Chapter.ID])
			importance := 1.0 + float64(len(storylines))*0.25 + float64(len(chapter.Chapter.Mysteries.Planted))*0.3
			scene := SceneRecord{
				ID:               sceneID,
				ChapterID:        chapter.Chapter.ID,
				ChapterTitle:     chapter.Chapter.Title,
				ChapterOrder:     chapter.Order,
				ParagraphStart:   start,
				ParagraphEnd:     end,
				Text:             text,
				TextSHA256:       hashString(text),
				POV:              pov,
				Characters:       displayNames(characters),
				Locations:        displayNames(locations),
				Items:            displayNames(items),
				Organizations:    displayNames(organizations),
				Storylines:       uniqueStrings(storylines...),
				PlantedMysteries: plantedMysteryIDs(chapter.Chapter),
				RecapSignals:     recapSignals,
				Importance:       importance,
			}
			b.index.Scenes = append(b.index.Scenes, scene)
			b.addNode(NodeRecord{
				ID: sceneID, Type: "scene", Name: text, ChapterID: chapter.Chapter.ID,
				ChapterOrder: chapter.Order, Scope: "actual", Importance: importance, Text: text,
			})
			b.addEntityEdge(chapterNodeID(chapter.Chapter.ID), "contains_scene", sceneID, "actual", chapter, start, end, "", importance)
			for _, name := range scene.Characters {
				b.addEntityEdge(sceneID, "mentions_character", entityNodeID("character", name), "actual", chapter, start, end, text, 1)
			}
			for _, name := range scene.Locations {
				b.addEntityEdge(sceneID, "mentions_location", entityNodeID("location", name), "actual", chapter, start, end, text, 1)
			}
			for _, name := range scene.Items {
				b.addEntityEdge(sceneID, "mentions_item", entityNodeID("item", name), "actual", chapter, start, end, text, 1)
			}
			for _, name := range scene.Organizations {
				b.addEntityEdge(sceneID, "mentions_organization", entityNodeID("organization", name), "actual", chapter, start, end, text, 1)
			}
			for _, name := range scene.Storylines {
				b.addEntityEdge(sceneID, "related_storyline", entityNodeID("storyline", name), "actual", chapter, start, end, text, 1)
			}
		}
	}
}

func (b *indexBuilder) buildVectors(ctx context.Context, previous *Index, skipErrors, skipEmbedding bool) error {
	b.vectors = map[string][]float32{}
	if b.embedder == nil || strings.TrimSpace(b.embedder.Model()) == "" {
		return nil
	}
	b.index.Vector = VectorInfo{Enabled: true, Model: b.embedder.Model()}
	reuseAllowed := previous != nil && previous.Vector.Model == b.embedder.Model()
	missing := make([]SceneRecord, 0)
	for i := range b.index.Scenes {
		scene := &b.index.Scenes[i]
		if vector := b.previousVectors[scene.ID]; reuseAllowed && len(vector) > 0 {
			b.vectors[scene.ID] = vector
			scene.VectorPosition = i
			b.index.Vector.Reused++
			continue
		}
		missing = append(missing, *scene)
	}
	if skipEmbedding {
		if len(missing) > 0 {
			b.vectorWarnings = append(b.vectorWarnings, fmt.Sprintf("ephemeral memory build reused %d vectors; %d new or changed scenes have no vector", len(b.vectors), len(missing)))
		}
		b.index.Vector.Count = len(b.vectors)
		if len(b.index.Scenes) > 0 {
			b.index.Vector.Coverage = float64(len(b.vectors)) / float64(len(b.index.Scenes))
		}
		return nil
	}
	const batchSize = 32
	for start := 0; start < len(missing); start += batchSize {
		end := start + batchSize
		if end > len(missing) {
			end = len(missing)
		}
		texts := make([]string, 0, end-start)
		for _, scene := range missing[start:end] {
			texts = append(texts, vectorText(scene))
		}
		vectors, err := b.embedder.Embed(ctx, texts)
		if err != nil {
			if !skipErrors {
				return fmt.Errorf("embed narrative memory scenes: %w", err)
			}
			b.vectorWarnings = append(b.vectorWarnings, "vector embedding failed; graph and lexical retrieval remain available: "+err.Error())
			b.index.Vector.Enabled = false
			b.vectors = map[string][]float32{}
			return nil
		}
		if len(vectors) != len(texts) {
			err := fmt.Errorf("embedding returned %d vectors for %d inputs", len(vectors), len(texts))
			if !skipErrors {
				return err
			}
			b.vectorWarnings = append(b.vectorWarnings, err.Error())
			b.index.Vector.Enabled = false
			b.vectors = map[string][]float32{}
			return nil
		}
		for i, vector := range vectors {
			if len(vector) == 0 {
				continue
			}
			scene := missing[start+i]
			b.vectors[scene.ID] = vector
			b.index.Vector.Generated++
			if b.index.Vector.Dimensions == 0 {
				b.index.Vector.Dimensions = len(vector)
			}
		}
	}
	for i := range b.index.Scenes {
		if _, ok := b.vectors[b.index.Scenes[i].ID]; ok {
			b.index.Scenes[i].VectorPosition = i + 1
		}
	}
	b.index.Vector.Count = len(b.vectors)
	if len(b.index.Scenes) > 0 {
		b.index.Vector.Coverage = float64(len(b.vectors)) / float64(len(b.index.Scenes))
	}
	return nil
}

func (b *indexBuilder) addMysteryNodes(chapter flatMemoryChapter) {
	for _, planted := range chapter.Chapter.Mysteries.Planted {
		id := "mystery:" + promiseKey(planted.ID, planted.Clue)
		promise := PromiseRecord{
			ID:           id,
			Kind:         "mystery",
			Text:         strings.TrimSpace(planted.Clue),
			ChapterID:    chapter.Chapter.ID,
			ChapterOrder: chapter.Order,
			Horizon:      strings.TrimSpace(planted.Horizon),
			Status:       "open",
			SourceSHA256: b.sources.Fingerprints["outline"],
			Sources:      []string{chapter.Chapter.ID},
		}
		if due := b.dueChapterOrder(chapter, planted.Horizon); due > 0 {
			promise.DueChapterOrder = due
			promise.DueChapterID = b.chapterIDForOrder(due)
		}
		b.upsertPromise(promise)
		b.addEntityEdge(chapterNodeID(chapter.Chapter.ID), "plants", id, "planned", chapter, 0, 0, planted.Clue, 2)
	}
	for _, resolved := range chapter.Chapter.Mysteries.Resolved {
		id := "mystery:" + promiseKey(resolved.ID, "")
		if index, ok := b.promiseByID[id]; ok {
			b.index.Promises[index].Status = "resolved"
			b.index.Promises[index].ResolvedChapter = chapter.Order
		}
		b.addEntityEdge(chapterNodeID(chapter.Chapter.ID), "resolves", id, "planned", chapter, 0, 0, resolved.Resolution, 2)
	}
	if payoff := chapter.Chapter.ChapterPayoff; payoff != nil && strings.TrimSpace(payoff.Hook) != "" {
		id := "promise:" + hashString("hook|" + strings.TrimSpace(payoff.Hook))[:16]
		b.upsertPromise(PromiseRecord{
			ID: id, Kind: "hook", Text: strings.TrimSpace(payoff.Hook), ChapterID: chapter.Chapter.ID,
			ChapterOrder: chapter.Order, Status: "open", SourceSHA256: b.sources.Fingerprints["outline"],
			Sources: []string{chapter.Chapter.ID},
		})
		b.addEntityEdge(chapterNodeID(chapter.Chapter.ID), "plants", id, "planned", chapter, 0, 0, payoff.Hook, 1)
	}
	if contract := chapter.Volume.PayoffContract; contract != nil && strings.TrimSpace(contract.VolumeQuestion) != "" {
		text := strings.TrimSpace(contract.VolumeQuestion)
		id := "promise:" + hashString("volume_question|" + chapter.Volume.ID + "|" + text)[:16]
		if _, exists := b.promiseByID[id]; !exists {
			due := b.volumeLastOrder(chapter.Volume.ID)
			b.upsertPromise(PromiseRecord{
				ID: id, Kind: "volume_question", Text: text, ChapterID: chapter.Chapter.ID,
				ChapterOrder:    maxInt(1, b.volumeFirstOrder(chapter.Volume.ID)),
				DueChapterOrder: due, DueChapterID: b.chapterIDForOrder(due),
				Status: "open", SourceSHA256: b.sources.Fingerprints["outline"],
				Sources: []string{chapter.Volume.ID},
			})
		}
	}
}

func (b *indexBuilder) addRecapEdges(chapter flatMemoryChapter) {
	recap := b.sources.Recaps[chapter.Chapter.ID]
	if recap == nil {
		return
	}
	for _, text := range uniqueStrings(append(recap.Unresolved, recap.Promises...)...) {
		id := "promise:" + hashString("recap|" + normalizeEntityName(text))[:16]
		b.upsertPromise(PromiseRecord{
			ID: id, Kind: "recap_promise", Text: text, ChapterID: chapter.Chapter.ID,
			ChapterOrder: chapter.Order, LastSeenChapter: chapter.Order, Status: "open",
			SourceSHA256: b.sources.Fingerprints["recap:"+chapter.Chapter.ID],
			Sources:      []string{chapter.Chapter.ID},
		})
		b.addEntityEdge(chapterNodeID(chapter.Chapter.ID), "tracks", id, "actual", chapter, 0, 0, text, 1)
	}
}

func (b *indexBuilder) addCraftEdges(itemsWithTimeline map[string]bool) {
	for _, character := range b.sources.Characters {
		b.addCharacterNode(character.Name, character.RoleInStory, character.Voice, float64(maxInt(character.PowerLevel, 1))/10)
		for _, affiliation := range character.Affiliations {
			b.addOrganizationNode(affiliation, "", 1)
			b.addCanonEdge(entityNodeID("character", character.Name), "affiliated_with", entityNodeID("organization", affiliation), character.Name+" affiliation", 1)
		}
	}
	for _, location := range b.sources.Locations {
		b.addLocationNode(location.Name)
		for _, connected := range location.ConnectedLocations {
			b.addLocationNode(connected)
			b.addCanonEdge(entityNodeID("location", location.Name), "connected_to", entityNodeID("location", connected), location.Name+" connection", 1)
		}
	}
	for _, item := range b.sources.Items {
		b.addItemNode(item.Name, item.Owner, 1)
		if owner := strings.TrimSpace(item.Owner); owner != "" && !itemsWithTimeline[normalizeEntityName(item.Name)] {
			if b.entityExists("character", owner) {
				b.addCanonEdge(entityNodeID("character", owner), "owns", entityNodeID("item", item.Name), item.Name+" owner", 1)
			} else if b.entityExists("organization", owner) {
				b.addCanonEdge(entityNodeID("organization", owner), "owns", entityNodeID("item", item.Name), item.Name+" owner", 1)
			}
		}
	}
	for _, organization := range b.sources.Organizations {
		b.addOrganizationNode(organization.Name, organization.Significance, 1)
		for _, member := range organization.Members {
			b.addCharacterNode(member, "", "", 1)
			b.addCanonEdge(entityNodeID("character", member), "member_of", entityNodeID("organization", organization.Name), organization.Name+" member", 1)
		}
		for _, ally := range organization.Allies {
			b.addOrganizationNode(ally, "", 1)
			b.addCanonEdge(entityNodeID("organization", organization.Name), "allied_with", entityNodeID("organization", ally), organization.Name+" ally", 1)
		}
		for _, enemy := range organization.Enemies {
			b.addOrganizationNode(enemy, "", 1)
			b.addCanonEdge(entityNodeID("organization", organization.Name), "opposes", entityNodeID("organization", enemy), organization.Name+" enemy", 1)
		}
	}
}

type ownershipSpan struct {
	Item        string
	Actor       string
	FromChapter int
	ToChapter   int
	Reason      string
	SourceHash  string
}

func itemOwnershipSpans(chapters []flatMemoryChapter) ([]ownershipSpan, map[string]bool) {
	type openOwner struct {
		item        string
		actor       string
		fromChapter int
		reason      string
		sourceHash  string
	}
	open := map[string]openOwner{}
	spans := []ownershipSpan{}
	itemsWithTimeline := map[string]bool{}
	for _, chapter := range chapters {
		for _, event := range chapter.Chapter.Events {
			targetType := event.GetTargetType()
			if targetType != models.TargetTypeItem {
				continue
			}
			item := strings.TrimSpace(event.GetTarget())
			actor := strings.TrimSpace(event.GetActor())
			if item == "" {
				continue
			}
			key := normalizeEntityName(item)
			switch strings.ToLower(strings.TrimSpace(event.GetAction())) {
			case models.ActionAcquire:
				itemsWithTimeline[key] = true
				if previous, ok := open[key]; ok {
					spans = append(spans, ownershipSpan{
						Item: item, Actor: previous.actor, FromChapter: previous.fromChapter,
						ToChapter: chapter.Order, Reason: previous.reason, SourceHash: previous.sourceHash,
					})
				}
				if actor != "" {
					open[key] = openOwner{
						item: item, actor: actor, fromChapter: chapter.Order,
						reason:     firstNonEmpty(event.Result, event.Details, event.Context, "acquired item"),
						sourceHash: hashString(eventSearchText(event)),
					}
				}
			case models.ActionLose, models.ActionConsume:
				if previous, ok := open[key]; ok {
					spans = append(spans, ownershipSpan{
						Item: item, Actor: previous.actor, FromChapter: previous.fromChapter,
						ToChapter: chapter.Order, Reason: previous.reason, SourceHash: previous.sourceHash,
					})
					delete(open, key)
				}
				itemsWithTimeline[key] = true
			}
		}
	}
	for _, previous := range open {
		spans = append(spans, ownershipSpan{
			Item: previous.item, Actor: previous.actor, FromChapter: previous.fromChapter,
			Reason: previous.reason, SourceHash: previous.sourceHash,
		})
	}
	return spans, itemsWithTimeline
}

func (b *indexBuilder) addOwnershipEdges(spans []ownershipSpan) {
	for _, span := range spans {
		if span.Actor == "" || span.Item == "" {
			continue
		}
		b.addItemNode(span.Item, span.Actor, 1)
		b.addCharacterNode(span.Actor, "", "", 1)
		b.addEdge(EdgeRecord{
			From: entityNodeID("character", span.Actor), Type: "owns", To: entityNodeID("item", span.Item),
			Scope: "actual", ValidFromChapter: span.FromChapter, ValidToChapter: span.ToChapter,
			SourceSHA256: span.SourceHash, Reason: span.Reason, Importance: 1.5,
		})
	}
}

func (b *indexBuilder) finishPromises() {
	maxRecapOrder := 0
	for _, chapter := range b.index.Chapters {
		if chapter.RecapSHA256 != "" && chapter.RecapSHA256 != "missing" && chapter.Order > maxRecapOrder {
			maxRecapOrder = chapter.Order
		}
	}
	for i := range b.index.Promises {
		promise := &b.index.Promises[i]
		if promise.Kind == "recap_promise" && promise.LastSeenChapter > 0 && promise.LastSeenChapter < maxRecapOrder && promise.Status != "resolved" {
			promise.Status = "resolved"
			promise.ResolvedChapter = promise.LastSeenChapter + 1
		}
		b.addNode(NodeRecord{
			ID: promise.ID, Type: "promise", Name: promise.Kind, ChapterID: promise.ChapterID,
			ChapterOrder: promise.ChapterOrder, Scope: "planned", Importance: promiseImportance(*promise),
			Text: promise.Text,
		})
	}
}

func (b *indexBuilder) finishStats() {
	openPromises := 0
	for _, promise := range b.index.Promises {
		if promise.Status != "resolved" {
			openPromises++
		}
	}
	manuscripts := 0
	for _, chapter := range b.index.Chapters {
		if chapter.HasManuscript {
			manuscripts++
		}
	}
	b.index.Stats = Stats{
		Chapters:     len(b.index.Chapters),
		Manuscripts:  manuscripts,
		Scenes:       len(b.index.Scenes),
		Nodes:        len(b.index.Nodes),
		Edges:        len(b.index.Edges),
		Promises:     len(b.index.Promises),
		OpenPromises: openPromises,
	}
	b.index.Vector.Count = len(b.vectors)
}

func (b *indexBuilder) upsertPromise(promise PromiseRecord) {
	if strings.TrimSpace(promise.ID) == "" || strings.TrimSpace(promise.Text) == "" {
		return
	}
	if index, ok := b.promiseByID[promise.ID]; ok {
		existing := &b.index.Promises[index]
		if existing.Status != "resolved" {
			existing.Status = promise.Status
		}
		existing.Sources = uniqueStrings(append(existing.Sources, promise.Sources...)...)
		if existing.DueChapterOrder == 0 && promise.DueChapterOrder > 0 {
			existing.DueChapterOrder = promise.DueChapterOrder
			existing.DueChapterID = promise.DueChapterID
		}
		if promise.LastSeenChapter > existing.LastSeenChapter {
			existing.LastSeenChapter = promise.LastSeenChapter
		}
		return
	}
	b.promiseByID[promise.ID] = len(b.index.Promises)
	b.index.Promises = append(b.index.Promises, promise)
}

func (b *indexBuilder) mentionedEntities(text string) (characters, locations, items, organizations, storylines []string) {
	out := map[string][]string{}
	for kind, names := range b.entityCatalog.Names {
		for normalized := range names {
			display := b.entityCatalog.Text[kind][normalized]
			if display == "" {
				continue
			}
			if containsNormalized(text, display) || containsNormalized(text, normalized) {
				out[kind] = append(out[kind], display)
			}
		}
	}
	return uniqueStrings(out["character"]...), uniqueStrings(out["location"]...),
		uniqueStrings(out["item"]...), uniqueStrings(out["organization"]...), uniqueStrings(out["storyline"]...)
}

func (b *indexBuilder) entityExists(kind, name string) bool {
	names := b.entityCatalog.Names[kind]
	if names == nil {
		return false
	}
	return names[normalizeEntityName(name)]
}

func (b *indexBuilder) addNode(node NodeRecord) {
	if strings.TrimSpace(node.ID) == "" {
		return
	}
	if index, ok := b.nodeByID[node.ID]; ok {
		existing := &b.index.Nodes[index]
		if existing.Text == "" {
			existing.Text = node.Text
		}
		if node.Importance > existing.Importance {
			existing.Importance = node.Importance
		}
		return
	}
	b.nodeByID[node.ID] = len(b.index.Nodes)
	b.index.Nodes = append(b.index.Nodes, node)
}

func (b *indexBuilder) addEntityEdge(from, edgeType, to, scope string, chapter flatMemoryChapter, paragraphStart, paragraphEnd int, reason string, importance float64) {
	b.addEdge(EdgeRecord{
		From: from, Type: edgeType, To: to, Scope: scope,
		ChapterID: chapter.Chapter.ID, ChapterOrder: chapter.Order,
		ValidFromChapter: chapter.Order, ParagraphStart: paragraphStart, ParagraphEnd: paragraphEnd,
		SourceSHA256: b.sources.ContentFingerprint(chapter.Chapter.ID), Reason: reason, Importance: importance,
	})
}

func (b *indexBuilder) addCanonEdge(from, edgeType, to, reason string, importance float64) {
	b.addEdge(EdgeRecord{
		From: from, Type: edgeType, To: to, Scope: "canon",
		ValidFromChapter: 1, Reason: reason, Importance: importance,
		SourceSHA256: b.sources.Fingerprints["craft:characters"],
	})
}

func (b *indexBuilder) addEdge(edge EdgeRecord) {
	if edge.From == "" || edge.To == "" || edge.Type == "" {
		return
	}
	key := edge.From + "|" + edge.Type + "|" + edge.To + "|" + edge.Scope + "|" + strconv.Itoa(edge.ChapterOrder)
	if b.edgeSeen[key] {
		return
	}
	b.edgeSeen[key] = true
	b.index.Edges = append(b.index.Edges, edge)
}

func (b *indexBuilder) addCharacterNode(name, role, voice string, importance float64) {
	if strings.TrimSpace(name) == "" {
		return
	}
	b.addNode(NodeRecord{ID: entityNodeID("character", name), Type: "character", Name: name, Scope: "canon", Importance: importance, Text: strings.TrimSpace(role + " " + voice)})
}

func (b *indexBuilder) addItemNode(name, owner string, importance float64) {
	if strings.TrimSpace(name) == "" {
		return
	}
	b.addNode(NodeRecord{ID: entityNodeID("item", name), Type: "item", Name: name, Scope: "canon", Importance: importance, Text: owner})
}

func (b *indexBuilder) addLocationNode(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	b.addNode(NodeRecord{ID: entityNodeID("location", name), Type: "location", Name: name, Scope: "canon", Importance: 1})
}

func (b *indexBuilder) addOrganizationNode(name, text string, importance float64) {
	if strings.TrimSpace(name) == "" {
		return
	}
	b.addNode(NodeRecord{ID: entityNodeID("organization", name), Type: "organization", Name: name, Scope: "canon", Importance: importance, Text: text})
}

func (b *indexBuilder) addStorylineNode(name string, importance float64) {
	if strings.TrimSpace(name) == "" {
		return
	}
	b.addNode(NodeRecord{ID: entityNodeID("storyline", name), Type: "storyline", Name: name, Scope: "canon", Importance: importance})
}

func (s *canonicalSources) ContentFingerprint(chapterID string) string {
	if s == nil {
		return ""
	}
	return s.Fingerprints["chapter:"+chapterID]
}

func (b *indexBuilder) dueChapterOrder(chapter flatMemoryChapter, horizon string) int {
	normalized := strings.ReplaceAll(strings.ReplaceAll(normalizeEntityName(horizon), "_", ""), "-", "")
	switch normalized {
	case "thisvolume", "this-volume", "本卷":
		return b.volumeLastOrder(chapter.Volume.ID)
	case "nextvolume", "下一卷":
		return b.nextVolumeLastOrder(chapter.Volume.ID)
	case "book", "全书", "series", "系列":
		if len(b.sources.Chapters) == 0 {
			return 0
		}
		return b.sources.Chapters[len(b.sources.Chapters)-1].Order
	default:
		return 0
	}
}

func (b *indexBuilder) volumeFirstOrder(volumeID string) int {
	for _, chapter := range b.sources.Chapters {
		if chapter.Volume.ID == volumeID {
			return chapter.Order
		}
	}
	return 0
}

func (b *indexBuilder) volumeLastOrder(volumeID string) int {
	last := 0
	for _, chapter := range b.sources.Chapters {
		if chapter.Volume.ID == volumeID {
			last = chapter.Order
		}
	}
	return last
}

func (b *indexBuilder) nextVolumeLastOrder(volumeID string) int {
	volumeIDs := []string{}
	for _, chapter := range b.sources.Chapters {
		if len(volumeIDs) == 0 || volumeIDs[len(volumeIDs)-1] != chapter.Volume.ID {
			volumeIDs = append(volumeIDs, chapter.Volume.ID)
		}
	}
	for i, id := range volumeIDs {
		if id == volumeID && i+1 < len(volumeIDs) {
			return b.volumeLastOrder(volumeIDs[i+1])
		}
	}
	return b.volumeLastOrder(volumeID)
}

func (b *indexBuilder) chapterIDForOrder(order int) string {
	for _, chapter := range b.sources.Chapters {
		if chapter.Order == order {
			return chapter.Chapter.ID
		}
	}
	return ""
}

func paragraphWindows(paragraphs []project.ChapterParagraph, size, stride int) [][]project.ChapterParagraph {
	if len(paragraphs) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(paragraphs)
	}
	if stride <= 0 {
		stride = size
	}
	out := make([][]project.ChapterParagraph, 0)
	for start := 0; start < len(paragraphs); start += stride {
		end := start + size
		if end > len(paragraphs) {
			end = len(paragraphs)
		}
		out = append(out, paragraphs[start:end])
		if end == len(paragraphs) {
			break
		}
	}
	return out
}

func joinParagraphText(paragraphs []project.ChapterParagraph) string {
	values := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph.Text) != "" {
			values = append(values, strings.TrimSpace(paragraph.Text))
		}
	}
	return strings.Join(values, "\n\n")
}

func outlineSceneMetadata(chapter *models.Chapter, windowIndex, windowCount int) (string, string, []string) {
	if chapter == nil || len(chapter.Scenes) == 0 || windowCount <= 0 {
		return "", "", nil
	}
	index := windowIndex * len(chapter.Scenes) / windowCount
	if index >= len(chapter.Scenes) {
		index = len(chapter.Scenes) - 1
	}
	scene := chapter.Scenes[index]
	return strings.TrimSpace(scene.POV), strings.TrimSpace(scene.Location), append([]string(nil), scene.Characters...)
}

func chapterPOV(chapter *models.Chapter) string {
	if chapter == nil {
		return ""
	}
	for _, scene := range chapter.Scenes {
		if strings.TrimSpace(scene.POV) != "" {
			return strings.TrimSpace(scene.POV)
		}
	}
	return ""
}

func chapterStorylines(chapter *models.Chapter) []string {
	if chapter == nil {
		return nil
	}
	out := make([]string, 0, len(chapter.StorylineAdvances))
	for _, advance := range chapter.StorylineAdvances {
		out = append(out, advance.StorylineName)
	}
	return uniqueStrings(out...)
}

func recapSignalTexts(recap *models.ChapterRecap) []string {
	if recap == nil {
		return nil
	}
	values := append([]string{}, recap.PlotBeats...)
	values = append(values, recap.Decisions...)
	values = append(values, recap.Reveals...)
	values = append(values, recap.Unresolved...)
	values = append(values, recap.Promises...)
	values = append(values, recap.Status...)
	values = append(values, recap.Items...)
	return uniqueStrings(values...)
}

func plantedMysteryIDs(chapter *models.Chapter) []string {
	if chapter == nil {
		return nil
	}
	out := make([]string, 0, len(chapter.Mysteries.Planted))
	for _, planted := range chapter.Mysteries.Planted {
		out = append(out, promiseKey(planted.ID, planted.Clue))
	}
	return uniqueStrings(out...)
}

func eventSearchText(event models.Event) string {
	return strings.Join(uniqueStrings(
		event.GetActor(), event.GetAction(), event.GetTarget(), event.GetTargetType(),
		event.Subject, event.Change, event.Context, event.Details, event.Result,
	), " ")
}

func promiseImportance(promise PromiseRecord) float64 {
	switch promise.Kind {
	case "volume_question":
		return 3
	case "mystery":
		return 2.5
	case "hook":
		return 1.5
	default:
		return 1.2
	}
}

func promiseKey(id, fallback string) string {
	value := strings.TrimSpace(id)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return "unknown"
	}
	return normalizeEntityName(value)
}

func entityNodeID(kind, name string) string {
	return kind + ":" + normalizeEntityName(name)
}

func chapterNodeID(id string) string {
	return "chapter:" + strings.TrimSpace(id)
}

func normalizeEntityName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), " ")
}

func containsNormalized(text, value string) bool {
	text = normalizeEntityName(text)
	value = normalizeEntityName(value)
	return value != "" && strings.Contains(text, value)
}

func uniqueStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeEntityName(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func appendUnique(value string, values []string) []string {
	key := normalizeEntityName(value)
	for _, value := range values {
		if normalizeEntityName(value) == key {
			return values
		}
	}
	return append(values, strings.TrimSpace(value))
}

func displayNames(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			out = append(out, key)
		}
	}
	return uniqueStrings(out...)
}

func vectorText(scene SceneRecord) string {
	return strings.Join([]string{
		scene.ChapterTitle,
		strings.Join(scene.Characters, ", "),
		strings.Join(scene.Locations, ", "),
		strings.Join(scene.Storylines, ", "),
		scene.Text,
	}, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
