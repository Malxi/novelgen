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
	"os/exec"
	"strings"
	"time"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

const protocolVersion = "2024-11-05"

// cliTimeout bounds one shelled-out tool call. The MCP transport is
// single-threaded, so a hung CLI must not block every later request forever.
const cliTimeout = 5 * time.Minute

// Server is a stdio MCP server bound to one project root.
type Server struct {
	Root *store.Root
	Exe  string
}

// New returns a server for root.
func New(root *store.Root, exe string) *Server {
	return &Server{Root: root, Exe: exe}
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
		result = map[string]interface{}{"tools": toolDefinitions()}
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
	switch name {
	case "novelgen_status":
		value, err := s.status()
		if err != nil {
			return nil, err
		}
		return textResult(value, false), nil
	case "novelgen_query":
		return s.runCLI(ctx, queryArgs(args), nil)
	case "novelgen_check":
		return s.runCLI(ctx, checkArgs(args), nil)
	case "novelgen_patch":
		patch, ok := args["patch"]
		if !ok {
			return nil, fmt.Errorf("patch is required")
		}
		body, err := json.Marshal(patch)
		if err != nil {
			return nil, err
		}
		return s.runCLI(ctx, patchArgs(args), body)
	case "novelgen_replace":
		artifact, ok := args["artifact"]
		if !ok {
			return nil, fmt.Errorf("artifact is required")
		}
		body, err := json.Marshal(artifact)
		if err != nil {
			return nil, err
		}
		return s.runCLI(ctx, replaceArgs(args), body)
	case "novelgen_refresh":
		return s.runCLI(ctx, refreshArgs(args), nil)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) status() (interface{}, error) {
	if s.Root == nil {
		return nil, fmt.Errorf("project root is not configured")
	}
	status, err := project.Open(s.Root).Status()
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

func (s *Server) runCLI(ctx context.Context, args []string, stdin []byte) (*toolResult, error) {
	if s.Root == nil {
		return nil, fmt.Errorf("project root is not configured")
	}
	exe := s.Exe
	if strings.TrimSpace(exe) == "" {
		exe = "novelgen"
	}
	args = append(args, "--dir", s.Root.Dir())
	runCtx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, exe, args...)
	command.Dir = s.Root.Dir()
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

func queryArgs(args map[string]interface{}) []string {
	out := []string{"tool", "query", stringArg(args, "section")}
	if value := stringArg(args, "type"); value != "" {
		out = append(out, "--type", value)
	}
	if value := stringArg(args, "id"); value != "" {
		out = append(out, "--id", value)
	}
	if value := stringArg(args, "name"); value != "" {
		out = append(out, "--name", value)
	}
	if value := stringArg(args, "view"); value != "" {
		out = append(out, "--view", value)
	}
	if value := intArg(args, "limit"); value > 0 {
		out = append(out, "--limit", fmt.Sprint(value))
	}
	return out
}

func checkArgs(args map[string]interface{}) []string {
	out := []string{"tool", "check", "all"}
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
	}
	if value := intArg(args, "maxIssues"); value > 0 {
		out = append(out, "--max-issues", fmt.Sprint(value))
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

func toolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "novelgen_status",
			"description": "Return Novelgen project state and the next stage.",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			"name":        "novelgen_query",
			"description": "Read project state through `novelgen tool query`.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"section": map[string]interface{}{"type": "string", "description": "story-setup, outline, craft, chapter, context, ideas, logs"},
					"type":    map[string]interface{}{"type": "string"},
					"id":      map[string]interface{}{"type": "string"},
					"name":    map[string]interface{}{"type": "string"},
					"view":    map[string]interface{}{"type": "string"},
					"limit":   map[string]interface{}{"type": "number"},
				},
				"required": []string{"section"},
			},
		},
		{
			"name":        "novelgen_check",
			"description": "Run a scoped Novelgen check.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target":      map[string]interface{}{"type": "string"},
					"scope":       map[string]interface{}{"type": "string"},
					"id":          map[string]interface{}{"type": "string"},
					"minPriority": map[string]interface{}{"type": "string"},
					"maxIssues":   map[string]interface{}{"type": "number"},
				},
			},
		},
		{
			"name":        "novelgen_patch",
			"description": "Apply or dry-run a validated Novelgen patch.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"section": map[string]interface{}{"type": "string"},
					"target":  map[string]interface{}{"type": "string"},
					"id":      map[string]interface{}{"type": "string"},
					"patch":   map[string]interface{}{"type": "object"},
					"apply":   map[string]interface{}{"type": "boolean"},
				},
				"required": []string{"section", "patch"},
			},
		},
		{
			"name":        "novelgen_replace",
			"description": "Validate and (with apply=true) replace a complete setup/outline/craft/chapter/recap artifact. Without apply this is a dry run; whole-outline results list removed chapters.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"section":  map[string]interface{}{"type": "string"},
					"target":   map[string]interface{}{"type": "string"},
					"id":       map[string]interface{}{"type": "string"},
					"artifact": map[string]interface{}{"type": "object"},
					"apply":    map[string]interface{}{"type": "boolean", "description": "Write the replacement; default false (dry run)."},
				},
				"required": []string{"section", "artifact"},
			},
		},
		{
			"name":        "novelgen_refresh",
			"description": "Refresh derived chapter DSL or narrative memory.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target":    map[string]interface{}{"type": "string"},
					"id":        map[string]interface{}{"type": "string"},
					"batchSize": map[string]interface{}{"type": "number"},
				},
				"required": []string{"target"},
			},
		},
	}
}
