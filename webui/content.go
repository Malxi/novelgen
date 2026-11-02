package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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
