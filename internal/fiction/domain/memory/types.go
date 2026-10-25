// Package memory builds a rebuildable narrative index over canonical project
// artifacts. The index combines a deterministic story graph, scene-level
// lexical retrieval, optional vectors and graph-aware context selection.
//
// The index is never canon. Setup, outline, craft, recaps and chapter markdown
// remain the source of truth and can always regenerate the index.
package memory

import (
	"context"
	"time"
)

const IndexVersion = 2

type Index struct {
	Version            int               `json:"version"`
	BuiltAt            time.Time         `json:"built_at"`
	SourceFingerprints map[string]string `json:"source_fingerprints"`
	Chapters           []ChapterRecord   `json:"chapters"`
	Scenes             []SceneRecord     `json:"scenes"`
	Nodes              []NodeRecord      `json:"nodes"`
	Edges              []EdgeRecord      `json:"edges"`
	Promises           []PromiseRecord   `json:"promises"`
	Vector             VectorInfo        `json:"vector"`
	Stats              Stats             `json:"stats"`
}

type ChapterRecord struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	VolumeID      string   `json:"volume_id"`
	Order         int      `json:"order"`
	Summary       string   `json:"summary,omitempty"`
	Conflict      string   `json:"conflict,omitempty"`
	Characters    []string `json:"characters,omitempty"`
	Location      string   `json:"location,omitempty"`
	Timeline      string   `json:"timeline,omitempty"`
	POV           string   `json:"pov,omitempty"`
	Storylines    []string `json:"storylines,omitempty"`
	HasManuscript bool     `json:"has_manuscript"`
	ContentSHA256 string   `json:"content_sha256,omitempty"`
	RecapSHA256   string   `json:"recap_sha256,omitempty"`
	RecapStatus   []string `json:"recap_status,omitempty"`
	RecapItems    []string `json:"recap_items,omitempty"`
	RecapLastLine string   `json:"recap_last_line,omitempty"`
	RecapNextHint string   `json:"recap_next_opening_hint,omitempty"`
}

type SceneRecord struct {
	ID               string   `json:"id"`
	ChapterID        string   `json:"chapter_id"`
	ChapterTitle     string   `json:"chapter_title"`
	ChapterOrder     int      `json:"chapter_order"`
	ParagraphStart   int      `json:"paragraph_start"`
	ParagraphEnd     int      `json:"paragraph_end"`
	Text             string   `json:"text"`
	TextSHA256       string   `json:"text_sha256"`
	POV              string   `json:"pov,omitempty"`
	Characters       []string `json:"characters,omitempty"`
	Locations        []string `json:"locations,omitempty"`
	Items            []string `json:"items,omitempty"`
	Organizations    []string `json:"organizations,omitempty"`
	Storylines       []string `json:"storylines,omitempty"`
	PlantedMysteries []string `json:"planted_mysteries,omitempty"`
	RecapSignals     []string `json:"recap_signals,omitempty"`
	Importance       float64  `json:"importance,omitempty"`
	VectorPosition   int      `json:"vector_position,omitempty"`
}

type NodeRecord struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Name         string  `json:"name"`
	ChapterID    string  `json:"chapter_id,omitempty"`
	ChapterOrder int     `json:"chapter_order,omitempty"`
	Scope        string  `json:"scope,omitempty"` // planned | actual | canon
	Importance   float64 `json:"importance,omitempty"`
	Text         string  `json:"text,omitempty"`
}

type EdgeRecord struct {
	From             string  `json:"from"`
	Type             string  `json:"type"`
	To               string  `json:"to"`
	Scope            string  `json:"scope,omitempty"` // planned | actual | canon
	ChapterID        string  `json:"chapter_id,omitempty"`
	ChapterOrder     int     `json:"chapter_order,omitempty"`
	ValidFromChapter int     `json:"valid_from_chapter,omitempty"`
	ValidToChapter   int     `json:"valid_to_chapter,omitempty"`
	ParagraphStart   int     `json:"paragraph_start,omitempty"`
	ParagraphEnd     int     `json:"paragraph_end,omitempty"`
	SourceSHA256     string  `json:"source_sha256,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	Importance       float64 `json:"importance,omitempty"`
}

type PromiseRecord struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"` // mystery | recap_promise | hook | volume_question
	Text            string   `json:"text"`
	ChapterID       string   `json:"chapter_id"`
	ChapterOrder    int      `json:"chapter_order"`
	DueChapterID    string   `json:"due_chapter_id,omitempty"`
	DueChapterOrder int      `json:"due_chapter_order,omitempty"`
	Horizon         string   `json:"horizon,omitempty"`
	Status          string   `json:"status"` // open | resolved | unknown
	LastSeenChapter int      `json:"last_seen_chapter,omitempty"`
	ResolvedChapter int      `json:"resolved_chapter,omitempty"`
	SourceSHA256    string   `json:"source_sha256,omitempty"`
	Sources         []string `json:"sources,omitempty"`
}

type VectorInfo struct {
	Enabled    bool    `json:"enabled"`
	Model      string  `json:"model,omitempty"`
	Dimensions int     `json:"dimensions,omitempty"`
	Count      int     `json:"count,omitempty"`
	FileSHA256 string  `json:"file_sha256,omitempty"`
	Reused     int     `json:"reused,omitempty"`
	Generated  int     `json:"generated,omitempty"`
	Coverage   float64 `json:"coverage,omitempty"`
}

