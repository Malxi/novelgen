package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

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
