package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
	taskSem   = make(chan struct{}, 4)
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

	execPath, _ := os.Executable()
	cwd, _ := os.Getwd()
	for _, candidate := range novelGenCandidates(execPath, cwd, exePath) {
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

func novelGenCandidates(execPath, cwd, exePath string) []string {
	candidates := []string{}
	if strings.TrimSpace(execPath) != "" {
		webuiDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(webuiDir, "..", "bin", exePath),
			filepath.Join(webuiDir, exePath),
			filepath.Join(webuiDir, "..", exePath),
		)
	}
	if strings.TrimSpace(cwd) != "" {
		candidates = append(candidates,
			filepath.Join(cwd, "..", "bin", exePath),
			filepath.Join(cwd, exePath),
			filepath.Join(cwd, "bin", exePath),
			filepath.Join(cwd, "webui", exePath),
			filepath.Join(cwd, "..", exePath),
		)
	}
	return candidates
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
	cancel    context.CancelFunc
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
		api.GET("/runs", listRunMetrics)
		api.GET("/runs/:id", getRunMetrics)

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
