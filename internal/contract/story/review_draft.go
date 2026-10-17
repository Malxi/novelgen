package models

// DraftReview contains review results for a single draft.
//
// It lives in the shared story contract rather than the draft agent package:
// persistence, continuity checks and CLI commands all consume it, and none of
// them should have to import the agent pipeline to name the review shape.
type DraftReview struct {
	ChapterID            string                `json:"chapter_id"`
	ChapterTitle         string                `json:"chapter_title"`
	OverallScore         int                   `json:"overall_score"`         // 1-10
	PlotCoherence        PlotCoherenceReview   `json:"plot_coherence"`        // 剧情连贯性
	PlotRationality      RationalityReview     `json:"plot_rationality"`      // 情节合理性
	CharacterConsistency CharacterReview       `json:"character_consistency"` // 角色一致性
	PacingReview         PacingReview          `json:"pacing_review"`         // 节奏评价
	SceneContinuity      SceneContinuityReview `json:"scene_continuity"`      // 场景/转场连续性
	CharacterPresence    PresenceReview        `json:"character_presence"`    // 角色出场一致性（启发式）
	RecapQuality         GateReview            `json:"recap_quality"`         // Recap 质量门禁（启发式）
	Suggestions          []string              `json:"suggestions"`           // 改进建议
	NeedsRevision        bool                  `json:"needs_revision"`        // 是否需要重写
}

// PlotCoherenceReview evaluates plot continuity.
type PlotCoherenceReview struct {
	Score       int      `json:"score"`       // 1-10
	Issues      []string `json:"issues"`      // 连贯性问题
	Suggestions []string `json:"suggestions"` // 改进建议
}

// RationalityReview evaluates plot logic.
type RationalityReview struct {
	Score       int      `json:"score"`       // 1-10
	LogicFlaws  []string `json:"logic_flaws"` // 逻辑漏洞
	Suggestions []string `json:"suggestions"` // 改进建议
}

// CharacterReview evaluates character consistency.
type CharacterReview struct {
	Score           int      `json:"score"`           // 1-10
	Inconsistencies []string `json:"inconsistencies"` // 角色不一致之处
	Suggestions     []string `json:"suggestions"`     // 改进建议
}

// PacingReview evaluates story pacing.
type PacingReview struct {
	Score       int      `json:"score"`       // 1-10
	Issues      []string `json:"issues"`      // 节奏问题
	Suggestions []string `json:"suggestions"` // 改进建议
}

// SceneContinuityReview evaluates scene-to-scene continuity.
type SceneContinuityReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// PresenceReview evaluates whether outlined entities actually appear.
type PresenceReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// GateReview is a generic quality gate result.
type GateReview struct {
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

// VolumeReview contains reviews for all drafts in a volume.
type VolumeReview struct {
	VolumeID    string        `json:"volume_id"`
	VolumeTitle string        `json:"volume_title"`
	Reviews     []DraftReview `json:"reviews"`
	Summary     string        `json:"summary"`
}
