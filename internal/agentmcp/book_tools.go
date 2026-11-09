package agentmcp

// bookToolDefinitions is the canonical agent-facing tool surface. The legacy
// novelgen_* names remain accepted as aliases but are no longer advertised.
func bookToolDefinitions(allowCreate bool) []map[string]interface{} {
	projectDir := map[string]interface{}{
		"type":        "string",
		"description": "Project directory (relative to the MCP workspace or absolute inside it). Omit in project mode or when the workspace itself is a project.",
	}
	tools := make([]map[string]interface{}, 0, 8)
	if allowCreate {
		tools = append(tools, map[string]interface{}{
			"name":        "book_init",
			"description": "Create a new Novelgen book under the MCP workspace and install its guidance.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":       map[string]interface{}{"type": "string", "description": "Book title."},
					"projectDir": map[string]interface{}{"type": "string", "description": "Optional directory relative to the workspace."},
					"genre":      map[string]interface{}{"type": "string"},
					"parts":      map[string]interface{}{"type": "number"},
					"volumes":    map[string]interface{}{"type": "number"},
					"chapters":   map[string]interface{}{"type": "number", "description": "Chapters per volume."},
					"language":   map[string]interface{}{"type": "string"},
				},
				"required": []string{"name"},
			},
		})
	}
	tools = append(tools,
		map[string]interface{}{
			"name":        "book_status",
			"description": "Return book progress and the next step.",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"projectDir": projectDir},
			},
		},
		map[string]interface{}{
			"name":        "book_read",
			"description": "Read setup, outline, craft, chapter, context, or logs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir": projectDir,
					"section":    map[string]interface{}{"type": "string", "description": "setup, outline, craft, chapter, context, logs"},
					"type":       map[string]interface{}{"type": "string"},
					"id":         map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"entityType": map[string]interface{}{"type": "string"},
					"chapterId":  map[string]interface{}{"type": "string"},
					"volumeId":   map[string]interface{}{"type": "string"},
					"view":       map[string]interface{}{"type": "string"},
					"fields":     map[string]interface{}{"type": "string"},
					"content":    map[string]interface{}{"type": "boolean"},
					"limit":      map[string]interface{}{"type": "number"},
					"maxChars":   map[string]interface{}{"type": "number"},
					"intent":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"section"},
			},
		},
		map[string]interface{}{
			"name":        "book_write",
			"description": "Patch or replace a complete artifact. Dry run unless apply=true.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir":     projectDir,
					"section":        map[string]interface{}{"type": "string"},
					"mode":           map[string]interface{}{"type": "string", "description": "patch or replace (default inferred)."},
					"target":         map[string]interface{}{"type": "string"},
					"id":             map[string]interface{}{"type": "string"},
					"artifact":       map[string]interface{}{"type": "object"},
					"patch":          map[string]interface{}{"type": "object"},
					"apply":          map[string]interface{}{"type": "boolean"},
					"refreshDerived": map[string]interface{}{"type": "boolean"},
					"maxIssues":      map[string]interface{}{"type": "number"},
				},
				"required": []string{"section"},
			},
		},
		map[string]interface{}{
			"name":        "book_refresh",
			"description": "Rebuild derived chapter DSL or narrative memory.",
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
		map[string]interface{}{
			"name":        "book_check",
			"description": "Run quality/schema/rpg checks and return findings.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projectDir":  projectDir,
					"profile":     map[string]interface{}{"type": "string", "description": "quality, schema, rpg, custom, all"},
					"target":      map[string]interface{}{"type": "string"},
					"scope":       map[string]interface{}{"type": "string"},
					"id":          map[string]interface{}{"type": "string"},
					"minPriority": map[string]interface{}{"type": "string"},
					"minSeverity": map[string]interface{}{"type": "string"},
					"maxIssues":   map[string]interface{}{"type": "number"},
					"maxFindings": map[string]interface{}{"type": "number"},
				},
			},
		},
		map[string]interface{}{
			"name":        "book_ideas",
			"description": "Draw deterministic content ideas (namecards, scenarios, world packs, ...).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type":     map[string]interface{}{"type": "string"},
					"genre":    map[string]interface{}{"type": "string"},
					"mode":     map[string]interface{}{"type": "string"},
					"level":    map[string]interface{}{"type": "string"},
					"seed":     map[string]interface{}{"type": "number"},
					"count":    map[string]interface{}{"type": "number"},
					"select":   map[string]interface{}{"type": "number"},
					"source":   map[string]interface{}{"type": "string"},
					"category": map[string]interface{}{"type": "string"},
					"view":     map[string]interface{}{"type": "string"},
				},
			},
		},
		map[string]interface{}{
			"name":        "book_guide",
			"description": "Read generation guidance (workflow, skill, or reference).",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"kind": map[string]interface{}{"type": "string", "description": "workflow, skill, or reference"},
					"name": map[string]interface{}{"type": "string"},
				},
				"required": []string{"kind"},
			},
		},
	)
	// Keep the pre-book names visible for one migration window. Existing project
	// skills and WorkBuddy prompts may still call them; new skills use book_*.
	tools = append(tools, toolDefinitions(allowCreate)...)
	return tools
}