type Stats struct {
	Chapters     int `json:"chapters"`
	Manuscripts  int `json:"manuscripts"`
	Scenes       int `json:"scenes"`
	Nodes        int `json:"nodes"`
	Edges        int `json:"edges"`
	Promises     int `json:"promises"`
	OpenPromises int `json:"open_promises"`
}

type BuildOptions struct {
	Embedder         Embedder
	Previous         *Index
	PreviousVectors  map[string][]float32
	SceneWindow      int
	SceneStride      int
	SkipVectorErrors bool
	SkipEmbedding    bool
	Force            bool
}

type BuildResult struct {
	Index          *Index
	Vectors        map[string][]float32
	Stats          Stats
	UpToDate       bool
	ChangedSources []string
	Warnings       []string
}

type RefreshResult struct {
	OK             bool       `json:"ok"`
	Status         string     `json:"status"` // rebuilt | up_to_date
	IndexPath      string     `json:"index_path"`
	VectorsPath    string     `json:"vectors_path,omitempty"`
	BuiltAt        time.Time  `json:"built_at"`
	ChangedSources []string   `json:"changed_sources,omitempty"`
	Stats          Stats      `json:"stats"`
	Vector         VectorInfo `json:"vector"`
	Warnings       []string   `json:"warnings,omitempty"`
}

type InspectResult struct {
	Status         string     `json:"status"` // missing | fresh | stale | error
	IndexPath      string     `json:"index_path"`
	VectorsPath    string     `json:"vectors_path,omitempty"`
	BuiltAt        *time.Time `json:"built_at,omitempty"`
	ChangedSources []string   `json:"changed_sources,omitempty"`
	Stats          Stats      `json:"stats,omitempty"`
	Vector         VectorInfo `json:"vector,omitempty"`
	Error          string     `json:"error,omitempty"`
}

type Embedder interface {
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type QueryOptions struct {
	ChapterID     string
	Intent        string
	Focus         string
	Limit         int
	MaxChars      int
	IncludeTarget bool
	Embedder      Embedder
}

type ContextResult struct {
	ChapterID    string           `json:"chapter_id"`
	ChapterOrder int              `json:"chapter_order"`
	Intent       string           `json:"intent"`
	MustInclude  MustIncludeFacts `json:"must_include"`
	Promises     []PromiseHit     `json:"promises,omitempty"`
	Retrieved    []RetrievedScene `json:"retrieved,omitempty"`
	Citations    []Citation       `json:"citations,omitempty"`
	Meta         QueryMeta        `json:"meta"`
	Warnings     []string         `json:"warnings,omitempty"`
}

type MustIncludeFacts struct {
	ActiveCharacters []string `json:"active_characters,omitempty"`
	Locations        []string `json:"locations,omitempty"`
	Storylines       []string `json:"storylines,omitempty"`
	CurrentState     []string `json:"current_state,omitempty"`
	PreviousRecap    string   `json:"previous_recap,omitempty"`
}

type PromiseHit struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Text            string `json:"text"`
	ChapterID       string `json:"chapter_id"`
	DueChapterID    string `json:"due_chapter_id,omitempty"`
	DueChapterOrder int    `json:"due_chapter_order,omitempty"`
	Horizon         string `json:"horizon,omitempty"`
	Status          string `json:"status"`
	Reason          string `json:"reason,omitempty"`
}

type RetrievedScene struct {
	ID             string   `json:"id"`
	ChapterID      string   `json:"chapter_id"`
	ChapterTitle   string   `json:"chapter_title"`
	ChapterOrder   int      `json:"chapter_order"`
	ParagraphStart int      `json:"paragraph_start"`
	ParagraphEnd   int      `json:"paragraph_end"`
	Text           string   `json:"text"`
	SourceSHA256   string   `json:"source_sha256"`
	Score          float64  `json:"score"`
	Reasons        []string `json:"reasons,omitempty"`
	Characters     []string `json:"characters,omitempty"`
	Locations      []string `json:"locations,omitempty"`
	Storylines     []string `json:"storylines,omitempty"`
}

type Citation struct {
	ChapterID      string `json:"chapter_id"`
	ParagraphStart int    `json:"paragraph_start"`
	ParagraphEnd   int    `json:"paragraph_end"`
	SHA256         string `json:"sha256"`
}

type QueryMeta struct {
	Status              string `json:"status"` // fresh | ephemeral | stale
	IndexBuiltAt        string `json:"index_built_at,omitempty"`
	SourceFingerprint   string `json:"source_fingerprint,omitempty"`
	LexicalCandidates   int    `json:"lexical_candidates"`
	GraphCandidates     int    `json:"graph_candidates"`
	VectorCandidates    int    `json:"vector_candidates"`
	ReturnedScenes      int    `json:"returned_scenes"`
	ReturnedChars       int    `json:"returned_chars"`
	MaxChars            int    `json:"max_chars"`
	VectorUsed          bool   `json:"vector_used"`
	VectorModel         string `json:"vector_model,omitempty"`
	SceneCount          int    `json:"scene_count"`
	NodeCount           int    `json:"node_count"`
	EdgeCount           int    `json:"edge_count"`
	OpenPromises        int    `json:"open_promises"`
	ElapsedMilliseconds int64  `json:"elapsed_ms"`
}
