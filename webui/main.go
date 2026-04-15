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
	"runtime"
	"strings"
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
			return true
		},
	}
	clients = make(map[string]*websocket.Conn)
)

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

var tasks = make(map[string]*Task)

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
	clients[clientID] = conn
	defer delete(clients, clientID)

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
	for _, conn := range clients {
		conn.WriteJSON(map[string]interface{}{
			"type": "task_update",
			"data": task,
		})
	}
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

	cmd := exec.Command("novelgen", args...)
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
		Message:   "Task created",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tasks[task.ID] = task

	// Execute task asynchronously
	go executeTask(task, req.Command, req.Args)

	c.JSON(http.StatusOK, APIResponse{Success: true, Data: task})
}

func executeTask(task *Task, command string, args map[string]interface{}) {
	task.Status = "running"
	task.Message = "Executing command..."
	task.UpdatedAt = time.Now()
	broadcastTaskUpdate(task)

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

	// Find project directory
	projectPath := ""
	if projectDir, ok := args["project_dir"].(string); ok && projectDir != "" {
		projectPath = projectDir
		debugInfo.WriteString(fmt.Sprintf("Project path from args: %s\n", projectPath))
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
					debugInfo.WriteString(fmt.Sprintf("Found project path: %s\n", projectPath))
					break
				}
			}
		}
	}

	// Build command with working directory
	cmd := exec.Command("novelgen", cmdArgs...)

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

	task.Output = debugInfo.String() + string(output)
	task.UpdatedAt = time.Now()

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		task.Message = "Task failed"
	} else {
		task.Status = "completed"
		task.Progress = 100
		task.Message = "Task completed successfully"
	}

	broadcastTaskUpdate(task)
}

func listTasks(c *gin.Context) {
	taskList := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, task)
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: taskList})
}

func getTask(c *gin.Context) {
	id := c.Param("id")
	task, exists := tasks[id]
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: "Task not found"})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Success: true, Data: task})
}

func deleteTask(c *gin.Context) {
	id := c.Param("id")
	delete(tasks, id)
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

	fullPath := filepath.Join(projectPath, path)

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

	c.JSON(http.StatusOK, APIResponse{Success: true})
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
