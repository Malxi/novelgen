package prompts

import (
	"fmt"
	"reflect"
	"strings"
)

// StructToPrompt converts a struct to a formatted prompt string
// It uses reflection to traverse the struct and create a human-readable format
//
// Tags:
//   - prompt:"name" - custom field name in output
//   - prompt:"-" - skip this field
//   - prompt:"inline" - inline nested struct (don't add extra indentation)
//   - prompt:"bullet" - format slice items as bullet points
//
// Example:
//
//	type StorySetup struct {
//	    ProjectName string   `prompt:"Project Name"`
//	    Genres      []string `prompt:"Genres" bullet:"true"`
//	}
func StructToPrompt(v interface{}, indent string) string {
	if v == nil {
		return "None"
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "None"
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		return structToPrompt(val, indent, "")
	case reflect.Slice, reflect.Array:
		return sliceToPrompt(val, indent)
	case reflect.Map:
		return mapToPrompt(val, indent)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// structToPrompt converts a struct value to prompt format
func structToPrompt(val reflect.Value, indent, fieldTag string) string {
	typ := val.Type()
	var result strings.Builder

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Check prompt tag
		promptTag := fieldType.Tag.Get("prompt")
		if promptTag == "-" {
			continue
		}

		// Get field name
		fieldName := fieldType.Name
		if promptTag != "" && promptTag != "inline" {
			fieldName = promptTag
		}

		// Check if inline
		isInline := promptTag == "inline"

		// Format field value
		fieldValue := formatValue(field, indent, isInline)

		// Skip empty values unless it's a boolean or explicitly marked
		if fieldValue == "" || fieldValue == "None" {
			if field.Kind() != reflect.Bool {
				continue
			}
		}

		if isInline {
			result.WriteString(fieldValue)
		} else {
			result.WriteString(fmt.Sprintf("%s%s: %s\n", indent, fieldName, fieldValue))
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// formatValue formats a single field value
func formatValue(val reflect.Value, indent string, inline bool) string {
	switch val.Kind() {
	case reflect.String:
		s := val.String()
		if s == "" {
			return "None"
		}
		return s

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", val.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", val.Uint())

	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%.2f", val.Float())

	case reflect.Bool:
		return fmt.Sprintf("%v", val.Bool())

	case reflect.Slice, reflect.Array:
		return sliceToPrompt(val, indent)

	case reflect.Struct:
		if inline {
			return structToPrompt(val, indent, "")
		}
		return "\n" + structToPrompt(val, indent+"  ", "")

	case reflect.Ptr:
		if val.IsNil() {
			return "None"
		}
		return formatValue(val.Elem(), indent, inline)

	case reflect.Map:
		return mapToPrompt(val, indent)

	default:
		return fmt.Sprintf("%v", val.Interface())
	}
}

// sliceToPrompt converts a slice to bullet point format
func sliceToPrompt(val reflect.Value, indent string) string {
	if val.Len() == 0 {
		return "None"
	}

	var result strings.Builder
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		elemStr := formatValue(elem, indent+"  ", false)

		// If element is a simple type, use bullet format
		if isSimpleType(elem.Kind()) {
			result.WriteString(fmt.Sprintf("\n%s- %s", indent, elemStr))
		} else {
			// For complex types, add a separator
			if i > 0 {
				result.WriteString("\n")
			}
			result.WriteString(fmt.Sprintf("\n%s[%d]", indent, i+1))
			result.WriteString("\n" + elemStr)
		}
	}

	return result.String()
}

// mapToPrompt converts a map to prompt format
func mapToPrompt(val reflect.Value, indent string) string {
	if val.Len() == 0 {
		return "None"
	}

	var result strings.Builder
	for _, key := range val.MapKeys() {
		value := val.MapIndex(key)
		keyStr := fmt.Sprintf("%v", key.Interface())
		valueStr := formatValue(value, indent+"  ", false)
		result.WriteString(fmt.Sprintf("\n%s%s: %s", indent, keyStr, valueStr))
	}

	return result.String()
}

// isSimpleType checks if a kind is a simple/primitive type
func isSimpleType(kind reflect.Kind) bool {
	switch kind {
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		return true
	default:
		return false
	}
}

// StructToPromptData converts a struct to a map[string]string for use in prompt templates
// This is useful when you want to use {{field_name}} placeholders in your prompt
func StructToPromptData(v interface{}) map[string]string {
	result := make(map[string]string)
	if v == nil {
		return result
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return result
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanInterface() {
			continue
		}

		promptTag := fieldType.Tag.Get("prompt")
		if promptTag == "-" {
			continue
		}

		fieldName := fieldType.Name
		if promptTag != "" {
			fieldName = promptTag
		}

		// Convert to snake_case for template keys
		key := toSnakeCase(fieldName)
		result[key] = StructToPrompt(field.Interface(), "")
	}

	return result
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

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
