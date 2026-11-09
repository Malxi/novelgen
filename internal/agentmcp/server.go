// Package agentmcp implements a small local MCP stdio server over the Novelgen
// tool plane. It deliberately exposes project tools, not a second workflow
// engine: skills and adapters decide what to do; Go validates every write.
package agentmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/agentkit"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/agentruntime"
	"novelgen/internal/platform/store"
)

const protocolVersion = "2024-11-05"

// cliTimeout bounds one shelled-out tool call. The MCP transport is
// single-threaded, so a hung CLI must not block every later request forever.
const cliTimeout = 5 * time.Minute

// initTimeout bounds project creation, which only writes scaffolding.
const initTimeout = 2 * time.Minute

// Server is a stdio MCP server. In project mode it is bound to one project
// root; in global mode it accepts a projectDir per call and can create a new
// project through book_init.
type Server struct {
	Root        *store.Root
	Exe         string
	Workspace   string
	AllowCreate bool
	SessionID   string
}

// New returns a server for one project root.
func New(root *store.Root, exe string) *Server {
	return &Server{Root: root, Exe: exe, SessionID: newMCPSessionID()}
}

// NewGlobal returns a workspace-scoped server for WorkBuddy-first use. The
// workspace bounds project creation and every projectDir argument; callers pass
// the project directory to each tool because one server can serve many books.
func NewGlobal(workspace, exe string) *Server {
	dir, err := filepath.Abs(strings.TrimSpace(workspace))
	if err != nil || strings.TrimSpace(workspace) == "" {
		dir, _ = filepath.Abs(".")
	}
	return &Server{Workspace: dir, Exe: exe, AllowCreate: true, SessionID: newMCPSessionID()}
}

func newMCPSessionID() string {
	if value := strings.TrimSpace(os.Getenv("CODEBUDDY_SESSION_ID")); value != "" {
		return value
	}
	return fmt.Sprintf("mcp-%d", time.Now().UnixNano())
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Serve reads newline-delimited JSON-RPC requests until EOF.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}
		if len(request.ID) == 0 {
			// Notifications (for example notifications/initialized) have no response.
			continue
		}
		response := s.handle(ctx, request)
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, request rpcRequest) rpcResponse {
	response := rpcResponse{JSONRPC: "2.0", ID: request.ID}
	var result interface{}
	var err error
	switch request.Method {
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "novelgen",
				"version": "0.1.0",
			},
		}
	case "ping":
		result = map[string]interface{}{}
	case "tools/list":
		result = map[string]interface{}{"tools": bookToolDefinitions(s.AllowCreate)}
	case "tools/call":
		var params toolCallParams
		if err = json.Unmarshal(request.Params, &params); err != nil {
			break
		}
		result, err = s.callTool(ctx, params.Name, params.Arguments)
	default:
		response.Error = &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", request.Method)}
		return response
	}
	if err != nil {
		response.Error = &rpcError{Code: -32000, Message: err.Error()}
		return response
	}
	response.Result = result
	return response
}

