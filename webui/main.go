package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"novelgen/internal/models"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	clients   = make(map[string]*websocket.Conn)
	clientsMu sync.RWMutex
	writesMu  sync.Mutex
)

// getNovelGenPath returns the path to novelgen executable.
func getNovelGenPath() string {
	exePath := "novelgen.exe"
	if runtime.GOOS != "windows" {
		exePath = "novelgen"
	}

	candidates := []string{}
	if execPath, err := os.Executable(); err == nil {
		webuiDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(webuiDir, exePath),
			filepath.Join(webuiDir, "..", "bin", exePath),
			filepath.Join(webuiDir, "..", exePath),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, exePath),
			filepath.Join(cwd, "bin", exePath),
			filepath.Join(cwd, "webui", exePath),
			filepath.Join(cwd, "..", "bin", exePath),
			filepath.Join(cwd, "..", exePath),
		)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
	}

	// Fallback to PATH
	return "novelgen"
}

type Task struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectInfo struct {
	Name          string            `json:"name"`
	Path          string            `json:"path"`
	Language      string            `json:"language"`
	Structure     map[string]int    `json:"structure"`
	ChapterConfig map[string]int    `json:"chapter_config"`
	LLM           map[string]string `json:"llm"`
	Exists        bool              `json:"exists"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type AICallSummary struct {
	ID               string    `json:"id"`
	Agent            string    `json:"agent"`
	Command          string    `json:"command,omitempty"`
	Model            string    `json:"model,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	HasInput         bool      `json:"has_input"`
	HasOutput        bool      `json:"has_output"`
	InputChars       int       `json:"input_chars"`
	OutputChars      int       `json:"output_chars"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	TotalTokens      int       `json:"total_tokens,omitempty"`
	Legacy           bool      `json:"legacy"`
}

type AICallDetail struct {
	AICallSummary
	Skills       []string `json:"skills,omitempty"`
	SystemPrompt string   `json:"system_prompt"`
	UserPrompt   string   `json:"user_prompt"`
	Response     string   `json:"response"`
	PromptPath   string   `json:"prompt_path,omitempty"`
	ResponsePath string   `json:"response_path,omitempty"`
}

type FileVersion struct {
	Filename  string `json:"filename"`
	CreatedAt string `json:"created_at"`
	Size      int64  `json:"size"`
}

type aiCallLogFile struct {
	ID           string    `json:"id"`
	Agent        string    `json:"agent"`
	Command      string    `json:"command"`
	Skills       []string  `json:"skills"`
	Model        string    `json:"model"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	SystemPrompt string    `json:"system_prompt"`
	UserPrompt   string    `json:"user_prompt"`
	Response     string    `json:"response"`
	Usage        struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

var (
	tasks   = make(map[string]*Task)
	tasksMu sync.RWMutex
)

func main() {
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	r.Use(cors.New(config))

	// API routes
	api := r.Group("/api")
	{
		// Project management
		api.GET("/projects", listProjects)
		api.POST("/projects", createProject)
		api.GET("/projects/current", getCurrentProject)
		api.GET("/projects/:path", getProject)

		// Workflow commands
		api.POST("/tasks", createTask)
		api.GET("/tasks", listTasks)
		api.GET("/tasks/:id", getTask)
		api.DELETE("/tasks/:id", deleteTask)

		// Content management
		api.GET("/content/outline", getOutline)
		api.GET("/content/outline/versions", listOutlineVersions)
		api.POST("/content/outline/restore", restoreOutlineVersion)
		api.GET("/content/setup", getStorySetup)
		api.GET("/content/characters", getCharacters)
		api.GET("/content/locations", getLocations)
		api.GET("/content/items", getItems)
		api.GET("/content/chapters", getChapters)
		api.GET("/content/chapters/:id", getChapter)
		api.GET("/content/drafts", getDrafts)
		api.GET("/content/drafts/:id", getDraft)
		api.GET("/content/recaps", getRecaps)
		api.GET("/content/reviews", getReviews)
		api.GET("/ai-calls", listAICalls)
		api.GET("/ai-calls/:id", getAICall)

		// RPG data management
		api.GET("/rpg/data", getRPGData)
		api.GET("/rpg/characters", getRPGCharacters)
		api.GET("/rpg/skills", getRPGSkills)
		api.GET("/rpg/items", getRPGItems)
		api.GET("/rpg/classes", getRPGClasses)
		api.GET("/rpg/events", getRPGEvents)
		api.GET("/rpg/quests", getRPGQuests)

		// Simulation reports
		api.GET("/simulations", listSimulationReports)
		api.GET("/simulations/:id", getSimulationReport)

		// File operations
		api.GET("/files/*path", getFile)
		api.POST("/files/*path", saveFile)
	}

	// WebSocket for real-time updates
	r.GET("/ws", handleWebSocket)

	// Static files (frontend)
	r.Use(static.Serve("/", static.LocalFile("./frontend/dist", false)))
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	// Open browser
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://localhost:%s", *port))
	}()

	fmt.Printf("🚀 NovelGen Web UI is running at http://localhost:%s\n", *port)
	log.Fatal(r.Run(":" + *port))
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	clientID := uuid.New().String()
	clientsMu.Lock()
	clients[clientID] = conn
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clients, clientID)
		clientsMu.Unlock()
	}()

	// Send initial connection success
	conn.WriteJSON(map[string]string{
		"type": "connected",
		"id":   clientID,
	})

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastTaskUpdate(task *Task) {
	snapshot := cloneTask(task)
	clientsMu.RLock()
	connections := make([]*websocket.Conn, 0, len(clients))
	for _, conn := range clients {
		connections = append(connections, conn)
	}
	clientsMu.RUnlock()

	writesMu.Lock()
	defer writesMu.Unlock()
	for _, conn := range connections {
		_ = conn.WriteJSON(map[string]interface{}{
			"type": "task_update",
			"data": snapshot,
		})
	}
}

