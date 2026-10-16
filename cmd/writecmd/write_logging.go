package writecmd

import (
	"os"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/store"
)

func setupWriteRunLogging(projectRoot, label string) {
	log := logger.Default()
	log.SetProjectDir(projectRoot)
	if err := log.EnableFileLogging(); err != nil {
		logger.Warn("Failed to enable file logging: %v", err)
		return
	}

	cwd, _ := os.Getwd()
	logger.Info("Command: %s", strings.Join(os.Args, " "))
	logger.Info("Run label: %s", label)
	logger.Info("Working dir: %s", cwd)
	logger.Info("Project root: %s", projectRoot)
	logger.Info("Log file: %s", log.LogFilePath())
	logger.Info("Prompt logs: %s", store.At(projectRoot).LogFile("prompts"))
	logger.Info("Response logs: %s", store.At(projectRoot).LogFile("responses"))
}

func finalChapterPath(projectRoot string, chapter *models.Chapter) string {
	if chapter == nil {
		return store.At(projectRoot).ChaptersDir()
	}
	return store.At(projectRoot).ChapterFile(chapter.ID)
}
