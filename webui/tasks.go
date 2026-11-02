package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func createTask(c *gin.Context) {
	var req struct {
		Type    string                 `json:"type"`
		Command string                 `json:"command"`
		Args    map[string]interface{} `json:"args"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	task := &Task{
		ID:        uuid.New().String(),
		Type:      req.Type,
		Status:    "pending",
		Progress:  0,
		Message:   describeTask(req.Command, req.Args) + " queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel

	tasksMu.Lock()
	tasks[task.ID] = task
	taskSnapshot := cloneTask(task)
	tasksMu.Unlock()

	// Execute task asynchronously
	go executeTask(ctx, task, req.Command, req.Args)

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: taskSnapshot})
}

// buildTaskArgs converts a JSON task request into a deterministic CLI argv.
// Map iteration order must never leak into the command line: the same request
// has to produce the same argv for logs, retries and allowlist matching.
func buildTaskArgs(command string, args map[string]interface{}) []string {
	cmdArgs := []string{command}
	subcommand, _ := args["subcommand"].(string)
	if subcommand != "" {
		cmdArgs = append(cmdArgs, subcommand)
	}
	if positional, ok := args["_positional"].(string); ok && positional != "" {
		cmdArgs = append(cmdArgs, positional)
	}

	// Web tasks are non-interactive. compose regen prompts in the terminal when
	// --prompt is omitted, so provide a default to avoid hanging background jobs.
	if command == "compose" && subcommand == "regen" {
		if prompt, ok := args["prompt"].(string); !ok || strings.TrimSpace(prompt) == "" {
			args["prompt"] = "Regenerate this outline element while preserving continuity and improving motivation, conflict, beats, payoff, and hook."
		}
	}
	// Auto-add --force for compose gen to allow regeneration with backup.
	if command == "compose" && subcommand == "gen" {
		cmdArgs = append(cmdArgs, "--force")
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "project_dir" || key == "subcommand" || key == "_positional" {
			continue
		}
		switch value := args[key].(type) {
		case string:
			if value != "" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), value)
			}
		case int, int64, float64:
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), fmt.Sprintf("%v", value))
		case bool:
			if value {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
			}
		}
	}
	return cmdArgs
}

func executeTask(ctx context.Context, task *Task, command string, args map[string]interface{}) {
	taskSem <- struct{}{}
	defer func() { <-taskSem }()

	defer task.cancel()

	broadcastTaskUpdate(updateTask(task, func(t *Task) {
		t.Status = "running"
		t.Progress = 5
		t.Message = "Running " + describeTask(command, args)
	}))

	// Find project directory first (needed for backup)
	projectPath := ""
	if projectDir, ok := args["project_dir"].(string); ok && projectDir != "" {
		resolved, err := resolveProjectDir(projectDir)
		if err != nil {
			broadcastTaskUpdate(updateTask(task, func(t *Task) {
				t.Status = "failed"
				t.Message = "Invalid project directory: " + err.Error()
			}))
			return
		}
		projectPath = resolved
	}

	if projectPath == "" {
		// Try to find project in books directory
		projectPath = firstProjectInBooks()
	}

	// Auto-backup outline before compose gen/regen
	if command == "compose" {
		if subcommand, ok := args["subcommand"].(string); ok {
			if subcommand == "gen" || subcommand == "regen" {
				if projectPath != "" {
					broadcastTaskUpdate(updateTask(task, func(t *Task) {
						t.Message = "Backing up existing outline..."
					}))

					if backupName, err := backupOutline(projectPath); err != nil {
						broadcastTaskUpdate(updateTask(task, func(t *Task) {
							t.Message = "Warning: Failed to backup outline: " + err.Error()
						}))
					} else if backupName != "" {
						broadcastTaskUpdate(updateTask(task, func(t *Task) {
							t.Message = "Outline backed up: " + backupName
						}))
					}
				}
			}
		}
	}

	cmdArgs := buildTaskArgs(command, args)

	// Collect debug info
	var debugInfo strings.Builder
	debugInfo.WriteString("=== DEBUG INFO ===\n")

	// Re-find project directory (it was already found earlier for backup)
	if projectPath == "" {
		if projectDir, ok := args["project_dir"].(string); ok && projectDir != "" {
			if resolved, err := resolveProjectDir(projectDir); err == nil {
				projectPath = resolved
			}
		}
	}

	if projectPath == "" {
		// Try to find project in books directory
		projectPath = firstProjectInBooks()
	}

	if projectPath != "" {
		debugInfo.WriteString(fmt.Sprintf("Project path: %s\n", projectPath))
	}

	// Build command with working directory
	cmd := exec.CommandContext(ctx, getNovelGenPath(), cmdArgs...)

	// Set working directory if project path is provided
	if projectPath != "" {
		// Convert to absolute path if relative
		absPath, err := filepath.Abs(projectPath)
		if err == nil {
			projectPath = absPath
		}
		cmd.Dir = projectPath
		debugInfo.WriteString(fmt.Sprintf("Final project path (absolute): %s\n", projectPath))
		debugInfo.WriteString(fmt.Sprintf("Command working directory: %s\n", cmd.Dir))

		// Check if story_setup.json exists
		setupPath := filepath.Join(projectPath, "story", "setup", "story_setup.json")
		if _, err := statProjectPath(projectPath, "story/setup/story_setup.json"); err != nil {
			debugInfo.WriteString(fmt.Sprintf("WARNING: story_setup.json not found at: %s\n", setupPath))
		} else {
			debugInfo.WriteString(fmt.Sprintf("story_setup.json found at: %s\n", setupPath))
		}

		// Check if novel.json exists
		novelPath := filepath.Join(projectPath, "novel.json")
		if _, err := statProjectPath(projectPath, "novel.json"); err != nil {
			debugInfo.WriteString(fmt.Sprintf("WARNING: novel.json not found at: %s\n", novelPath))
		} else {
			debugInfo.WriteString(fmt.Sprintf("novel.json found at: %s\n", novelPath))
		}
	}

	debugInfo.WriteString(fmt.Sprintf("Command: novelgen %s\n", strings.Join(cmdArgs, " ")))
	debugInfo.WriteString("==================\n\n")

	// Capture output
	output, err := cmd.CombinedOutput()

	if err != nil {
		broadcastTaskUpdate(updateTask(task, func(t *Task) {
			t.Output = debugInfo.String() + string(output)
			t.Status = "failed"
			t.Error = err.Error()
			t.Message = "Task failed"
		}))
	} else {
		broadcastTaskUpdate(updateTask(task, func(t *Task) {
			t.Output = debugInfo.String() + string(output)
			t.Status = "completed"
			t.Progress = 100
			t.Message = "Task completed successfully"
		}))
	}
}

func describeTask(command string, args map[string]interface{}) string {
	parts := []string{command}
	if subcommand, ok := args["subcommand"].(string); ok && subcommand != "" {
		parts = append(parts, subcommand)
	}
	if positional, ok := args["_positional"].(string); ok && positional != "" {
		parts = append(parts, positional)
	}
	if prompt, ok := args["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		trimmed := strings.TrimSpace(prompt)
		if len([]rune(trimmed)) > 28 {
			runes := []rune(trimmed)
			trimmed = string(runes[:28]) + "..."
		}
		parts = append(parts, fmt.Sprintf("(%s)", trimmed))
	}
	return strings.Join(parts, " ")
}

func listTasks(c *gin.Context) {
	tasksMu.RLock()
	taskList := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, cloneTask(task))
	}
	tasksMu.RUnlock()
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: taskList})
}

func getTask(c *gin.Context) {
	id := c.Param("id")
	tasksMu.RLock()
	task, exists := tasks[id]
	taskSnapshot := cloneTask(task)
	tasksMu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Task not found"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: taskSnapshot})
}

func deleteTask(c *gin.Context) {
	id := c.Param("id")
	tasksMu.Lock()
	if task, ok := tasks[id]; ok && task.cancel != nil {
		task.cancel()
	}
	delete(tasks, id)
	tasksMu.Unlock()
	c.JSON(http.StatusOK, APIResponse{Success: true})
}