func cloneTask(task *Task) *Task {
	if task == nil {
		return nil
	}
	copied := *task
	return &copied
}

func updateTask(task *Task, update func(*Task)) *Task {
	tasksMu.Lock()
	update(task)
	task.UpdatedAt = time.Now()
	snapshot := cloneTask(task)
	tasksMu.Unlock()
	return snapshot
}

func listProjects(c *gin.Context) {
	booksDir := "../books"
	projects := []ProjectInfo{}

	entries, err := os.ReadDir(booksDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: projects})
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			projectPath := filepath.Join(booksDir, entry.Name())
			info := loadProjectInfo(projectPath)
			if info.Exists {
				projects = append(projects, *info)
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: projects})
}

func loadProjectInfo(path string) *ProjectInfo {
	info := &ProjectInfo{
		Path:   path,
		Exists: false,
	}

	novelJSONPath := filepath.Join(path, "novel.json")
	data, err := os.ReadFile(novelJSONPath)
	if err != nil {
		return info
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return info
	}

	info.Exists = true
	if name, ok := config["name"].(string); ok {
		info.Name = name
	}
	if lang, ok := config["language"].(string); ok {
		info.Language = lang
	}

	if structure, ok := config["structure"].(map[string]interface{}); ok {
		info.Structure = make(map[string]int)
		for k, v := range structure {
			if val, ok := v.(float64); ok {
				info.Structure[k] = int(val)
			}
		}
	}

	if chapterConfig, ok := config["chapter_config"].(map[string]interface{}); ok {
		info.ChapterConfig = make(map[string]int)
		for k, v := range chapterConfig {
			if val, ok := v.(float64); ok {
				info.ChapterConfig[k] = int(val)
			}
		}
	}

	if llm, ok := config["llm"].(map[string]interface{}); ok {
		info.LLM = make(map[string]string)
		for k, v := range llm {
			if val, ok := v.(string); ok {
				info.LLM[k] = val
			}
		}
	}

	return info
}

func createProject(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Genre    string `json:"genre"`
		Chapters int    `json:"chapters"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Language string `json:"language"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	// Create project using novelgen CLI
	args := []string{"init", req.Name}
	if req.Chapters > 0 {
		args = append(args, "--chapter", fmt.Sprintf("%d", req.Chapters))
	}
	if req.Genre != "" {
		args = append(args, "--genre", req.Genre)
	}
	if req.Provider != "" {
		args = append(args, "--provider", req.Provider)
	}
	if req.Model != "" {
		args = append(args, "--mode", req.Model)
	}
	if req.Language != "" {
		args = append(args, "--language", req.Language)
	}

	projectPath := filepath.Join("../books", req.Name)
	os.MkdirAll(projectPath, 0755)
	os.Chdir(projectPath)

	cmd := exec.Command(getNovelGenPath(), args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create project: %s\n%s", err.Error(), string(output)),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"output": string(output)},
	})
}

func getCurrentProject(c *gin.Context) {
	// Try to find novel.json in current directory or parent
	cwd, _ := os.Getwd()
	info := loadProjectInfo(cwd)
	if info.Exists {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: info})
		return
	}

	// Try parent directory
	parent := filepath.Dir(cwd)
	info = loadProjectInfo(parent)
	if info.Exists {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: info})
		return
	}

	c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "No project found"})
}

func getProject(c *gin.Context) {
	path := c.Param("path")
	projectPath := filepath.Join("../books", path)
	info := loadProjectInfo(projectPath)
	if !info.Exists {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Project not found"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: info})
}

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

	tasksMu.Lock()
	tasks[task.ID] = task
	taskSnapshot := cloneTask(task)
	tasksMu.Unlock()

	// Execute task asynchronously
	go executeTask(task, req.Command, req.Args)

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: taskSnapshot})
}

