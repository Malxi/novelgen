package utils

import "strings"

// ExtractChapterNumber returns the chapter-number component of a chapter ID.
//
// Historical behavior note: the original helper split on "_" first and always
// returned that last segment, so IDs without an underscore (for example
// "P1-V1-C2") are returned unchanged rather than reduced to "2". Callers build
// file names like "chapter-<value>.md" from the result, so this preserves the
// exact on-disk naming that existing projects rely on.
func ExtractChapterNumber(chapterID string) string {
	parts := strings.Split(chapterID, "_")
	return parts[len(parts)-1]
}
