package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Browsers always send Origin on WebSocket handshakes; only
			// accept handshakes that started from this machine.
			return isLocalOrigin(r.Header.Get("Origin"))
		},
	}
	clients   = make(map[string]*websocket.Conn)
	clientsMu sync.RWMutex
	writesMu  sync.Mutex
)

// getNovelGenPath returns the path to novelgen executable.
func getNovelGenPath() string {
	// NOVELGEN_BIN pins the binary explicitly. Deployments use it when the
	// binary lives outside the directories we probe, and tests use it to
	// substitute a fake CLI.
	if explicit := strings.TrimSpace(os.Getenv("NOVELGEN_BIN")); explicit != "" {
		return explicit
	}

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
	host := flag.String("host", "127.0.0.1", "Interface to bind (use 0.0.0.0 to expose on the LAN; a token is then required)")
	port := flag.String("port", "8080", "Server port")
	authTokenFlag := flag.String("auth-token", "", "API token; generated automatically when binding a non-loopback address")
	projectsRootFlag := flag.String("projects-root", "", "Directory that holds novel projects (default: ../books next to the server)")
	extraRootsFlag := flag.String("allow-project-root", "", "Comma-separated extra directories that may be served as projects")
	noBrowserFlag := flag.Bool("no-browser", false, "Do not open a browser window on startup")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}
	booksRoot, roots, err := configureProjectRoots(cwd, *projectsRootFlag, splitCommaList(*extraRootsFlag))
	if err != nil {
		log.Fatalf("configure project roots: %v", err)
	}
	projectsRootAbs = booksRoot
	allowedProjectRoots = roots

	authToken = strings.TrimSpace(*authTokenFlag)
	if authToken == "" && !isLoopbackHost(*host) {
		generated, err := newAuthToken()
		if err != nil {
			log.Fatalf("generate API token: %v", err)
		}
		authToken = generated
	}

	gin.SetMode(gin.ReleaseMode)
	r := buildRouter(true)

	// Open browser
	if !*noBrowserFlag {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(startupURL(*host, *port, authToken))
		}()
	}

	fmt.Printf("🚀 NovelGen Web UI is running at %s\n", startupURL(*host, *port, authToken))
	fmt.Printf("   Project roots: %s\n", strings.Join(allowedProjectRoots, ", "))
	if authToken != "" {
		fmt.Printf("   API token: %s (also accepted via the %s header)\n", authToken, authTokenHeader)
	}
	log.Fatal(r.Run(net.JoinHostPort(*host, *port)))
}

// buildRouter wires the HTTP surface. Tests build the router with logging
// disabled; the server builds it with gin's request logger enabled.
func buildRouter(withLogging bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if withLogging {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery())

	// CORS: only local origins may talk to this server.
	config := cors.DefaultConfig()
	config.AllowOriginFunc = isLocalOrigin
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	r.Use(cors.New(config))
	r.Use(originGuard())

	// API routes
	api := r.Group("/api")
	api.Use(authGuard())
	api.Use(projectGuard())
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
	r.GET("/ws", authGuard(), handleWebSocket)

	// Static files (frontend)
	r.Use(bootstrapAuthCookie())
	r.Use(static.Serve("/", static.LocalFile("./frontend/dist", false)))
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})
	return r
}

// startupURL builds the URL the user should open, keeping loopback URLs
// friendly while still including the token when one is required.
func startupURL(host, port, token string) string {
	displayHost := host
	if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "::" {
		displayHost = "localhost"
	}
	raw := fmt.Sprintf("http://%s:%s", displayHost, port)
	if token == "" {
		return raw
	}
	return raw + "/?token=" + url.QueryEscape(token)
}

func splitCommaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
	booksDir := projectsRootAbs
	projects := []ProjectInfo{}

	descriptors, err := listProjectsAt(booksDir)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: projects})
		return
	}

	for _, descriptor := range descriptors {
		if !descriptor.Exists {
			continue
		}
		info := projectInfoFromConfig(descriptor.Path, descriptor.Config, descriptor.Name)
		if len(descriptor.Config) == 0 {
			info = loadProjectInfo(descriptor.Path)
		}
		info.Path = displayProjectPath(descriptor.Path)
		projects = append(projects, *info)
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: projects})
}

