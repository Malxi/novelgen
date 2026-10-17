package writecmd

import (
	"testing"

	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

func TestWriteTargetWordsExplicitDefaultBeatsProjectConfig(t *testing.T) {
	previous := writeWordsFlag
	writeWordsFlag = 2000
	t.Cleanup(func() { writeWordsFlag = previous })

	cmd := &cobra.Command{Use: "gen"}
	cmd.Flags().IntVar(&writeWordsFlag, "words", 2000, "")
	if err := cmd.Flags().Set("words", "2000"); err != nil {
		t.Fatalf("set words: %v", err)
	}
	config := &models.ProjectConfig{}
	config.ChapterConfig.TargetWordsPerChapter = 3000
	if got := writeTargetWords(cmd, config); got != 2000 {
		t.Fatalf("explicit --words 2000 was overridden by config: got %d", got)
	}

	cmd2 := &cobra.Command{Use: "gen"}
	cmd2.Flags().IntVar(&writeWordsFlag, "words", 2000, "")
	if got := writeTargetWords(cmd2, config); got != 3000 {
		t.Fatalf("default --words should defer to project config: got %d", got)
	}
}