func executeTask(task *Task, command string, args map[string]interface{}) {
	broadcastTaskUpdate(updateTask(task, func(t *Task) {
		t.Status = "running"
		t.Progress = 5
		t.Message = "Running " + describeTask(command, args)
	}))

	// Find project directory first (needed for backup)
	projectPath := ""
	if projectDir, ok := args["project_dir"].(string); ok && projectDir != "" {
		projectPath = projectDir
	}

	if projectPath == "" {
		// Try to find project in books directory
		booksDir := "../books"
		entries, _ := os.ReadDir(booksDir)
		for _, entry := range entries {
			if entry.IsDir() {
				possiblePath := filepath.Join(booksDir, entry.Name())
				if _, err := os.Stat(filepath.Join(possiblePath, "novel.json")); err == nil {
					projectPath = possiblePath
					break
				}
			}
		}
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

	// Build command arguments
	cmdArgs := []string{command}

	// Handle subcommand for compose command
	if subcommand, ok := args["subcommand"].(string); ok && subcommand != "" {
		cmdArgs = append(cmdArgs, subcommand)
	}

	// Handle positional arguments (for commands like 'rpg simulate <chapter_id>')
	if positional, ok := args["_positional"].(string); ok && positional != "" {
		cmdArgs = append(cmdArgs, positional)
	}

	// Auto-add --force for compose gen to allow regeneration with backup
	if command == "compose" {
		if subcommand, ok := args["subcommand"].(string); ok && subcommand == "gen" {
			cmdArgs = append(cmdArgs, "--force")
		}
	}

	// Add args based on command type
	for key, value := range args {
		// Skip project_dir as it's used for directory switching, not command arg
		// Skip subcommand as it's already handled above
		// Skip _positional as it's already handled above
		if key == "project_dir" || key == "subcommand" || key == "_positional" {
			continue
		}
		switch v := value.(type) {
		case string:
			if v != "" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), v)
			}
		case int, int64, float64:
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), fmt.Sprintf("%v", v))
		case bool:
			if v {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
			}
		}
	}

	// Collect debug info
	var debugInfo strings.Builder
	debugInfo.WriteString("=== DEBUG INFO ===\n")

	// Re-find project directory (it was already found earlier for backup)
	if projectPath == "" {
		if projectDir, ok := args["project_dir"].(string); ok && projectDir != "" {
			projectPath = projectDir
		}
	}

	if projectPath == "" {
		// Try to find project in books directory
		booksDir := "../books"
		entries, _ := os.ReadDir(booksDir)
		for _, entry := range entries {
			if entry.IsDir() {
				possiblePath := filepath.Join(booksDir, entry.Name())
				if _, err := os.Stat(filepath.Join(possiblePath, "novel.json")); err == nil {
					projectPath = possiblePath
					break
				}
			}
		}
	}

	if projectPath != "" {
		debugInfo.WriteString(fmt.Sprintf("Project path: %s\n", projectPath))
	}

	// Build command with working directory
	cmd := exec.Command(getNovelGenPath(), cmdArgs...)

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
		if _, err := os.Stat(setupPath); err != nil {
			debugInfo.WriteString(fmt.Sprintf("WARNING: story_setup.json not found at: %s\n", setupPath))
		} else {
			debugInfo.WriteString(fmt.Sprintf("story_setup.json found at: %s\n", setupPath))
		}

		// Check if novel.json exists
		novelPath := filepath.Join(projectPath, "novel.json")
		if _, err := os.Stat(novelPath); err != nil {
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
	delete(tasks, id)
	tasksMu.Unlock()
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

func getOutline(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	outlinePath := filepath.Join(projectPath, "story", "compose", "outline.json")
	data, err := os.ReadFile(outlinePath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Outline not found"})
		return
	}

	var outline interface{}
	if err := json.Unmarshal(data, &outline); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: outline})
}

// backupOutline creates a backup of the current outline with timestamp
func backupOutline(projectPath string) (string, error) {
	return backupFile(projectPath, "story/compose/outline.json")
}

func backupStorySetup(projectPath string) (string, error) {
	return backupFile(projectPath, "story/setup/story_setup.json")
}

func backupFile(projectPath, contentPath string) (string, error) {
	cleanPath, err := cleanContentPath(contentPath)
	if err != nil {
		return "", err
	}

	sourcePath := filepath.Join(projectPath, cleanPath)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", nil
	}

	backupsDir := filepath.Join(projectPath, filepath.Dir(cleanPath), "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %w", err)
	}

	ext := filepath.Ext(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), ext)
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("%s_%s%s", base, timestamp, ext)
	backupPath := filepath.Join(backupsDir, backupFilename)
	for suffix := 2; ; suffix++ {
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			break
		}
		backupFilename = fmt.Sprintf("%s_%s_%d%s", base, timestamp, suffix, ext)
		backupPath = filepath.Join(backupsDir, backupFilename)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupFilename, nil
}

