package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

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

	canonical := isCanonicalProjectPath(cleanPath)
	if canonical {
		err = replaceProjectFile(projectPath, cleanPath, []byte(req.Content))
	} else {
		err = writeProjectFile(projectPath, cleanPath, []byte(req.Content))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: err.Error()})
		return
	}

	if !canonical {
		if err := syncDerivedFileAfterSave(projectPath, cleanPath); err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{Success: false, Error: "Saved file, but failed to sync derived file: " + err.Error()})
			return
		}
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