// displayProjectPath keeps the short "../books/<name>" shape the frontend
// expects while the server works with absolute paths internally.
func displayProjectPath(abs string) string {
	rel, err := filepath.Rel(mustGetwd(), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)+"..") {
		return abs
	}
	return rel
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// firstProjectInBooks returns the first project directory under the projects
// root, mirroring the previous "../books" scan without depending on the
// process working directory.
func firstProjectInBooks() string {
	projects, err := listProjectsAt(projectsRootAbs)
	if err != nil {
		return ""
	}
	for _, project := range projects {
		if project.Exists {
			return project.Path
		}
	}
	return ""
}

func loadProjectInfo(path string) *ProjectInfo {
	data, err := readProjectFile(path, "novel.json")
	if err != nil {
		return &ProjectInfo{Path: path}
	}
	return projectInfoFromConfig(path, data, filepath.Base(path))
}

func projectInfoFromConfig(path string, data []byte, fallbackName string) *ProjectInfo {
	info := &ProjectInfo{Path: path, Exists: false}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return info
	}

	info.Exists = true
	if name, ok := config["name"].(string); ok {
		info.Name = name
	}
	if info.Name == "" {
		info.Name = fallbackName
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

	projectName, err := safeSegment(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid project name"})
		return
	}

	// Create project using novelgen CLI
	args := []string{"init", projectName}
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

	projectPath := filepath.Join(projectsRootAbs, projectName)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create project directory: %s", err.Error()),
		})
		return
	}

	cmd := exec.Command(getNovelGenPath(), args...)
	// Run in the project directory instead of changing the process working
	// directory, which would race with every other concurrent request.
	cmd.Dir = projectPath
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
	name, err := safeSegment(c.Param("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "invalid project name"})
		return
	}
	projectPath := filepath.Join(projectsRootAbs, name)
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

	// Web tasks are non-interactive. compose regen prompts in the terminal when
	// --prompt is omitted, so provide a default to avoid hanging background jobs.
	if command == "compose" {
		if subcommand, ok := args["subcommand"].(string); ok && subcommand == "regen" {
			if prompt, ok := args["prompt"].(string); !ok || strings.TrimSpace(prompt) == "" {
				args["prompt"] = "Regenerate this outline element while preserving continuity and improving motivation, conflict, beats, payoff, and hook."
			}
		}
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
	delete(tasks, id)
	tasksMu.Unlock()
	c.JSON(http.StatusOK, APIResponse{Success: true})
}

func getOutline(c *gin.Context) {
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "story/compose/outline.json")
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

func backupFile(projectPath, contentPath string) (string, error) {
	sourcePath, err := resolveProjectContentPath(projectPath, contentPath)
	if err != nil {
		return "", err
	}
	cleanPath, err := cleanContentPath(contentPath)
	if err != nil {
		return "", err
	}
	if _, err := statProjectPathAt(projectPath, sourcePath); isNotExist(err) {
		return "", nil
	}

	backupsDir := filepath.Join(projectPath, filepath.Dir(cleanPath), "backups")
	ext := filepath.Ext(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), ext)
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("%s_%s%s", base, timestamp, ext)
	backupPath := filepath.Join(backupsDir, backupFilename)
	for suffix := 2; ; suffix++ {
		if _, err := statProjectPathAt(projectPath, backupPath); isNotExist(err) {
			break
		}
		backupFilename = fmt.Sprintf("%s_%s_%d%s", base, timestamp, suffix, ext)
		backupPath = filepath.Join(backupsDir, backupFilename)
	}

	data, err := readProjectFileAt(projectPath, sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if err := writeProjectFileAt(projectPath, backupPath, data); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupFilename, nil
}

func projectPathFromQuery(c *gin.Context) string {
	if resolved, ok := c.Get(projectContextKey); ok {
		if path, ok := resolved.(string); ok && path != "" {
			return path
		}
	}
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

func fileVersions(projectPath, contentPath string) ([]FileVersion, error) {
	cleanPath, err := cleanContentPath(contentPath)
	if err != nil {
		return nil, err
	}
	if _, err := resolveProjectContentPath(projectPath, contentPath); err != nil {
		return nil, err
	}

	ext := filepath.Ext(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), ext)
	backupsDir := filepath.Join(projectPath, filepath.Dir(cleanPath), "backups")
	if _, err := statProjectPathAt(projectPath, backupsDir); isNotExist(err) {
		return []FileVersion{}, nil
	}

	entries, err := listProjectDirAt(projectPath, backupsDir)
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

	targetPath, err := resolveProjectContentPath(projectPath, cleanPath)
	if err != nil {
		return err
	}
	backupPath, err := resolveProjectContentPath(projectPath, filepath.Join(filepath.Dir(cleanPath), "backups", filename))
	if err != nil {
		return err
	}
	if _, err := statProjectPathAt(projectPath, backupPath); isNotExist(err) {
		return fmt.Errorf("backup not found")
	}

	if _, err := backupFile(projectPath, cleanPath); err != nil {
		return fmt.Errorf("failed to backup current file: %w", err)
	}

	data, err := readProjectFileAt(projectPath, backupPath)
	if err != nil {
		return err
	}
	if err := writeProjectFileAt(projectPath, targetPath, data); err != nil {
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "story/setup/story_setup.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "story/craft/characters.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "story/craft/locations.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "story/craft/items.json")
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
	projectPath := projectPathFromQuery(c)

	entries, err := listProjectDir(projectPath, "chapters")
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
	id, err := safeSegment(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid chapter id"})
		return
	}
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "chapters/"+id+".md")
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
	projectPath := projectPathFromQuery(c)

	entries, err := listProjectDir(projectPath, "drafts")
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
	id, err := safeSegment(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid draft id"})
		return
	}
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "drafts/"+id+".md")
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
	projectPath := projectPathFromQuery(c)

	entries, err := listProjectDir(projectPath, "story/recaps")
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
	projectPath := projectPathFromQuery(c)

	entries, err := listProjectDir(projectPath, "story/reviews")
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

// Agent logs use <agent>_<timestamp>[_<nanos>[_<attempt>]] so concurrent
// writers never overwrite each other; accept any number of numeric suffixes.
var aiLogFilenamePattern = regexp.MustCompile(`^(.+)_(\d{8}_\d{6})(?:_\d+)*\.(md|json)$`)

func getFile(c *gin.Context) {
	path := c.Param("path")
	projectPath := projectPathFromQuery(c)

	fullPath, err := resolveProjectContentPath(projectPath, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid path"})
		return
	}
	data, err := readProjectFileAt(projectPath, fullPath)
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
	projectPath := projectPathFromQuery(c)

	var req struct {
		Content string `json:"content"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	_, err := resolveProjectContentPath(projectPath, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid path"})
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

	// Ensure directory exists
	if err := writeProjectFile(projectPath, cleanPath, []byte(req.Content)); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if err := syncDerivedFileAfterSave(projectPath, cleanPath); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Saved file, but failed to sync derived file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: map[string]string{"backup": backupName}})
}

// syncDerivedFileAfterSave regenerates the derived files for a saved content
// file. The save itself is the user's editor action, but every derived artifact
// is produced by the novelgen CLI, so webui never normalizes project state
// in-process; that keeps "mutations go through exec" true for the whole webui.
func syncDerivedFileAfterSave(projectPath, cleanPath string) error {
	if filepath.ToSlash(cleanPath) != "story/compose/outline.json" {
		return nil
	}

	cmd := novelGenNormalizeCommand(projectPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("novelgen compose normalize failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// novelGenNormalizeCommand builds the CLI invocation that normalizes the
// outline and rewrites its derived markdown even when no cleanup applied.
func novelGenNormalizeCommand(projectPath string) *exec.Cmd {
	cmd := exec.Command(getNovelGenPath(), "compose", "normalize", "--force")
	cmd.Dir = projectPath
	return cmd
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	data, err := readProjectFile(projectPath, "rpg_data.json")
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
	projectPath := projectPathFromQuery(c)

	reports := []map[string]interface{}{}

	entries, err := listProjectDir(projectPath, "simulation_reports")
	if err != nil {
		// Directory doesn't exist, return empty list
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: reports})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			data, err := readProjectFile(projectPath, "simulation_reports/"+entry.Name())
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
	projectPath := projectPathFromQuery(c)

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

	var reportData []byte
	var err error

	for _, name := range possibleNames {
		reportData, err = readProjectFile(projectPath, "simulation_reports/"+name)
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