func cleanContentPath(path string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimPrefix(path, "/"))
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("invalid path")
	}
	return cleanPath, nil
}

func projectPathFromQuery(c *gin.Context) string {
	projectPath := c.Query("project")
	if projectPath == "" {
		return "."
	}
	return projectPath
}

func listOutlineVersions(c *gin.Context) {
	versions, err := fileVersions(projectPathFromQuery(c), "story/compose/outline.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: versions})
}

func restoreOutlineVersion(c *gin.Context) {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := restoreVersion(projectPathFromQuery(c), "story/compose/outline.json", req.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"message": "Outline restored successfully"}})
}

func listFileVersions(c *gin.Context) {
	contentPath := c.Query("path")
	versions, err := fileVersions(projectPathFromQuery(c), contentPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: versions})
}

func restoreFileVersion(c *gin.Context) {
	var req struct {
		Path     string `json:"path"`
		Filename string `json:"filename"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := restoreVersion(projectPathFromQuery(c), req.Path, req.Filename); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"message": "Version restored successfully"}})
}

func fileVersions(projectPath, contentPath string) ([]FileVersion, error) {
	cleanPath, err := cleanContentPath(contentPath)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), ext)
	backupsDir := filepath.Join(projectPath, filepath.Dir(cleanPath), "backups")
	if _, err := os.Stat(backupsDir); os.IsNotExist(err) {
		return []FileVersion{}, nil
	}

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return nil, err
	}

	versions := []FileVersion{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), base+"_") || !strings.HasSuffix(entry.Name(), ext) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		versions = append(versions, FileVersion{
			Filename:  entry.Name(),
			CreatedAt: versionCreatedAt(base, ext, entry.Name(), info.ModTime()),
			Size:      info.Size(),
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Filename > versions[j].Filename
	})
	return versions, nil
}

func restoreVersion(projectPath, contentPath, filename string) error {
	cleanPath, err := cleanContentPath(contentPath)
	if err != nil {
		return err
	}
	if !safeLogName(filename) {
		return fmt.Errorf("invalid filename")
	}

	ext := filepath.Ext(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), ext)
	if !strings.HasPrefix(filename, base+"_") || !strings.HasSuffix(filename, ext) {
		return fmt.Errorf("backup does not match target file")
	}

	backupPath := filepath.Join(projectPath, filepath.Dir(cleanPath), "backups", filename)
	targetPath := filepath.Join(projectPath, cleanPath)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found")
	}

	if _, err := backupFile(projectPath, cleanPath); err != nil {
		return fmt.Errorf("failed to backup current file: %w", err)
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return err
	}
	return nil
}

func versionCreatedAt(base, ext, filename string, fallback time.Time) string {
	timestampStr := strings.TrimSuffix(strings.TrimPrefix(filename, base+"_"), ext)
	if len(timestampStr) > len("20060102_150405") {
		timestampStr = timestampStr[:len("20060102_150405")]
	}
	if t, err := time.Parse("20060102_150405", timestampStr); err == nil {
		return t.Format("2006-01-02 15:04:05")
	}
	return fallback.Format("2006-01-02 15:04:05")
}

func getStorySetup(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	setupPath := filepath.Join(projectPath, "story", "setup", "story_setup.json")
	data, err := os.ReadFile(setupPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Story setup not found"})
		return
	}

	var setup interface{}
	if err := json.Unmarshal(data, &setup); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: setup})
}

func getCharacters(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	charactersPath := filepath.Join(projectPath, "story", "craft", "characters.json")
	data, err := os.ReadFile(charactersPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Characters not found"})
		return
	}

	var charactersMap map[string]interface{}
	if err := json.Unmarshal(data, &charactersMap); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	// Convert map to array
	characters := make([]interface{}, 0, len(charactersMap))
	for _, value := range charactersMap {
		characters = append(characters, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"characters": characters}})
}

func getLocations(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	locationsPath := filepath.Join(projectPath, "story", "craft", "locations.json")
	data, err := os.ReadFile(locationsPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Locations not found"})
		return
	}

	var locationsMap map[string]interface{}
	if err := json.Unmarshal(data, &locationsMap); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	// Convert map to array
	locations := make([]interface{}, 0, len(locationsMap))
	for _, value := range locationsMap {
		locations = append(locations, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"locations": locations}})
}

func getItems(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	itemsPath := filepath.Join(projectPath, "story", "craft", "items.json")
	data, err := os.ReadFile(itemsPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Items not found"})
		return
	}

	var itemsMap map[string]interface{}
	if err := json.Unmarshal(data, &itemsMap); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	// Convert map to array
	items := make([]interface{}, 0, len(itemsMap))
	for _, value := range itemsMap {
		items = append(items, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"items": items}})
}

func getChapters(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	chaptersDir := filepath.Join(projectPath, "chapters")
	entries, err := os.ReadDir(chaptersDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []map[string]string{}})
		return
	}

	chapters := []map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			chapters = append(chapters, map[string]string{
				"id":   strings.TrimSuffix(entry.Name(), ".md"),
				"name": entry.Name(),
			})
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: chapters})
}

func getChapter(c *gin.Context) {
	id := c.Param("id")
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	chapterPath := filepath.Join(projectPath, "chapters", id+".md")
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Chapter not found"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]string{
			"id":      id,
			"content": string(data),
		},
	})
}

func getDrafts(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	draftsDir := filepath.Join(projectPath, "drafts")
	entries, err := os.ReadDir(draftsDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []map[string]string{}})
		return
	}

	drafts := []map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			drafts = append(drafts, map[string]string{
				"id":   strings.TrimSuffix(entry.Name(), ".md"),
				"name": entry.Name(),
			})
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: drafts})
}

func getDraft(c *gin.Context) {
	id := c.Param("id")
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	draftPath := filepath.Join(projectPath, "drafts", id+".md")
	data, err := os.ReadFile(draftPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Draft not found"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]string{
			"id":      id,
			"content": string(data),
		},
	})
}

func getRecaps(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	recapsDir := filepath.Join(projectPath, "story", "recaps")
	entries, err := os.ReadDir(recapsDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []map[string]string{}})
		return
	}

	recaps := []map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			recaps = append(recaps, map[string]string{
				"id":   strings.TrimSuffix(entry.Name(), ".json"),
				"name": entry.Name(),
			})
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: recaps})
}

func getReviews(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	reviewsDir := filepath.Join(projectPath, "story", "reviews")
	entries, err := os.ReadDir(reviewsDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: []map[string]string{}})
		return
	}

	reviews := []map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			reviews = append(reviews, map[string]string{
				"id":   strings.TrimSuffix(entry.Name(), ".json"),
				"name": entry.Name(),
			})
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: reviews})
}

var aiLogFilenamePattern = regexp.MustCompile(`^(.+)_(\d{8}_\d{6})(?:_\d+)?\.(md|json)$`)

type legacyAILog struct {
	Name  string
	Path  string
	Agent string
	Time  time.Time
	Size  int64
}

func listAICalls(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	calls := make([]AICallSummary, 0)
	structured, err := listStructuredAICalls(projectPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}
	calls = append(calls, structured...)
	calls = append(calls, listLegacyAICalls(projectPath)...)

	sort.Slice(calls, func(i, j int) bool {
		return calls[i].StartedAt.After(calls[j].StartedAt)
	})

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: calls})
}

func getAICall(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	id := c.Param("id")
	if strings.HasPrefix(id, "structured:") {
		detail, err := readStructuredAICall(projectPath, strings.TrimPrefix(id, "structured:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	if strings.HasPrefix(id, "legacy-response:") {
		detail, err := readLegacyResponseOnlyAICall(projectPath, strings.TrimPrefix(id, "legacy-response:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	if strings.HasPrefix(id, "legacy:") {
		detail, err := readLegacyAICall(projectPath, strings.TrimPrefix(id, "legacy:"))
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: detail})
		return
	}

	c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid AI call id"})
}

func listStructuredAICalls(projectPath string) ([]AICallSummary, error) {
	dir := filepath.Join(projectPath, "logs", "ai_calls")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	calls := make([]AICallSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		detail, err := readStructuredAICall(projectPath, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		calls = append(calls, detail.AICallSummary)
	}
	return calls, nil
}

func readStructuredAICall(projectPath, id string) (*AICallDetail, error) {
	if !safeLogName(id) {
		return nil, fmt.Errorf("invalid AI call id")
	}
	path := filepath.Join(projectPath, "logs", "ai_calls", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry aiCallLogFile
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if entry.ID == "" {
		entry.ID = id
	}

	inputChars := len(entry.SystemPrompt) + len(entry.UserPrompt)
	detail := &AICallDetail{
		AICallSummary: AICallSummary{
			ID:               "structured:" + id,
			Agent:            entry.Agent,
			Command:          entry.Command,
			Model:            entry.Model,
			StartedAt:        entry.StartedAt,
			HasInput:         strings.TrimSpace(entry.SystemPrompt) != "" || strings.TrimSpace(entry.UserPrompt) != "",
			HasOutput:        strings.TrimSpace(entry.Response) != "",
			InputChars:       inputChars,
			OutputChars:      len(entry.Response),
			PromptTokens:     entry.Usage.PromptTokens,
			CompletionTokens: entry.Usage.CompletionTokens,
			TotalTokens:      entry.Usage.TotalTokens,
			Legacy:           false,
		},
		Skills:       entry.Skills,
		SystemPrompt: entry.SystemPrompt,
		UserPrompt:   entry.UserPrompt,
		Response:     entry.Response,
	}
	if detail.StartedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			detail.StartedAt = info.ModTime()
		}
	}
	return detail, nil
}

func listLegacyAICalls(projectPath string) []AICallSummary {
	prompts := readLegacyAILogs(filepath.Join(projectPath, "logs", "prompts"), ".md")
	responses := readLegacyAILogs(filepath.Join(projectPath, "logs", "responses"), ".md")
	pairs := pairLegacyAILogs(prompts, responses)

	calls := make([]AICallSummary, 0, len(prompts)+len(responses))
	for _, prompt := range prompts {
		response := pairs[prompt.Name]
		summary := AICallSummary{
			ID:          "legacy:" + prompt.Name,
			Agent:       prompt.Agent,
			StartedAt:   prompt.Time,
			HasInput:    true,
			HasOutput:   response != nil,
			InputChars:  int(prompt.Size),
			OutputChars: 0,
			Legacy:      true,
		}
		if response != nil {
			summary.OutputChars = int(response.Size)
		}
		calls = append(calls, summary)
	}

	matchedResponses := map[string]bool{}
	for _, response := range pairs {
		if response != nil {
			matchedResponses[response.Name] = true
		}
	}
	for _, response := range responses {
		if matchedResponses[response.Name] {
			continue
		}
		calls = append(calls, AICallSummary{
			ID:          "legacy-response:" + response.Name,
			Agent:       response.Agent,
			StartedAt:   response.Time,
			HasInput:    false,
			HasOutput:   true,
			OutputChars: int(response.Size),
			Legacy:      true,
		})
	}

	return calls
}

func readLegacyAICall(projectPath, promptName string) (*AICallDetail, error) {
	if !safeLogName(promptName) || !strings.HasSuffix(promptName, ".md") {
		return nil, fmt.Errorf("invalid prompt log name")
	}
	promptPath := filepath.Join(projectPath, "logs", "prompts", promptName)
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}

	promptLog, err := legacyAILogFromPath(promptPath)
	if err != nil {
		return nil, err
	}
	responses := readLegacyAILogs(filepath.Join(projectPath, "logs", "responses"), ".md")
	pairs := pairLegacyAILogs([]legacyAILog{promptLog}, responses)
	response := pairs[promptLog.Name]

	systemPrompt, userPrompt := splitPromptLog(string(promptData))
	detail := &AICallDetail{
		AICallSummary: AICallSummary{
			ID:          "legacy:" + promptLog.Name,
			Agent:       promptLog.Agent,
			StartedAt:   promptLog.Time,
			HasInput:    true,
			HasOutput:   response != nil,
			InputChars:  len(systemPrompt) + len(userPrompt),
			OutputChars: 0,
			Legacy:      true,
		},
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		PromptPath:   promptPath,
	}
	if response != nil {
		responseData, _ := os.ReadFile(response.Path)
		detail.Response = extractAIResponse(string(responseData))
		detail.ResponsePath = response.Path
		detail.OutputChars = len(detail.Response)
	}
	return detail, nil
}

func readLegacyResponseOnlyAICall(projectPath, responseName string) (*AICallDetail, error) {
	if !safeLogName(responseName) || !strings.HasSuffix(responseName, ".md") {
		return nil, fmt.Errorf("invalid response log name")
	}
	responsePath := filepath.Join(projectPath, "logs", "responses", responseName)
	responseData, err := os.ReadFile(responsePath)
	if err != nil {
		return nil, err
	}
	responseLog, err := legacyAILogFromPath(responsePath)
	if err != nil {
		return nil, err
	}
	response := extractAIResponse(string(responseData))
	return &AICallDetail{
		AICallSummary: AICallSummary{
			ID:          "legacy-response:" + responseLog.Name,
			Agent:       responseLog.Agent,
			StartedAt:   responseLog.Time,
			HasInput:    false,
			HasOutput:   true,
			OutputChars: len(response),
			Legacy:      true,
		},
		Response:     response,
		ResponsePath: responsePath,
	}, nil
}

func readLegacyAILogs(dir, suffix string) []legacyAILog {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	logs := make([]legacyAILog, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		log, err := legacyAILogFromPath(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Time.Before(logs[j].Time)
	})
	return logs
}

func legacyAILogFromPath(path string) (legacyAILog, error) {
	name := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return legacyAILog{}, err
	}
	agent, loggedAt := parseAILogFilename(name, info.ModTime())
	return legacyAILog{
		Name:  name,
		Path:  path,
		Agent: agent,
		Time:  loggedAt,
		Size:  info.Size(),
	}, nil
}

func pairLegacyAILogs(prompts, responses []legacyAILog) map[string]*legacyAILog {
	pairs := map[string]*legacyAILog{}
	used := map[string]bool{}
	for _, prompt := range prompts {
		var best *legacyAILog
		for i := range responses {
			response := responses[i]
			if used[response.Name] || response.Agent != prompt.Agent || response.Time.Before(prompt.Time) {
				continue
			}
			if response.Time.Sub(prompt.Time) > 2*time.Hour {
				continue
			}
			if best == nil || response.Time.Before(best.Time) {
				candidate := response
				best = &candidate
			}
		}
		if best != nil {
			used[best.Name] = true
		}
		pairs[prompt.Name] = best
	}
	return pairs
}

func parseAILogFilename(name string, fallback time.Time) (string, time.Time) {
	matches := aiLogFilenamePattern.FindStringSubmatch(name)
	if len(matches) != 4 {
		return strings.TrimSuffix(name, filepath.Ext(name)), fallback
	}
	loggedAt, err := time.ParseInLocation("20060102_150405", matches[2], time.Local)
	if err != nil {
		loggedAt = fallback
	}
	return matches[1], loggedAt
}

func splitPromptLog(content string) (string, string) {
	systemMarker := "# SYSTEM PROMPT"
	userMarker := "# USER PROMPT"
	systemIndex := strings.Index(content, systemMarker)
	userIndex := strings.Index(content, userMarker)
	if systemIndex == -1 || userIndex == -1 || userIndex <= systemIndex {
		return content, ""
	}
	systemPrompt := strings.TrimSpace(content[systemIndex+len(systemMarker) : userIndex])
	systemPrompt = strings.Trim(systemPrompt, "- \n\r\t")
	userPrompt := strings.TrimSpace(content[userIndex+len(userMarker):])
	return strings.TrimSpace(systemPrompt), userPrompt
}

func extractAIResponse(content string) string {
	marker := "# AI RESPONSE"
	if idx := strings.Index(content, marker); idx != -1 {
		content = content[idx+len(marker):]
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if newline := strings.Index(content, "\n"); newline != -1 {
			content = content[newline+1:]
		}
		if end := strings.LastIndex(content, "```"); end != -1 {
			content = content[:end]
		}
	}
	return strings.TrimSpace(content)
}

