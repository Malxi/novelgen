package logic

import (
	"fmt"
	"strconv"
	"strings"

	"nolvegen/internal/models"
)

// IDManager handles ID generation and mapping for story structure elements
type IDManager struct {
	outline *models.Outline
}

// NewIDManager creates a new IDManager
func NewIDManager(outline *models.Outline) *IDManager {
	return &IDManager{outline: outline}
}

// GeneratePartID generates a part ID (e.g., "P1", "P2")
func (m *IDManager) GeneratePartID(partNum int) string {
	return fmt.Sprintf("P%d", partNum)
}

// GenerateVolumeID generates a volume ID (e.g., "P1-V1", "P2-V3")
func (m *IDManager) GenerateVolumeID(partNum, volumeNum int) string {
	return fmt.Sprintf("P%d-V%d", partNum, volumeNum)
}

// GenerateChapterID generates a chapter ID (e.g., "P1-V1-C1", "P2-V3-C5")
func (m *IDManager) GenerateChapterID(partNum, volumeNum, chapterNum int) string {
	return fmt.Sprintf("P%d-V%d-C%d", partNum, volumeNum, chapterNum)
}

// ParsePartID parses a part ID and returns the part number
func (m *IDManager) ParsePartID(partID string) (int, error) {
	partID = strings.TrimPrefix(partID, "P")
	return strconv.Atoi(partID)
}

// ParseVolumeID parses a volume ID and returns part and volume numbers
func (m *IDManager) ParseVolumeID(volumeID string) (partNum, volumeNum int, err error) {
	volumeID = strings.ToUpper(volumeID)
	parts := strings.Split(volumeID, "-V")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid volume ID format: %s", volumeID)
	}
	partNum, err = strconv.Atoi(strings.TrimPrefix(parts[0], "P"))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid part number in volume ID: %s", volumeID)
	}
	volumeNum, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid volume number in volume ID: %s", volumeID)
	}
	return partNum, volumeNum, nil
}

// ParseChapterID parses a chapter ID and returns part, volume, and chapter numbers
func (m *IDManager) ParseChapterID(chapterID string) (partNum, volumeNum, chapterNum int, err error) {
	chapterID = strings.ToUpper(chapterID)
	parts := strings.Split(chapterID, "-")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid chapter ID format: %s", chapterID)
	}
	partNum, err = strconv.Atoi(strings.TrimPrefix(parts[0], "P"))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid part number in chapter ID: %s", chapterID)
	}
	volumeNum, err = strconv.Atoi(strings.TrimPrefix(parts[1], "V"))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid volume number in chapter ID: %s", chapterID)
	}
	chapterNum, err = strconv.Atoi(strings.TrimPrefix(parts[2], "C"))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid chapter number in chapter ID: %s", chapterID)
	}
	return partNum, volumeNum, chapterNum, nil
}

// ResolvePartID resolves a part number or ID to a part ID
func (m *IDManager) ResolvePartID(partInput string) (string, error) {
	// If it's already a valid part ID (e.g., "P1", "p2")
	if strings.HasPrefix(strings.ToUpper(partInput), "P") {
		num, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(partInput), "P"))
		if err == nil && num > 0 {
			return m.GeneratePartID(num), nil
		}
	}

	// Otherwise, treat as a number
	partNum, err := strconv.Atoi(partInput)
	if err != nil {
		return "", fmt.Errorf("invalid part input: %s", partInput)
	}
	if partNum < 1 {
		return "", fmt.Errorf("part number must be >= 1: %d", partNum)
	}

	return m.GeneratePartID(partNum), nil
}

// ResolveVolumeID resolves volume input to a volume ID
// Supports: "1" (global volume number), "P1-V1" (full ID), "1-2" (part-volume format)
func (m *IDManager) ResolveVolumeID(volumeInput string, partInput string) (string, error) {
	// If full volume ID provided (e.g., "P1-V1")
	if strings.Contains(strings.ToUpper(volumeInput), "-V") {
		partNum, volNum, err := m.ParseVolumeID(volumeInput)
		if err != nil {
			return "", err
		}
		return m.GenerateVolumeID(partNum, volNum), nil
	}

	// If part is specified, treat volumeInput as volume number within that part
	if partInput != "" {
		partID, err := m.ResolvePartID(partInput)
		if err != nil {
			return "", err
		}
		partNum, _ := m.ParsePartID(partID)

		volNum, err := strconv.Atoi(volumeInput)
		if err != nil {
			return "", fmt.Errorf("invalid volume number: %s", volumeInput)
		}
		if volNum < 1 {
			return "", fmt.Errorf("volume number must be >= 1: %d", volNum)
		}

		return m.GenerateVolumeID(partNum, volNum), nil
	}

	// Otherwise, treat as global volume number
	globalVolNum, err := strconv.Atoi(volumeInput)
	if err != nil {
		return "", fmt.Errorf("invalid volume input: %s", volumeInput)
	}
	if globalVolNum < 1 {
		return "", fmt.Errorf("volume number must be >= 1: %d", globalVolNum)
	}

	// Find the volume by global index
	currentVol := 0
	for partIdx, part := range m.outline.Parts {
		for volIdx := range part.Volumes {
			currentVol++
			if currentVol == globalVolNum {
				return m.GenerateVolumeID(partIdx+1, volIdx+1), nil
			}
		}
	}

	return "", fmt.Errorf("volume %d not found", globalVolNum)
}

