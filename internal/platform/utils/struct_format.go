package utils

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// StructToJSONSchema converts a struct to a JSON schema representation for AI output format
// It uses reflection to generate a schema that shows the expected JSON structure
//
// Tags:
//   - json:"name" - field name in JSON
//   - json:"name,omitempty" - optional field
//   - desc:"description" - field description for schema
//
// Example:
//
//	type StorySetup struct {
//	    ProjectName string `json:"project_name" desc:"Name of the novel project"`
//	}
//	schema := StructToJSONSchema(StorySetup{}, "  ")
func StructToJSONSchema(v interface{}, indent string) string {
	if v == nil {
		return "null"
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "null"
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		return structToJSONSchema(val, indent)
	case reflect.Slice, reflect.Array:
		return sliceToJSONSchema(val, indent)
	case reflect.Map:
		return mapToJSONSchema(val, indent)
	case reflect.String:
		return "\"string\""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "unknown"
	}
}

// structToJSONSchema converts a struct to JSON schema format
func structToJSONSchema(val reflect.Value, indent string) string {
	typ := val.Type()
	var result strings.Builder

	result.WriteString("{\n")

	fieldCount := 0
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get json tag
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		// Parse json tag
		fieldName := fieldType.Name
		isOptional := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					isOptional = true
				}
			}
		}

		// Get description from desc tag
		desc := fieldType.Tag.Get("desc")

		if fieldCount > 0 {
			result.WriteString(",\n")
		}
		fieldCount++

		// Write field
		result.WriteString(fmt.Sprintf("%s\"%s\": ", indent+"  ", fieldName))

		// Write value/schema
		fieldSchema := fieldToJSONSchema(field, indent+"  ")
		result.WriteString(fieldSchema)

		// Add description comment if present
		if desc != "" {
			result.WriteString(fmt.Sprintf(" // %s", desc))
		}
		if isOptional {
			result.WriteString(" (optional)")
		}
	}

	if fieldCount > 0 {
		result.WriteString("\n")
	}
	result.WriteString(indent + "}")

	return result.String()
}

// fieldToJSONSchema converts a single field to JSON schema format
func fieldToJSONSchema(val reflect.Value, indent string) string {
	switch val.Kind() {
	case reflect.String:
		return "\"string\""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "0"
	case reflect.Float32, reflect.Float64:
		return "0.0"
	case reflect.Bool:
		return "true"
	case reflect.Slice, reflect.Array:
		return sliceToJSONSchema(val, indent)
	case reflect.Struct:
		return structToJSONSchema(val, indent)
	case reflect.Ptr:
		if val.IsNil() {
			return "null"
		}
		return fieldToJSONSchema(val.Elem(), indent)
	case reflect.Map:
		return mapToJSONSchema(val, indent)
	default:
		return fmt.Sprintf("\"%s\"", val.Type().String())
	}
}

// sliceToJSONSchema converts a slice to JSON schema format
func sliceToJSONSchema(val reflect.Value, indent string) string {
	if val.Len() == 0 {
		// Try to get element type from type info
		elemType := val.Type().Elem()
		switch elemType.Kind() {
		case reflect.String:
			return "[\"string\"]"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "[0]"
		case reflect.Float32, reflect.Float64:
			return "[0.0]"
		case reflect.Bool:
			return "[true]"
		case reflect.Struct:
			// Create zero value of element to get schema
			elemVal := reflect.New(elemType).Elem()
			return "[\n" + indent + "  " + structToJSONSchema(elemVal, indent+"  ") + "\n" + indent + "]"
		default:
			return "[]"
		}
	}

	// Use first element as example
	elemSchema := fieldToJSONSchema(val.Index(0), indent+"  ")
	return "[\n" + indent + "  " + elemSchema + "\n" + indent + "]"
}

// mapToJSONSchema converts a map to JSON schema format
func mapToJSONSchema(val reflect.Value, indent string) string {
	if val.Len() == 0 {
		return "{}"
	}

	var result strings.Builder
	result.WriteString("{\n")

	keys := val.MapKeys()
	for i, key := range keys {
		if i > 0 {
			result.WriteString(",\n")
		}
		value := val.MapIndex(key)
		keyStr := fmt.Sprintf("%v", key.Interface())
		valueSchema := fieldToJSONSchema(value, indent+"  ")
		result.WriteString(fmt.Sprintf("%s\"%s\": %s", indent+"  ", keyStr, valueSchema))
	}

	result.WriteString("\n" + indent + "}")
	return result.String()
}

