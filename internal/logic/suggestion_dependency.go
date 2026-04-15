package logic

import (
	"strings"

	"novelgen/internal/models"
)

// SuggestionNode represents a suggestion in the dependency tree
type SuggestionNode struct {
	ID           string
	Type         string // "part", "volume", "chapter"
	Suggestion   models.ReviewSuggestion
	Dependencies []string // IDs of suggestions this depends on
	Level        int      // Tree level: 0=part, 1=volume, 2=chapter
}

// SuggestionDependencyAnalyzer analyzes dependencies between suggestions
type SuggestionDependencyAnalyzer struct {
	outline *models.Outline
	idMgr   *IDManager
}

// NewSuggestionDependencyAnalyzer creates a new analyzer
func NewSuggestionDependencyAnalyzer(outline *models.Outline) *SuggestionDependencyAnalyzer {
	return &SuggestionDependencyAnalyzer{
		outline: outline,
		idMgr:   NewIDManager(outline),
	}
}

// BuildDependencyTree builds a dependency tree from suggestions
// Rules:
// 1. Part suggestions have no dependencies (they are roots)
// 2. Volume suggestions depend on their parent Part
// 3. Chapter suggestions depend on their parent Volume AND previous Chapter (for continuity)
func (a *SuggestionDependencyAnalyzer) BuildDependencyTree(suggestions []models.ReviewSuggestion) []*SuggestionNode {
	// First, categorize all suggestions by type
	partSugs := make(map[string][]models.ReviewSuggestion)
	volumeSugs := make(map[string][]models.ReviewSuggestion)
	chapterSugs := make(map[string][]models.ReviewSuggestion)

	for _, s := range suggestions {
		targetType := a.determineTargetType(s.TargetID)
		switch targetType {
		case "part":
			partSugs[s.TargetID] = append(partSugs[s.TargetID], s)
		case "volume":
			volumeSugs[s.TargetID] = append(volumeSugs[s.TargetID], s)
		case "chapter":
			chapterSugs[s.TargetID] = append(chapterSugs[s.TargetID], s)
		}
	}

	// Build nodes
	var nodes []*SuggestionNode
	nodeMap := make(map[string]*SuggestionNode)

	// Create part nodes (level 0, no dependencies)
	for partID, sugs := range partSugs {
		for _, s := range sugs {
			node := &SuggestionNode{
				ID:           a.makeNodeID(partID, s),
				Type:         "part",
				Suggestion:   s,
				Dependencies: []string{}, // Parts have no dependencies
				Level:        0,
			}
			nodes = append(nodes, node)
			nodeMap[node.ID] = node
		}
	}

	// Create volume nodes (level 1, depend on parent part)
	for volID, sugs := range volumeSugs {
		parentPartID := a.getParentPartID(volID)
		for _, s := range sugs {
			node := &SuggestionNode{
				ID:         a.makeNodeID(volID, s),
				Type:       "volume",
				Suggestion: s,
				Level:      1,
			}
			// Depend on any suggestions for the parent part
			for _, partNode := range nodes {
				if strings.HasPrefix(partNode.ID, parentPartID+"_") {
					node.Dependencies = append(node.Dependencies, partNode.ID)
				}
			}
			nodes = append(nodes, node)
			nodeMap[node.ID] = node
		}
	}

	// Create chapter nodes (level 2, depend on parent volume and previous chapter)
	for chapID, sugs := range chapterSugs {
		parentVolID := a.getParentVolumeID(chapID)
		prevChapID := a.getPreviousChapterID(chapID)

		for _, s := range sugs {
			node := &SuggestionNode{
				ID:         a.makeNodeID(chapID, s),
				Type:       "chapter",
				Suggestion: s,
				Level:      2,
			}

			// Depend on suggestions for parent volume
			for _, volNode := range nodes {
				if volNode.Type == "volume" && strings.HasPrefix(volNode.ID, parentVolID+"_") {
					node.Dependencies = append(node.Dependencies, volNode.ID)
				}
			}

			// Depend on suggestions for previous chapter (for continuity)
			if prevChapID != "" {
				for _, chapNode := range nodes {
					if chapNode.Type == "chapter" && strings.HasPrefix(chapNode.ID, prevChapID+"_") {
						node.Dependencies = append(node.Dependencies, chapNode.ID)
					}
				}
			}

			nodes = append(nodes, node)
			nodeMap[node.ID] = node
		}
	}

	return nodes
}