func (s *Server) callTool(ctx context.Context, name string, raw json.RawMessage) (*toolResult, error) {
	args := map[string]interface{}{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	projectDir := stringArg(args, "projectDir")
	switch name {
	case "book_init", "novelgen_init":
		return s.initProject(ctx, args)
	case "book_status", "novelgen_status":
		value, err := s.status(projectDir)
		if err != nil {
			return nil, err
		}
		return textResult(value, false), nil
	case "book_ideas":
		args["section"] = "ideas"
		return s.runStandaloneCLI(ctx, "book_ideas", queryArgs(args), nil)
	case "book_read", "novelgen_query":
		return s.runCLI(ctx, projectDir, "book_read", queryArgs(args), nil)
	case "book_check", "novelgen_check":
		return s.runCLI(ctx, projectDir, "book_check", checkArgs(args), nil)
	case "book_write":
		return s.writeTool(ctx, projectDir, args)
	case "novelgen_patch":
		patch, ok := args["patch"]
		if !ok {
			return nil, fmt.Errorf("patch is required")
		}
		body, err := json.Marshal(patch)
		if err != nil {
			return nil, err
		}
		return s.runCLI(ctx, projectDir, "novelgen_patch", patchArgs(args), body)
	case "novelgen_replace":
		artifact, ok := args["artifact"]
		if !ok {
			return nil, fmt.Errorf("artifact is required")
		}
		body, err := json.Marshal(artifact)
		if err != nil {
			return nil, err
		}
		return s.runCLI(ctx, projectDir, "novelgen_replace", replaceArgs(args), body)
	case "book_refresh", "novelgen_refresh":
		return s.runCLI(ctx, projectDir, "book_refresh", refreshArgs(args), nil)
	case "book_guide":
		value, err := guideResult(args)
		if err != nil {
			return nil, err
		}
		return textResult(value, false), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) writeTool(ctx context.Context, projectDir string, args map[string]interface{}) (*toolResult, error) {
	mode := strings.ToLower(strings.TrimSpace(stringArg(args, "mode")))
	if mode == "" {
		if _, ok := args["artifact"]; ok {
			mode = "replace"
		} else {
			mode = "patch"
		}
	}
	switch mode {
	case "replace":
		artifact, ok := args["artifact"]
		if !ok {
			return nil, fmt.Errorf("artifact is required for mode=replace")
		}
		body, err := json.Marshal(artifact)
		if err != nil {
			return nil, err
		}
		result, err := s.runCLI(ctx, projectDir, "book_write", replaceArgs(args), body)
		return withSchemaHint(result, stringArg(args, "section")), err
	case "patch":
		patch, ok := args["patch"]
		if !ok {
			return nil, fmt.Errorf("patch is required for mode=patch")
		}
		body, err := json.Marshal(patch)
		if err != nil {
			return nil, err
		}
		result, err := s.runCLI(ctx, projectDir, "book_write", patchArgs(args), body)
		return withSchemaHint(result, stringArg(args, "section")), err
	default:
		return nil, fmt.Errorf("unsupported mode %q; valid modes: patch, replace", mode)
	}
}

func guideResult(args map[string]interface{}) (map[string]interface{}, error) {
	kind := strings.ToLower(strings.TrimSpace(stringArg(args, "kind")))
	name := strings.TrimSpace(stringArg(args, "name"))
	switch kind {
	case "workflow":
		items, err := agentkit.ListWorkflows()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return map[string]interface{}{"ok": true, "kind": kind, "count": len(items), "items": items}, nil
		}
		content, err := agentkit.ReadWorkflow(name)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true, "kind": kind, "name": name, "content": content}, nil
	case "skill":
		items, err := agentruntime.ListEmbeddedSkills()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return map[string]interface{}{"ok": true, "kind": kind, "count": len(items), "items": items}, nil
		}
		content, err := agentruntime.ReadEmbeddedSkill(name)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true, "kind": kind, "name": name, "content": content}, nil
	case "reference":
		items, err := agentkit.ListReferences()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return map[string]interface{}{"ok": true, "kind": kind, "count": len(items), "items": items}, nil
		}
		content, err := agentkit.ReadReference(name)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true, "kind": kind, "name": name, "content": content}, nil
	default:
		return nil, fmt.Errorf("unsupported guide kind %q; valid kinds: workflow, skill, reference", kind)
	}
}

func (s *Server) status(projectDir string) (interface{}, error) {
	root, err := s.resolveProject(projectDir)
	if err != nil {
		return nil, err
	}
	status, err := project.Open(root).Status()
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"project": map[string]interface{}{
			"root":      status.Root,
			"name":      status.Name,
			"language":  status.Language,
			"structure": status.Structure,
		},
		"setup": status.Setup,
		"outline": map[string]interface{}{
			"parts":    status.Outline.Parts,
			"volumes":  status.Outline.Volumes,
			"chapters": status.Outline.Chapters,
		},
		"chapters": status.Chapters,
		"next": map[string]interface{}{
			"stage":    status.Next.Stage,
			"targetId": status.Next.TargetID,
			"reason":   status.Next.Reason,
		},
	}
	if len(status.Warnings) > 0 {
		result["warnings"] = status.Warnings
	}
	return result, nil
}

func (s *Server) runCLI(ctx context.Context, projectDir, tool string, args []string, stdin []byte) (*toolResult, error) {
	root, err := s.resolveProject(projectDir)
	if err != nil {
		return nil, err
	}
	exe := s.exe()
	args = append(args, "--dir", root.Dir())
	runCtx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, exe, args...)
	command.Dir = root.Dir()
	command.Env = append(os.Environ(),
		"NOVELGEN_ORIGIN=mcp",
		"NOVELGEN_TOOL="+tool,
		"NOVELGEN_MCP_SESSION="+s.SessionID,
		"NOVELGEN_INPUT_BYTES="+strconv.Itoa(len(stdin)),
	)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		text = strings.TrimSpace(stderr.String())
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if text == "" {
			text = fmt.Sprintf("novelgen command timed out after %s", cliTimeout)
		}
		return textResult(text, true), nil
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return textResult(text, true), nil
	}
	return textResult(text, false), nil
}

