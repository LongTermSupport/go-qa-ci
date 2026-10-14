package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// spellingProblems reports every key whose spelling differs from the documented one only by case.
// The TOML decoder matches field names case-insensitively, so without this check `[Tools] FMT = false`
// would be accepted and a later `[tools] fmt = true` would silently win.
func spellingProblems(raw map[string]any, schema reflect.Type) []string {
	var problems []string
	walkKeys(raw, schema, "", "", &problems)
	slices.Sort(problems)
	return problems
}

func walkKeys(value any, schema reflect.Type, given, canon string, problems *[]string) {
	for schema.Kind() == reflect.Pointer {
		schema = schema.Elem()
	}
	switch typed := value.(type) {
	case map[string]any:
		switch schema.Kind() {
		case reflect.Struct:
			walkStruct(typed, schema, given, canon, problems)
		case reflect.Map:
			for _, v := range typed {
				walkKeys(v, schema.Elem(), given, canon, problems)
			}
		}
	case []any:
		if schema.Kind() == reflect.Slice {
			for _, v := range typed {
				walkKeys(v, schema.Elem(), given, canon, problems)
			}
		}
	}
}

func walkStruct(raw map[string]any, schema reflect.Type, given, canon string, problems *[]string) {
	fields := map[string]reflect.Type{}
	for field := range schema.Fields() {
		if name := field.Tag.Get("toml"); name != "" {
			fields[name] = field.Type
		}
	}
	for key, value := range raw {
		childGiven := joinPath(given, key)
		childCanon := joinPath(canon, key)
		fieldType, exact := fields[key]
		if !exact {
			for name, candidate := range fields {
				if strings.EqualFold(name, key) {
					*problems = append(*problems, fmt.Sprintf("key %q must be spelt %q", childGiven, joinPath(canon, name)))
					childCanon = joinPath(canon, name)
					fieldType = candidate
					break
				}
			}
		}
		if fieldType != nil {
			walkKeys(value, fieldType, childGiven, childCanon, problems)
		}
	}
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}