func safeLogName(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
}

func getFile(c *gin.Context) {
	path := c.Param("path")
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	fullPath := filepath.Join(projectPath, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "File not found"})
		return
	}

	// Try to parse as JSON
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: jsonData})
		return
	}

	// Return as text
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]string{
			"content": string(data),
		},
	})
}

func saveFile(c *gin.Context) {
	path := c.Param("path")
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	cleanPath, err := cleanContentPath(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid path"})
		return
	}

	if strings.HasSuffix(cleanPath, ".json") && !json.Valid([]byte(req.Content)) {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid JSON"})
		return
	}

	backupName, err := backupFile(projectPath, cleanPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Failed to backup existing file: " + err.Error()})
		return
	}

	fullPath := filepath.Join(projectPath, cleanPath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := syncDerivedFileAfterSave(cleanPath, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Saved file, but failed to sync derived file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"backup": backupName}})
}

func syncDerivedFileAfterSave(cleanPath, fullPath string) error {
	if filepath.ToSlash(cleanPath) != "story/compose/outline.json" {
		return nil
	}

	outline, err := models.LoadOutline(fullPath)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	if err := outline.Save(fullPath); err != nil {
		return fmt.Errorf("failed to normalize outline json: %w", err)
	}

	markdownPath := filepath.Join(filepath.Dir(fullPath), "outline.md")
	if err := os.WriteFile(markdownPath, []byte(outline.ToMarkdown()), 0644); err != nil {
		return fmt.Errorf("failed to write outline markdown: %w", err)
	}

	return nil
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	exec.Command(cmd, args...).Start()
}