// ResolveChapterID resolves chapter input to a chapter ID
// Supports: "1" (global chapter number), "P1-V1-C1" (full ID), or with part/volume context
func (m *IDManager) ResolveChapterID(chapterInput string, partInput string, volumeInput string) (string, error) {
	// If full chapter ID provided (e.g., "P1-V1-C1")
	if strings.Count(strings.ToUpper(chapterInput), "-") == 2 {
		partNum, volNum, chapNum, err := m.ParseChapterID(chapterInput)
		if err != nil {
			return "", err
		}
		return m.GenerateChapterID(partNum, volNum, chapNum), nil
	}

	// If part and volume are specified
	if partInput != "" && volumeInput != "" {
		partID, err := m.ResolvePartID(partInput)
		if err != nil {
			return "", err
		}
		partNum, _ := m.ParsePartID(partID)

		volumeID, err := m.ResolveVolumeID(volumeInput, partInput)
		if err != nil {
			return "", err
		}
		_, volNum, _ := m.ParseVolumeID(volumeID)

		chapNum, err := strconv.Atoi(chapterInput)
		if err != nil {
			return "", fmt.Errorf("invalid chapter number: %s", chapterInput)
		}
		if chapNum < 1 {
			return "", fmt.Errorf("chapter number must be >= 1: %d", chapNum)
		}

		return m.GenerateChapterID(partNum, volNum, chapNum), nil
	}

	// Otherwise, treat as global chapter number
	globalChapNum, err := strconv.Atoi(chapterInput)
	if err != nil {
		return "", fmt.Errorf("invalid chapter input: %s", chapterInput)
	}
	if globalChapNum < 1 {
		return "", fmt.Errorf("chapter number must be >= 1: %d", globalChapNum)
	}

	// Find the chapter by global index
	currentChap := 0
	for partIdx, part := range m.outline.Parts {
		for volIdx, vol := range part.Volumes {
			for chapIdx := range vol.Chapters {
				currentChap++
				if currentChap == globalChapNum {
					return m.GenerateChapterID(partIdx+1, volIdx+1, chapIdx+1), nil
				}
			}
		}
	}

	return "", fmt.Errorf("chapter %d not found", globalChapNum)
}

// GetPartByID finds a part by its ID
func (m *IDManager) GetPartByID(partID string) *models.Part {
	for i := range m.outline.Parts {
		expectedID := m.GeneratePartID(i + 1)
		if m.outline.Parts[i].ID == partID || expectedID == partID {
			return &m.outline.Parts[i]
		}
	}
	return nil
}

// GetVolumeByID finds a volume by its ID
func (m *IDManager) GetVolumeByID(volumeID string) (*models.Volume, *models.Part) {
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			expectedID := m.GenerateVolumeID(partIdx+1, volIdx+1)
			if m.outline.Parts[partIdx].Volumes[volIdx].ID == volumeID || expectedID == volumeID {
				return &m.outline.Parts[partIdx].Volumes[volIdx], &m.outline.Parts[partIdx]
			}
		}
	}
	return nil, nil
}

// GetChapterByID finds a chapter by its ID
func (m *IDManager) GetChapterByID(chapterID string) (*models.Chapter, *models.Volume, *models.Part) {
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			for chapIdx := range m.outline.Parts[partIdx].Volumes[volIdx].Chapters {
				expectedID := m.GenerateChapterID(partIdx+1, volIdx+1, chapIdx+1)
				if m.outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx].ID == chapterID || expectedID == chapterID {
					return &m.outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx],
						&m.outline.Parts[partIdx].Volumes[volIdx],
						&m.outline.Parts[partIdx]
				}
			}
		}
	}
	return nil, nil, nil
}

// AssignIDsToOutline assigns generated IDs to all elements in the outline
func (m *IDManager) AssignIDsToOutline() {
	for partIdx := range m.outline.Parts {
		m.outline.Parts[partIdx].ID = m.GeneratePartID(partIdx + 1)

		for volIdx := range m.outline.Parts[partIdx].Volumes {
			m.outline.Parts[partIdx].Volumes[volIdx].ID = m.GenerateVolumeID(partIdx+1, volIdx+1)

			for chapIdx := range m.outline.Parts[partIdx].Volumes[volIdx].Chapters {
				m.outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx].ID =
					m.GenerateChapterID(partIdx+1, volIdx+1, chapIdx+1)
			}
		}
	}
}

// GetAllChapters returns all chapters in a flat list with their global index
func (m *IDManager) GetAllChapters() []*models.Chapter {
	var chapters []*models.Chapter
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			for chapIdx := range m.outline.Parts[partIdx].Volumes[volIdx].Chapters {
				chapters = append(chapters, &m.outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx])
			}
		}
	}
	return chapters
}

// GetAllVolumes returns all volumes in a flat list with their global index
func (m *IDManager) GetAllVolumes() []*models.Volume {
	var volumes []*models.Volume
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			volumes = append(volumes, &m.outline.Parts[partIdx].Volumes[volIdx])
		}
	}
	return volumes
}

// GetGlobalChapterNumber returns the global chapter number for a chapter ID
func (m *IDManager) GetGlobalChapterNumber(chapterID string) int {
	globalNum := 0
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			for chapIdx := range m.outline.Parts[partIdx].Volumes[volIdx].Chapters {
				globalNum++
				expectedID := m.GenerateChapterID(partIdx+1, volIdx+1, chapIdx+1)
				if m.outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx].ID == chapterID || expectedID == chapterID {
					return globalNum
				}
			}
		}
	}
	return -1
}

// GetGlobalVolumeNumber returns the global volume number for a volume ID
func (m *IDManager) GetGlobalVolumeNumber(volumeID string) int {
	globalNum := 0
	for partIdx := range m.outline.Parts {
		for volIdx := range m.outline.Parts[partIdx].Volumes {
			globalNum++
			expectedID := m.GenerateVolumeID(partIdx+1, volIdx+1)
			if m.outline.Parts[partIdx].Volumes[volIdx].ID == volumeID || expectedID == volumeID {
				return globalNum
			}
		}
	}
	return -1
}