func (s *Server) exe() string {
	if strings.TrimSpace(s.Exe) != "" {
		return strings.TrimSpace(s.Exe)
	}
	return "novelgen"
}

// resolveProject returns the project a tool call targets. Project-mode servers
// are fixed to Root; global servers require projectDir (or use the workspace
// itself when it already contains novel.json).
func (s *Server) resolveProject(projectDir string) (*store.Root, error) {
	projectDir = strings.TrimSpace(projectDir)
	if s.Root != nil {
		if projectDir == "" || filepath.Clean(projectDir) == filepath.Clean(s.Root.Dir()) {
			return s.Root, nil
		}
		return nil, fmt.Errorf("this MCP server is bound to %s; omit projectDir", s.Root.Dir())
	}
	if projectDir == "" {
		if s.Workspace != "" {
			if book, err := project.OpenDir(s.Workspace); err == nil && book.Root().HasConfig() {
				return book.Root(), nil
			}
		}
		return nil, fmt.Errorf("projectDir is required; call book_init first or pass the project directory")
	}
	target, err := s.projectPath(projectDir, true)
	if err != nil {
		return nil, err
	}
	book, err := project.OpenDir(target)
	if err != nil {
		return nil, err
	}
	root := book.Root()
	if !root.HasConfig() {
		return nil, fmt.Errorf("%s is not a Novelgen project (novel.json not found)", target)
	}
	return root, nil
}

// projectPath resolves a projectDir. Relative paths are always bounded by the
// workspace. Absolute paths may point at an existing project outside it (so a
// global server can continue a book after the workspace changes); creation uses
// requireInside=false and stays inside the workspace.
func (s *Server) projectPath(value string, allowOutside bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("projectDir is required")
	}
	absolute := filepath.IsAbs(value)
	if !absolute {
		base := s.Workspace
		if base == "" {
			base, _ = os.Getwd()
		}
		value = filepath.Join(base, value)
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	enforceWorkspace := !allowOutside || !absolute
	if enforceWorkspace && s.Workspace != "" {
		rel, relErr := filepath.Rel(s.Workspace, abs)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("projectDir %s is outside the MCP workspace %s", abs, s.Workspace)
		}
	}
	return abs, nil
}

// initProject creates a new Novelgen project through the CLI. The MCP server
// only supplies arguments; `novelgen init` remains the single owner of project
// scaffolding.
func (s *Server) initProject(ctx context.Context, args map[string]interface{}) (*toolResult, error) {
	if !s.AllowCreate {
		return nil, fmt.Errorf("book_init is only available with --global")
	}
	name := stringArg(args, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("name must be a book title, not a path")
	}
	target := ""
	if raw := stringArg(args, "projectDir"); raw != "" {
		resolved, err := s.projectPath(raw, false)
		if err != nil {
			return nil, err
		}
		target = resolved
	} else {
		target = filepath.Join(s.Workspace, safeProjectDirName(name))
		if s.Workspace == "" {
			return nil, fmt.Errorf("workspace is not configured; pass projectDir")
		}
	}
	if existing, err := project.OpenDir(target); err == nil && existing.Root().HasConfig() {
		return nil, fmt.Errorf("a Novelgen project already exists at %s", target)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("create project directory: %w", err)
	}

	chapters := intArg(args, "chapters")
	if chapters <= 0 {
		chapters = 20
	}
	if chapters > 10000 {
		chapters = 10000
	}
	parts := intArg(args, "parts")
	if parts <= 0 {
		parts = 1
	}
	volumes := intArg(args, "volumes")
	if volumes <= 0 {
		volumes = 1
	}
	language := stringArg(args, "language")
	if language == "" {
		language = "zh"
	}
	invocation := []string{
		"init", name,
		"--dir", target,
		"--parts", strconv.Itoa(parts),
		"--volumes", strconv.Itoa(volumes),
		"--chapter", strconv.Itoa(chapters),
		"--language", language,
	}
	if genre := stringArg(args, "genre"); genre != "" {
		invocation = append(invocation, "--genre", genre)
	}

	runCtx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, s.exe(), invocation...)
	command.Dir = target
	command.Env = append(os.Environ(),
		"NOVELGEN_ORIGIN=mcp",
		"NOVELGEN_TOOL=book_init",
		"NOVELGEN_MCP_SESSION="+s.SessionID,
	)
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return textResult(fmt.Sprintf("novelgen init failed: %v\n%s", err, text), true), nil
	}
	book, err := project.OpenDir(target)
	if err != nil {
		return nil, err
	}
	root := book.Root()
	if !root.HasConfig() {
		return textResult(fmt.Sprintf("novelgen init reported success but %s has no novel.json\n%s", target, text), true), nil
	}
	result := map[string]interface{}{
		"ok":         true,
		"projectDir": root.Dir(),
		"name":       name,
		"parts":      parts,
		"volumes":    volumes,
		"chapters":   chapters,
		"language":   language,
		"next": map[string]interface{}{
			"stage":    "setup",
			"targetId": "setup",
			"reason":   "项目已创建，下一步生成故事设定。",
		},
		"message": "Project created. Use this harness's model for setup/outline/prose and commit through book_write; do not run novelgen setup gen or the write pipeline.",
	}
	return textResult(result, false), nil
}