// RPG API handlers

func getRPGData(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: rpgData})
}

func getRPGCharacters(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	characters, ok := rpgData["characters"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"characters": []interface{}{}}})
		return
	}

	// Get instances from characters
	instances, ok := characters["instances"].(map[string]interface{})
	if !ok {
		// Try direct conversion for old format
		charactersList := make([]interface{}, 0, len(characters))
		for _, value := range characters {
			charactersList = append(charactersList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"characters": charactersList}})
		return
	}

	// Convert instances map to array
	charactersList := make([]interface{}, 0, len(instances))
	for _, value := range instances {
		charactersList = append(charactersList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"characters": charactersList}})
}

func getRPGSkills(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	skills, ok := rpgData["skills"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"skills": []interface{}{}}})
		return
	}

	// Get nested skills from skills.skills
	skillsMap, ok := skills["skills"].(map[string]interface{})
	if !ok {
		// Try direct conversion
		skillsList := make([]interface{}, 0, len(skills))
		for _, value := range skills {
			skillsList = append(skillsList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"skills": skillsList}})
		return
	}

	// Convert skills map to array
	skillsList := make([]interface{}, 0, len(skillsMap))
	for _, value := range skillsMap {
		skillsList = append(skillsList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"skills": skillsList}})
}

func getRPGItems(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	items, ok := rpgData["items"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"items": []interface{}{}}})
		return
	}

	// Get nested items from items.items
	itemsMap, ok := items["items"].(map[string]interface{})
	if !ok {
		// Try direct conversion
		itemsList := make([]interface{}, 0, len(items))
		for _, value := range items {
			itemsList = append(itemsList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"items": itemsList}})
		return
	}

	// Convert items map to array
	itemsList := make([]interface{}, 0, len(itemsMap))
	for _, value := range itemsMap {
		itemsList = append(itemsList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"items": itemsList}})
}

