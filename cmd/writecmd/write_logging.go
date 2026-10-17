package writecmd

import (
	"os"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
)

func setupWriteRunLogging(projectRoot *store.Root, label string) {
	log := logger.Default()
	log.SetProjectRoot(projectRoot)
	if err := log.EnableFileLogging(); err != nil {
		logger.Warn("Failed to enable file logging: %v", err)
		return
	}

	cwd, _ := os.Getwd()
	logger.Info("Command: %s", strings.Join(os.Args, " "))
	logger.Info("Run label: %s", label)
	logger.Info("Working dir: %s", cwd)
	logger.Info("Project root: %s", projectRoot.Dir())
	logger.Info("Log file: %s", log.LogFilePath())
	logger.Info("Prompt logs: %s", projectRoot.LogFile("prompts"))
	logger.Info("Response logs: %s", projectRoot.LogFile("responses"))
}

func finalChapterPath(projectRoot *store.Root, chapter *models.Chapter) string {
	if chapter == nil {
		return projectRoot.ChaptersDir()
	}
	return projectRoot.ChapterFile(chapter.ID)
}