func safeProjectDirName(name string) string {
	var builder strings.Builder
	for _, char := range name {
		switch char {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			builder.WriteRune('-')
		default:
			builder.WriteRune(char)
		}
	}
	dir := strings.Trim(strings.TrimSpace(builder.String()), ". ")
	if dir == "" {
		return "novel"
	}
	return dir
}

func (s *Server) runStandaloneCLI(ctx context.Context, tool string, args []string, stdin []byte) (*toolResult, error) {
	dir := strings.TrimSpace(s.Workspace)
	if dir == "" {
		dir, _ = os.Getwd()
	}
	runCtx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, s.exe(), args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"NOVELGEN_ORIGIN=mcp",
		"NOVELGEN_TOOL="+tool,
		"NOVELGEN_MCP_SESSION="+s.SessionID,
		"NOVELGEN_INPUT_BYTES="+strconv.Itoa(len(stdin)),
	)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		text = strings.TrimSpace(stderr.String())
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if text == "" {
			text = fmt.Sprintf("novelgen command timed out after %s", cliTimeout)
		}
		return textResult(text, true), nil
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return textResult(text, true), nil
	}
	return textResult(text, false), nil
}

func textResult(value interface{}, isError bool) *toolResult {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	default:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			text = fmt.Sprint(value)
		} else {
			text = string(data)
		}
	}
	return &toolResult{Content: []toolContent{{Type: "text", Text: text}}, IsError: isError}
}

func withSchemaHint(result *toolResult, section string) *toolResult {
	if result == nil || !result.IsError || len(result.Content) == 0 {
		return result
	}
	section = strings.ToLower(strings.TrimSpace(section))
	name := ""
	switch section {
	case "setup":
		name = "setup-schema"
	case "outline":
		name = "outline-schema"
	default:
		return result
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "cannot unmarshal") && !strings.Contains(text, "invalid setup JSON") && !strings.Contains(text, "invalid outline JSON") {
		return result
	}
	result.Content[0].Text = text + fmt.Sprintf("\nHint: read book_guide(kind=\"reference\", name=%q) before retrying.", name)
	return result
}

func queryArgs(args map[string]interface{}) []string {
	out := []string{"tool", "query", normalizeQuerySection(stringArg(args, "section"))}
	if value := stringArg(args, "type"); value != "" {
		out = append(out, "--type", value)
	}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if value := stringArg(args, "name"); value != "" {
		out = append(out, "--name", value)
	}
	if value := stringArg(args, "entityType"); value != "" {
		out = append(out, "--entity-type", value)
	}
	if value := stringArg(args, "chapterId"); value != "" {
		out = append(out, "--chapter-id", value)
	}
	if value := stringArg(args, "volumeId"); value != "" {
		out = append(out, "--volume-id", value)
	}
	if value := stringArg(args, "view"); value != "" {
		out = append(out, "--view", value)
	}
	if value := stringArg(args, "fields"); value != "" {
		out = append(out, "--fields", value)
	}
	if truthy(args["content"]) || truthy(args["includeContent"]) {
		out = append(out, "--content")
	}
	if value := intArg(args, "limit"); value > 0 {
		out = append(out, "--limit", fmt.Sprint(value))
	}
	if value := stringArg(args, "genre"); value != "" {
		out = append(out, "--genre", value)
	}
	if value := stringArg(args, "level"); value != "" {
		out = append(out, "--level", value)
	}
	if value := stringArg(args, "mode"); value != "" {
		out = append(out, "--mode", value)
	}
	if value := intArg(args, "seed"); value > 0 {
		out = append(out, "--seed", fmt.Sprint(value))
	}
	if value := intArg(args, "count"); value > 0 {
		out = append(out, "--count", fmt.Sprint(value))
	}
	if value := intArg(args, "select"); value > 0 {
		out = append(out, "--select", fmt.Sprint(value))
	}
	if value := stringArg(args, "source"); value != "" {
		out = append(out, "--source", value)
	}
	if value := stringArg(args, "category"); value != "" {
		out = append(out, "--category", value)
	}
	if value := stringArg(args, "intent"); value != "" {
		out = append(out, "--intent", value)
	}
	if value := intArg(args, "maxChars"); value > 0 {
		out = append(out, "--max-chars", fmt.Sprint(value))
	}
	return out
}

