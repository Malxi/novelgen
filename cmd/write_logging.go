package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/logger"
	"novelgen/internal/models"
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
	logger.Info("Prompt logs: %s", filepath.Join(projectRoot, "logs", "prompts"))
	logger.Info("Response logs: %s", filepath.Join(projectRoot, "logs", "responses"))
}

func finalChapterPath(projectRoot string, chapter *models.Chapter) string {
	if chapter == nil {
		return filepath.Join(projectRoot, "chapters")
	}
	return filepath.Join(projectRoot, "chapters", "chapter-"+chapter.ID+".md")
}
