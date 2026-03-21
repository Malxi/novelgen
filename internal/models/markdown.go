package models

import (
	"fmt"
	"reflect"
	"strings"
)

// JSONToMarkdown converts any struct to markdown using reflection
// It uses the following struct tags:
//   - md:"-" : ignore this field
//   - md:"title" : use as heading title (level 1-4 based on depth)
//   - md:"heading" : use as bold text on its own line
//   - md:"name" : field name to use as label
//   - md:"inline" : render on same line as label
//   - md:"list" : render as bullet list
//   - md:"numbered" : render as numbered list
//   - md:"code" : render as code block
//   - md:"quote" : render as blockquote
func JSONToMarkdown(v interface{}, depth int) string {
	var sb strings.Builder
	val := reflect.ValueOf(v)
	typ := val.Type()

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
		typ = val.Type()
	}

	// Only process structs
	if val.Kind() != reflect.Struct {
		return fmt.Sprintf("%v", v)
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Get md tag
		mdTag := fieldType.Tag.Get("md")
		if mdTag == "-" {
			continue
		}

		// Skip zero values for omitempty fields
		if isZeroValue(field) && strings.Contains(string(fieldType.Tag), "omitempty") {
			continue
		}

		// Process based on field kind and tag
		switch field.Kind() {
		case reflect.Slice, reflect.Array:
			sb.WriteString(renderSliceField(field, fieldType, mdTag, depth))
		case reflect.Struct:
			sb.WriteString(renderStructField(field, fieldType, mdTag, depth))
		case reflect.Map:
			sb.WriteString(renderMapField(field, fieldType, mdTag, depth))
		default:
			sb.WriteString(renderSimpleField(field, fieldType, mdTag, depth))
		}
	}

	return sb.String()
}

// renderSimpleField renders a simple field (string, int, bool, etc.)
func renderSimpleField(field reflect.Value, fieldType reflect.StructField, mdTag string, depth int) string {
	if isZeroValue(field) {
		return ""
	}

	var sb strings.Builder
	value := fmt.Sprintf("%v", field.Interface())
	fieldName := getFieldName(fieldType, mdTag)

	switch mdTag {
	case "title":
		headingLevel := min(depth+1, 6)
		sb.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", headingLevel), value))
	case "heading":
		sb.WriteString(fmt.Sprintf("**%s**\n\n", value))
	case "inline":
		sb.WriteString(fmt.Sprintf("%s: %s  \n", fieldName, value))
	case "code":
		sb.WriteString(fmt.Sprintf("**%s**:\n```\n%s\n```\n\n", fieldName, value))
	case "quote":
		sb.WriteString(fmt.Sprintf("**%s**:\n> %s\n\n", fieldName, value))
	default:
		// Default: render as "**Field:** value"
		sb.WriteString(fmt.Sprintf("**%s:** %s\n\n", fieldName, value))
	}

	return sb.String()
}

// renderSliceField renders a slice or array field
func renderSliceField(field reflect.Value, fieldType reflect.StructField, mdTag string, depth int) string {
	if field.Len() == 0 {
		return ""
	}

	var sb strings.Builder
	fieldName := getFieldName(fieldType, mdTag)

	// Check if it's a slice of primitives or structs
	elemKind := field.Type().Elem().Kind()
	if elemKind == reflect.Ptr {
		elemKind = field.Type().Elem().Elem().Kind()
	}

	switch mdTag {
	case "list", "numbered":
		// Render as list items
		isNumbered := mdTag == "numbered"
		sb.WriteString(fmt.Sprintf("**%s:**\n", fieldName))
		for i := 0; i < field.Len(); i++ {
			elem := field.Index(i)
			if elem.Kind() == reflect.Ptr && !elem.IsNil() {
				elem = elem.Elem()
			}

			if isNumbered {
				sb.WriteString(fmt.Sprintf("%d. ", i+1))
			} else {
				sb.WriteString("- ")
			}

			if elem.Kind() == reflect.Struct {
				// For struct slices, render inline or recursively
				sb.WriteString(renderStructInline(elem))
			} else {
				sb.WriteString(fmt.Sprintf("%v", elem.Interface()))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	default:
		// Default: render each element
		if elemKind == reflect.Struct {
			for i := 0; i < field.Len(); i++ {
				elem := field.Index(i)
				if elem.Kind() == reflect.Ptr && !elem.IsNil() {
					elem = elem.Elem()
				}
				sb.WriteString(JSONToMarkdown(elem.Interface(), depth+1))
			}
		} else {
			// Slice of primitives
			sb.WriteString(fmt.Sprintf("**%s:** ", fieldName))
			var items []string
			for i := 0; i < field.Len(); i++ {
				items = append(items, fmt.Sprintf("%v", field.Index(i).Interface()))
			}
			sb.WriteString(strings.Join(items, ", "))
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// renderStructField renders a struct field
func renderStructField(field reflect.Value, fieldType reflect.StructField, mdTag string, depth int) string {
	if field.Kind() == reflect.Ptr && field.IsNil() {
		return ""
	}

	var sb strings.Builder
	fieldName := getFieldName(fieldType, mdTag)

	switch mdTag {
	case "inline":
		sb.WriteString(fmt.Sprintf("**%s:** ", fieldName))
		sb.WriteString(renderStructInline(field))
		sb.WriteString("\n")
	default:
		// Recursively render struct
		sb.WriteString(JSONToMarkdown(field.Interface(), depth+1))
	}

	return sb.String()
}

// renderMapField renders a map field
func renderMapField(field reflect.Value, fieldType reflect.StructField, mdTag string, depth int) string {
	if field.Len() == 0 {
		return ""
	}

	var sb strings.Builder
	fieldName := getFieldName(fieldType, mdTag)

	sb.WriteString(fmt.Sprintf("**%s:**\n", fieldName))
	for _, key := range field.MapKeys() {
		value := field.MapIndex(key)
		sb.WriteString(fmt.Sprintf("- **%v**: ", key.Interface()))

		if value.Kind() == reflect.Struct || (value.Kind() == reflect.Ptr && !value.IsNil() && value.Elem().Kind() == reflect.Struct) {
			sb.WriteString(renderStructInline(value))
		} else {
			sb.WriteString(fmt.Sprintf("%v", value.Interface()))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

// renderStructInline renders a struct as inline text (for lists)
func renderStructInline(field reflect.Value) string {
	if field.Kind() == reflect.Ptr && !field.IsNil() {
		field = field.Elem()
	}
	if field.Kind() != reflect.Struct {
		return fmt.Sprintf("%v", field.Interface())
	}

	var parts []string
	typ := field.Type()

	for i := 0; i < field.NumField(); i++ {
		f := field.Field(i)
		ft := typ.Field(i)

		mdTag := ft.Tag.Get("md")
		if mdTag == "-" || mdTag == "title" {
			continue
		}

		if !isZeroValue(f) {
			fieldName := getFieldName(ft, mdTag)
			parts = append(parts, fmt.Sprintf("%s: %v", fieldName, f.Interface()))
		}
	}

	return strings.Join(parts, ", ")
}

// getFieldName returns the display name for a field
func getFieldName(fieldType reflect.StructField, mdTag string) string {
	// Check json tag for name
	jsonTag := fieldType.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		// Get just the name part (before comma)
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}
		if jsonTag != "" {
			return capitalizeFirst(jsonTag)
		}
	}

	// Fallback to field name
	return capitalizeFirst(fieldType.Name)
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
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
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return false // structs are never zero for this purpose
	default:
		return false
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
