// Code in this file is part of the compose command implementation.
// Split from compose.go; keep one command/workflow concern per file.

package composecmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"novelgen/internal/agents"
	"novelgen/internal/app"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
	"novelgen/internal/toolplane"
)

// composeProject resolves the project a compose command operates on. Compose used
// to require the working directory to be the project root, so `compose gen`
// failed from a subdirectory while `write` worked there; every compose path now
// comes from the store.
func composeProject(cmd *cobra.Command) (*project.Book, error) {
	book, err := project.ForCommand(cmd)
	if err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return nil, fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first")
	}
	return book, nil
}

func openComposeSession(cmd *cobra.Command) (*app.Session, error) {
	session, err := app.Open(cmd)
	if err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return nil, fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init' first: %w", err)
	}
	logger.Info("Loaded project config: %s", session.Config.Name)
	return session, nil
}

func loadComposeProjectState(cmd *cobra.Command) (*project.Book, *models.ProjectConfig, *models.StorySetup, *models.Outline, error) {
	session, err := app.Open(cmd)
	if err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return nil, nil, nil, nil, err
	}
	logger.Info("Loaded project config: %s", session.Config.Name)

	setup, err := session.RequireSetup()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	outline, err := session.RequireOutline()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return session.Book(), session.Config, setup, outline, nil
}

func newComposeAgentForProject(root *store.Root, projectConfig *models.ProjectConfig) (*agents.ComposeAgent, error) {
	if root == nil || projectConfig == nil {
		return nil, fmt.Errorf("project root and config are required")
	}
	return (&app.Session{Root: root, Config: projectConfig}).NewComposeAgent()
}

// saveOutlineMutation commits the canonical outline through the same mutation
// surface used by the agent tool plane. Compose has already run its own repair
// loop, so this is a host commit rather than a second quality gate.
func saveOutlineMutation(ctx context.Context, book *project.Book, outline *models.Outline) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := toolplane.NewMutationService(book.Root).ReplaceOutline(ctx, outline, toolplane.MutationOptions{
		Validate: false,
		Source:   "compose-command",
	})
	return err
}

// saveOutlineMutationRaw commits a scoped Agent SDK outline result without
// whole-outline normalization or derived markdown. The caller has deliberately
// preserved unselected volumes and updates markdown separately.
func saveOutlineMutationRaw(ctx context.Context, book *project.Book, outline *models.Outline) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := toolplane.NewMutationService(book.Root).ReplaceOutlineRaw(ctx, outline, toolplane.MutationOptions{
		Validate: false,
		Source:   "compose-command",
	})
	return err
}

// saveSetupMutation commits setup changes made by the compose pipeline (for
// example a setup/outline cross patch) through the same service.
func saveSetupMutation(ctx context.Context, book *project.Book, setup *models.StorySetup) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := toolplane.NewMutationService(book.Root).ReplaceSetup(ctx, setup, toolplane.MutationOptions{
		Validate: false,
		Source:   "compose-command",
	})
	return err
}

// backupOutlineFiles copies outline.json (and outline.md when present) into
// story/compose/backups before an improvement pass overwrites them.
func backupOutlineFiles(book *project.Book) error {
	backupPath, err := book.BackupFile(store.ScopeCompose, book.OutlineJSON(), "outline")
	if err != nil {
		return fmt.Errorf("failed to backup outline: %w", err)
	}
	if backupPath == "" {
		return fmt.Errorf("failed to read outline for backup: %w", os.ErrNotExist)
	}
	if _, err := book.BackupFile(store.ScopeCompose, book.OutlineMarkdown(), "outline"); err != nil {
		return fmt.Errorf("failed to backup outline markdown: %w", err)
	}

	logger.Info("Outline backed up to: %s", backupPath)
	return nil
}