func checkArgs(args map[string]interface{}) []string {
	kind := "all"
	switch strings.ToLower(strings.TrimSpace(stringArg(args, "profile"))) {
	case "quality":
		kind = "quality"
	case "schema":
		kind = "schema"
	case "rpg", "simulation":
		kind = "simulation"
	case "custom":
		kind = "custom"
	}
	out := []string{"tool", "check", kind}
	if value := stringArg(args, "target"); value != "" {
		out = append(out, "--target", value)
	}
	if value := stringArg(args, "scope"); value != "" {
		out = append(out, "--scope", value)
	}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if value := stringArg(args, "minPriority"); value != "" {
		out = append(out, "--min-priority", value)
	} else if value := stringArg(args, "minSeverity"); value != "" {
		out = append(out, "--min-priority", value)
	}
	maxIssues := intArg(args, "maxIssues")
	if maxIssues <= 0 {
		maxIssues = intArg(args, "maxFindings")
	}
	if maxIssues > 0 {
		out = append(out, "--max-issues", fmt.Sprint(maxIssues))
	}
	return out
}

func patchArgs(args map[string]interface{}) []string {
	out := []string{"tool", "patch", stringArg(args, "section")}
	if value := stringArg(args, "target"); value != "" {
		out = append(out, "--target", value)
	}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if truthy(args["apply"]) {
		out = append(out, "--apply")
	}
	if truthy(args["refreshDerived"]) {
		out = append(out, "--refresh-derived")
	}
	maxIssues := intArg(args, "maxIssues")
	if maxIssues <= 0 {
		maxIssues = 12
	}
	out = append(out, "--max-issues", fmt.Sprint(maxIssues))
	return out
}

func replaceArgs(args map[string]interface{}) []string {
	out := []string{"tool", "replace", stringArg(args, "section")}
	if value := stringArg(args, "target"); value != "" {
		out = append(out, "--target", value)
	}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if truthy(args["apply"]) {
		out = append(out, "--apply")
	}
	maxIssues := intArg(args, "maxIssues")
	if maxIssues <= 0 {
		maxIssues = 12
	}
	out = append(out, "--max-issues", fmt.Sprint(maxIssues))
	return out
}

func refreshArgs(args map[string]interface{}) []string {
	out := []string{"tool", "refresh", stringArg(args, "target")}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if value := intArg(args, "batchSize"); value > 0 {
		out = append(out, "--batch-size", fmt.Sprint(value))
	}
	return out
}

func stringArg(args map[string]interface{}, name string) string {
	value, _ := args[name].(string)
	return strings.TrimSpace(value)
}