func getRPGClasses(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	classes, ok := rpgData["classes"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"classes": []interface{}{}}})
		return
	}

	// Get nested classes from classes.classes
	classesMap, ok := classes["classes"].(map[string]interface{})
	if !ok {
		// Try direct conversion
		classesList := make([]interface{}, 0, len(classes))
		for _, value := range classes {
			classesList = append(classesList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"classes": classesList}})
		return
	}

	// Convert classes map to array
	classesList := make([]interface{}, 0, len(classesMap))
	for _, value := range classesMap {
		classesList = append(classesList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"classes": classesList}})
}

func getRPGEvents(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	events, ok := rpgData["events"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"events": []interface{}{}}})
		return
	}

	// Get nested events from events.events
	eventsMap, ok := events["events"].(map[string]interface{})
	if !ok {
		// Try direct conversion
		eventsList := make([]interface{}, 0, len(events))
		for _, value := range events {
			eventsList = append(eventsList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"events": eventsList}})
		return
	}

	// Convert events map to array
	eventsList := make([]interface{}, 0, len(eventsMap))
	for _, value := range eventsMap {
		eventsList = append(eventsList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"events": eventsList}})
}

func getRPGQuests(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	rpgPath := filepath.Join(projectPath, "rpg_data.json")
	data, err := os.ReadFile(rpgPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "RPG data not found"})
		return
	}

	var rpgData map[string]interface{}
	if err := json.Unmarshal(data, &rpgData); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	quests, ok := rpgData["quests"].(map[string]interface{})
	if !ok {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"quests": []interface{}{}}})
		return
	}

	// Get nested quests from quests.quests
	questsMap, ok := quests["quests"].(map[string]interface{})
	if !ok {
		// Try direct conversion
		questsList := make([]interface{}, 0, len(quests))
		for _, value := range quests {
			questsList = append(questsList, value)
		}
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"quests": questsList}})
		return
	}

	// Convert quests map to array
	questsList := make([]interface{}, 0, len(questsMap))
	for _, value := range questsMap {
		questsList = append(questsList, value)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]interface{}{"quests": questsList}})
}

