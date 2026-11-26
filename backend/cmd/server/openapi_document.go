package main

import (
	"net/url"
	"strconv"
	"strings"
)

func openAPIBaseURL(override, fallback string) string {
	baseURL := strings.TrimSpace(override)
	if baseURL == "" {
		baseURL = strings.TrimSpace(fallback)
	}
	return strings.TrimRight(baseURL, "/")
}

func openAPIReference(document map[string]any, reference string) (any, bool) {
	if !strings.HasPrefix(reference, "#/") {
		return nil, false
	}
	fragment, err := url.PathUnescape(strings.TrimPrefix(reference, "#/"))
	if err != nil {
		return nil, false
	}
	var current any = document
	for _, part := range strings.Split(fragment, "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch value := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = value[part]
			if !exists {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func expandOpenAPIReferences(value any, document map[string]any, seen map[string]bool, depth int) any {
	if depth > 24 {
		return value
	}
	switch node := value.(type) {
	case map[string]any:
		if reference, ok := node["$ref"].(string); ok && !seen[reference] {
			if target, exists := openAPIReference(document, reference); exists {
				if definition, ok := target.(map[string]any); ok {
					next := make(map[string]bool, len(seen)+1)
					for key, value := range seen {
						next[key] = value
					}
					next[reference] = true
					merged := make(map[string]any, len(definition)+len(node))
					for key, value := range definition {
						merged[key] = value
					}
					for key, value := range node {
						if key != "$ref" {
							merged[key] = value
						}
					}
					merged["x-pulse-ref"] = reference
					return expandOpenAPIReferences(merged, document, next, depth+1)
				}
			}
		}
		out := make(map[string]any, len(node))
		for key, child := range node {
			if key == "example" || key == "examples" || key == "default" {
				out[key] = child
			} else {
				out[key] = expandOpenAPIReferences(child, document, seen, depth+1)
			}
		}
		return out
	case []any:
		out := make([]any, len(node))
		for index, child := range node {
			out[index] = expandOpenAPIReferences(child, document, seen, depth+1)
		}
		return out
	default:
		return value
	}
}

func openAPIDocumentationSchema(document map[string]any, path string, operation map[string]any) map[string]any {
	out := expandOpenAPIReferences(operation, document, map[string]bool{}, 0).(map[string]any)
	if _, exists := out["security"]; !exists {
		if security, exists := document["security"]; exists {
			out["security"] = security
		}
	}
	parameters := []any{}
	if paths, ok := document["paths"].(map[string]any); ok {
		if item, ok := paths[path].(map[string]any); ok {
			if common, ok := item["parameters"].([]any); ok {
				parameters = append(parameters, common...)
			}
		}
	}
	if local, ok := out["parameters"].([]any); ok {
		parameters = append(parameters, local...)
	}
	if len(parameters) > 0 {
		merged, indexes := []any{}, map[string]int{}
		for _, parameter := range parameters {
			resolved := expandOpenAPIReferences(parameter, document, map[string]bool{}, 0)
			if definition, ok := resolved.(map[string]any); ok {
				name, _ := definition["name"].(string)
				location, _ := definition["in"].(string)
				key := name + ":" + location
				if index, exists := indexes[key]; exists {
					merged[index] = resolved
					continue
				}
				indexes[key] = len(merged)
			}
			merged = append(merged, resolved)
		}
		out["parameters"] = merged
	}
	meta := map[string]any{"path": path}
	if info, ok := document["info"].(map[string]any); ok {
		meta["title"], meta["version"] = info["title"], info["version"]
	}
	descriptions := map[string]any{}
	tagOrder := []any{}
	if tags, ok := document["tags"].([]any); ok {
		for _, value := range tags {
			if tag, ok := value.(map[string]any); ok {
				if name, ok := tag["name"].(string); ok {
					descriptions[name] = tag["description"]
					tagOrder = append(tagOrder, name)
				}
			}
		}
	}
	meta["tagDescriptions"] = descriptions
	meta["tagOrder"] = tagOrder
	if components, ok := document["components"].(map[string]any); ok {
		if schemes, ok := components["securitySchemes"]; ok {
			meta["securitySchemes"] = schemes
		}
	}
	if schemes, ok := document["securityDefinitions"]; ok {
		meta["securitySchemes"] = schemes
	}
	out["x-pulse-document"] = meta
	return out
}
