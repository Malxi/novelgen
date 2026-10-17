package toolplane

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/refresh"
)

type toolRefreshResult = refresh.Result

var (
	SuppressToolRefreshLogs        = refresh.SuppressLogs
	RefreshToolChapterDSL          = refresh.RefreshChapterDSL
	MakeToolRefreshChapterDSLCheck = refresh.MakeChapterDSLCheck
	firstExistingPath              = refresh.FirstExistingPath
)

func RunToolRefresh(cmd *cobra.Command, args []string) error {
	target := NormalizeKey(args[0])
	switch target {
	case "chapter-dsl", "chapters-dsl", "chapter-rpg-dsl", "rpg-chapters":
		return runToolRefreshChapterDSL(cmd)
	default:
		return fmt.Errorf("unsupported refresh target %q", target)
	}
}

func runToolRefreshChapterDSL(cmd *cobra.Command) error {
	restoreLogger := SuppressToolRefreshLogs()
	defer restoreLogger()

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root
	id := strings.TrimSpace(ToolRefreshFlags.ID)
	if id == "" {
		return fmt.Errorf("--id is required for chapter-dsl refresh")
	}
	result, err := RefreshToolChapterDSL(cmd.Context(), root, id, ToolRefreshFlags.BatchSize)
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}