func listSimulationReports(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	reportsDir := filepath.Join(projectPath, "simulation_reports")
	reports := []map[string]interface{}{}

	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		// Directory doesn't exist, return empty list
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: reports})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(reportsDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var report map[string]interface{}
			if err := json.Unmarshal(data, &report); err != nil {
				continue
			}

			// Extract basic info
			reportInfo := map[string]interface{}{
				"filename": entry.Name(),
			}
			if chapterID, ok := report["chapter_id"].(string); ok {
				reportInfo["chapter_id"] = chapterID
			}
			if chapterName, ok := report["chapter_name"].(string); ok {
				reportInfo["chapter_name"] = chapterName
			}
			if success, ok := report["success"].(bool); ok {
				reportInfo["success"] = success
			}

			reports = append(reports, reportInfo)
		}
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: reports})
}

func getSimulationReport(c *gin.Context) {
	projectPath := c.Query("project")
	if projectPath == "" {
		projectPath = "."
	}

	reportID := c.Param("id")
	if reportID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Report ID required"})
		return
	}

	// Sanitize reportID to prevent directory traversal
	reportID = strings.ReplaceAll(reportID, "..", "")
	reportID = strings.ReplaceAll(reportID, "/", "")
	reportID = strings.ReplaceAll(reportID, "\\", "")

	// Try different filename patterns
	possibleNames := []string{
		reportID,
		"simulation_" + reportID + ".json",
		reportID + ".json",
	}

	reportsDir := filepath.Join(projectPath, "simulation_reports")
	var reportData []byte
	var err error

	for _, name := range possibleNames {
		filePath := filepath.Join(reportsDir, name)
		reportData, err = os.ReadFile(filePath)
		if err == nil {
			break
		}
	}

	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Report not found"})
		return
	}

	var report interface{}
	if err := json.Unmarshal(reportData, &report); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: report})
}
