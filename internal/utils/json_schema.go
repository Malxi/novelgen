package utils

import (
	"reflect"
	"strings"
)

// StructToJSONSchemaObject converts a Go value into a minimal JSON Schema
// object suitable for SDKs that support structured JSON output.
func StructToJSONSchemaObject(v interface{}) map[string]interface{} {
	if v == nil {
		return map[string]interface{}{"type": "object"}
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return schemaForType(t, 0)
}

func schemaForType(t reflect.Type, depth int) map[string]interface{} {
	if depth > 8 {
		return map[string]interface{}{"type": "object"}
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		return schemaForStruct(t, depth+1)
	case reflect.Slice, reflect.Array:
		return map[string]interface{}{"type": "array", "items": schemaForType(t.Elem(), depth+1)}
	case reflect.Map:
		return map[string]interface{}{"type": "object", "additionalProperties": schemaForType(t.Elem(), depth+1)}
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	default:
		return map[string]interface{}{"type": "object"}
	}
}

func schemaForStruct(t reflect.Type, depth int) map[string]interface{} {
	properties := map[string]interface{}{}
	required := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, optional, ok := jsonFieldName(field)
		if !ok {
			continue
		}
		fieldSchema := schemaForType(field.Type, depth+1)
		if desc := strings.TrimSpace(field.Tag.Get("desc")); desc != "" {
			fieldSchema["description"] = desc
		}
		properties[name] = fieldSchema
		if !optional {
			required = append(required, name)
		}
	}
	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}
	name := field.Name
	optional := false
	if tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			name = parts[0]
		}
		for _, part := range parts[1:] {
			if part == "omitempty" {
				optional = true
			}
		}
	}
	return name, optional, true
}