// StructToMarkdown converts a struct to markdown format
// It uses reflection to traverse the struct and create a human-readable markdown document
//
// Tags:
//   - md:"title" - use this field as the title (h1/h2/h3 etc)
//   - md:"heading" - use this field as a heading
//   - md:"-" - skip this field
//   - md:"bullet" - format slice items as bullet points
//   - md:"number" - format slice items as numbered list
//
// Example:
//
//	type Chapter struct {
//	    Title   string   `md:"title"`
//	    Summary string   `md:"heading"`
//	    Beats   []string `md:"bullet"`
//	}
func StructToMarkdown(v interface{}, level int) string {
	if v == nil {
		return ""
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		return structToMarkdown(val, level)
	case reflect.Slice, reflect.Array:
		return sliceToMarkdown(val, level)
	case reflect.Map:
		return mapToMarkdown(val, level)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// structToMarkdown converts a struct to markdown format
func structToMarkdown(val reflect.Value, level int) string {
	typ := val.Type()
	var result strings.Builder

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Check markdown tag
		mdTag := fieldType.Tag.Get("md")
		if mdTag == "-" {
			continue
		}

		// Get field name from json tag or field name
		fieldName := fieldType.Name
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}
		// Convert snake_case to Title Case for display
		displayName := snakeToTitle(fieldName)

		// Skip empty values
		if isEmptyValue(field) {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			s := field.String()
			if mdTag == "title" {
				result.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", level), s))
			} else if mdTag == "heading" {
				result.WriteString(fmt.Sprintf("**%s:** %s\n\n", displayName, s))
			} else {
				result.WriteString(fmt.Sprintf("**%s:** %s\n\n", displayName, s))
			}

		case reflect.Slice, reflect.Array:
			if field.Len() == 0 {
				continue
			}
			elemKind := field.Type().Elem().Kind()

			// Check if it's a slice of primitives (string, int, etc.)
			if elemKind == reflect.String || elemKind == reflect.Int || elemKind == reflect.Int64 {
				result.WriteString(fmt.Sprintf("**%s:** ", displayName))
				items := make([]string, field.Len())
				for j := 0; j < field.Len(); j++ {
					items[j] = fmt.Sprintf("%v", field.Index(j).Interface())
				}
				result.WriteString(strings.Join(items, ", "))
				result.WriteString("\n\n")
			} else {
				// Slice of structs
				result.WriteString(fmt.Sprintf("**%s:**\n", displayName))
				for j := 0; j < field.Len(); j++ {
					elem := field.Index(j)
					if mdTag == "number" {
						result.WriteString(fmt.Sprintf("%d. ", j+1))
						result.WriteString(strings.TrimPrefix(structToMarkdown(elem, level+1), "- "))
					} else {
						result.WriteString(structToMarkdown(elem, level+1))
					}
				}
				result.WriteString("\n")
			}

		case reflect.Struct:
			result.WriteString(structToMarkdown(field, level))

		case reflect.Map:
			// Handle map types by showing key-value pairs
			if field.Len() == 0 {
				continue
			}
			result.WriteString(fmt.Sprintf("**%s:**\n", displayName))
			result.WriteString(mapToMarkdown(field, level+1))
			result.WriteString("\n")

		case reflect.Ptr:
			if field.IsNil() {
				continue
			}
			result.WriteString(fmt.Sprintf("**%s:**\n", displayName))
			result.WriteString(StructToMarkdown(field.Interface(), level+1))
			result.WriteString("\n")

		default:
			result.WriteString(fmt.Sprintf("**%s:** %v\n\n", displayName, field.Interface()))
		}
	}

	return result.String()
}

// sliceToMarkdown converts a slice to markdown format
func sliceToMarkdown(val reflect.Value, level int) string {
	if val.Len() == 0 {
		return ""
	}

	var result strings.Builder
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		result.WriteString(fmt.Sprintf("%d. ", i+1))
		result.WriteString(StructToMarkdown(elem.Interface(), level))
		result.WriteString("\n")
	}

	return result.String()
}

func mapToMarkdown(val reflect.Value, level int) string {
	if val.Len() == 0 {
		return ""
	}
	type entry struct {
		key   reflect.Value
		label string
	}
	entries := make([]entry, 0, val.Len())
	for _, key := range val.MapKeys() {
		entries = append(entries, entry{key: key, label: fmt.Sprintf("%v", key.Interface())})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].label < entries[j].label
	})

	var result strings.Builder
	for _, item := range entries {
		value := val.MapIndex(item.key)
		result.WriteString(fmt.Sprintf("- %s: %s\n", item.label, strings.TrimSpace(markdownValueString(value, level+1))))
	}
	return result.String()
}

func markdownValueString(val reflect.Value, level int) string {
	if !val.IsValid() {
		return ""
	}
	for val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.String:
		return val.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", val.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%.2f", val.Float())
	case reflect.Bool:
		return fmt.Sprintf("%v", val.Bool())
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
		return StructToMarkdown(val.Interface(), level)
	default:
		if val.CanInterface() {
			return fmt.Sprintf("%v", val.Interface())
		}
		return ""
	}
}

// isEmptyValue checks if a value is empty
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

// snakeToTitle converts snake_case to Title Case
func snakeToTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
