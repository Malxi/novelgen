package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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