// determineTargetType determines the type from ID
func (a *SuggestionDependencyAnalyzer) determineTargetType(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))

	// New format
	if strings.HasPrefix(id, "P") {
		if strings.Contains(id, "-C") {
			return "chapter"
		}
		if strings.Contains(id, "-V") {
			return "volume"
		}
		return "part"
	}

	// Legacy format
	parts := strings.Split(id, "_")
	switch len(parts) {
	case 1:
		return "part"
	case 2:
		return "volume"
	case 3:
		return "chapter"
	}
	return ""
}

// makeNodeID creates a unique node ID from suggestion
func (a *SuggestionDependencyAnalyzer) makeNodeID(targetID string, s models.ReviewSuggestion) string {
	// Include suggestion category and target ID to make it unique
	return targetID + "_" + s.Category + "_" + hashString(s.Suggestion)[:8]
}

// hashString creates a simple hash for string
func hashString(s string) string {
	var h int
	for i, c := range s {
		h += int(c) * (i + 1)
	}
	return string(rune('a' + (h % 26)))
}

// getParentPartID gets the parent part ID from a volume or chapter ID
func (a *SuggestionDependencyAnalyzer) getParentPartID(id string) string {
	id = strings.ToUpper(id)
	if strings.HasPrefix(id, "P") {
		parts := strings.Split(id, "-")
		if len(parts) >= 1 {
			return parts[0]
		}
	}
	return ""
}

// getParentVolumeID gets the parent volume ID from a chapter ID
func (a *SuggestionDependencyAnalyzer) getParentVolumeID(id string) string {
	id = strings.ToUpper(id)
	if strings.HasPrefix(id, "P") && strings.Contains(id, "-V") {
		parts := strings.Split(id, "-C")
		if len(parts) >= 1 {
			return parts[0]
		}
	}
	return ""
}

// getPreviousChapterID gets the ID of the previous chapter
func (a *SuggestionDependencyAnalyzer) getPreviousChapterID(chapID string) string {
	// Parse chapter ID
	partNum, volNum, chapNum, err := a.idMgr.ParseChapterID(chapID)
	if err != nil {
		return ""
	}

	// Get previous chapter number
	if chapNum > 1 {
		prevChapID := a.idMgr.GenerateChapterID(partNum, volNum, chapNum-1)
		return prevChapID
	}

	// If first chapter in volume, check if there's a previous volume
	if volNum > 1 {
		prevVolID := a.idMgr.GenerateVolumeID(partNum, volNum-1)
		// Find last chapter in previous volume
		for _, part := range a.outline.Parts {
			for _, vol := range part.Volumes {
				if vol.ID == prevVolID && len(vol.Chapters) > 0 {
					return vol.Chapters[len(vol.Chapters)-1].ID
				}
			}
		}
	}

	return ""
}

// GetExecutionOrder returns suggestions in dependency-respecting order
// Items at the same level can be executed concurrently
func (a *SuggestionDependencyAnalyzer) GetExecutionOrder(nodes []*SuggestionNode) [][]*SuggestionNode {
	// Group by level for concurrent execution
	levelMap := make(map[int][]*SuggestionNode)
	maxLevel := 0

	for _, node := range nodes {
		levelMap[node.Level] = append(levelMap[node.Level], node)
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}

	// Convert to slice of slices
	var order [][]*SuggestionNode
	for i := 0; i <= maxLevel; i++ {
		if nodes, ok := levelMap[i]; ok && len(nodes) > 0 {
			order = append(order, nodes)
		}
	}

	return order
}