func intArg(args map[string]interface{}, name string) int {
	switch value := args[name].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func truthy(value interface{}) bool {
	result, _ := value.(bool)
	return result
}

func normalizeQuerySection(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "setup", "story-setup", "storysetup":
		return "story-setup"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func toolDefinitions(allowCreate bool) []map[string]interface{} {
	projectDir := map[string]interface{}{
		"type":        "string",
		"description": "Project directory (relative to the MCP workspace or absolute inside it). Omit in project mode or after the workspace itself is a project.",
	}
	tools := make([]map[string]interface{}, 0, 7)
	if allowCreate {
		tools = append(tools, map[string]interface{}{
			"name":        "novelgen_init",
			"description": "Create a new Novelgen novel project under the MCP workspace and install the WorkBuddy project adapter. Returns the projectDir to pass to the other tools.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":       map[string]interface{}{"type": "string", "description": "Book title."},
					"projectDir": map[string]interface{}{"type": "string", "description": "Optional directory relative to the workspace; defaults to a directory named after the book."},
					"genre":      map[string]interface{}{"type": "string", "description": "Optional comma-separated genres."},
					"parts":      map[string]interface{}{"type": "number", "description": "Number of parts (default 1)."},
					"volumes":    map[string]interface{}{"type": "number", "description": "Number of volumes per part (default 1)."},
					"chapters":   map[string]interface{}{"type": "number", "description": "Number of chapters per volume (default 20)."},
					"language":   map[string]interface{}{"type": "string", "description": "Story language (default zh)."},
				},
				"required": []string{"name"},
			},
		})
	}
	tools = append(tools,
		map[string]interface{}{
			"name":        "novelgen_status",
			"description": "Return Novelgen project state and the next stage.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"projectDir": projectDir},
			},
		},
		map[string]interface{}{
			"name":        "novelgen_query",
			"description": "Read project state through `novelgen tool query`.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir":     projectDir,
					"section":        map[string]interface{}{"type": "string", "description": "story-setup, outline, craft, chapter, context, ideas, logs"},
					"type":           map[string]interface{}{"type": "string"},
					"id":             map[string]interface{}{"type": "string"},
					"name":           map[string]interface{}{"type": "string"},
					"entityType":     map[string]interface{}{"type": "string", "description": "Entity type for outline refs/events: character, item, location, storyline."},
					"chapterId":      map[string]interface{}{"type": "string"},
					"volumeId":       map[string]interface{}{"type": "string"},
					"view":           map[string]interface{}{"type": "string"},
					"fields":         map[string]interface{}{"type": "string"},
					"content":        map[string]interface{}{"type": "boolean", "description": "Include chapter/log text content when available."},
					"limit":          map[string]interface{}{"type": "number"},
					"genre":          map[string]interface{}{"type": "string", "description": "Genre hint for ideas queries."},
					"level":          map[string]interface{}{"type": "string"},
					"mode":           map[string]interface{}{"type": "string"},
					"seed":           map[string]interface{}{"type": "number"},
					"count":          map[string]interface{}{"type": "number"},
					"select":         map[string]interface{}{"type": "number"},
					"source":         map[string]interface{}{"type": "string"},
					"category":       map[string]interface{}{"type": "string"},
					"intent":         map[string]interface{}{"type": "string"},
					"maxChars":       map[string]interface{}{"type": "number"},
					"includeContent": map[string]interface{}{"type": "boolean", "description": "Alias for content."},
				},
				"required": []string{"section"},
			},
		},
		map[string]interface{}{
			"name":        "novelgen_check",
			"description": "Run a scoped Novelgen check.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir":  projectDir,
					"target":      map[string]interface{}{"type": "string"},
					"scope":       map[string]interface{}{"type": "string"},
					"id":          map[string]interface{}{"type": "string"},
					"minPriority": map[string]interface{}{"type": "string"},
					"maxIssues":   map[string]interface{}{"type": "number"},
				},
			},
		},
		map[string]interface{}{
			"name":        "novelgen_patch",
			"description": "Apply or dry-run a validated Novelgen patch.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir": projectDir,
					"section":    map[string]interface{}{"type": "string"},
					"target":     map[string]interface{}{"type": "string"},
					"id":         map[string]interface{}{"type": "string"},
					"patch":      map[string]interface{}{"type": "object"},
					"apply":      map[string]interface{}{"type": "boolean"},
					"maxIssues":  map[string]interface{}{"type": "number", "description": "Cap returned check issues (default 12)."},
				},
				"required": []string{"section", "patch"},
			},
		},
		map[string]interface{}{
			"name":        "novelgen_replace",
			"description": "Validate and (with apply=true) replace a complete setup/outline/craft/chapter/recap artifact. Without apply this is a dry run; whole-outline results list removed chapters.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir": projectDir,
					"section":    map[string]interface{}{"type": "string"},
					"target":     map[string]interface{}{"type": "string"},
					"id":         map[string]interface{}{"type": "string"},
					"artifact":   map[string]interface{}{"type": "object"},
					"apply":      map[string]interface{}{"type": "boolean", "description": "Write the replacement; default false (dry run)."},
					"maxIssues":  map[string]interface{}{"type": "number", "description": "Cap returned check issues (default 12)."},
				},
				"required": []string{"section", "artifact"},
			},
		},
		map[string]interface{}{
			"name":        "novelgen_refresh",
			"description": "Refresh derived chapter DSL or narrative memory.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir": projectDir,
					"target":     map[string]interface{}{"type": "string"},
					"id":         map[string]interface{}{"type": "string"},
					"batchSize":  map[string]interface{}{"type": "number"},
				},
				"required": []string{"target"},
			},
		},
	)
	return tools
}
