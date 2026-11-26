package main

import (
	"encoding/json"
	"testing"
)

func TestOpenAPIBaseURLPrefersUserTargetAndFallsBackToDocument(t *testing.T) {
	if got := openAPIBaseURL(" http://cloud-disk:8082/ ", "http://127.0.0.1:8082"); got != "http://cloud-disk:8082" {
		t.Fatalf("user target was not used: %q", got)
	}
	if got := openAPIBaseURL("", "http://127.0.0.1:8082/"); got != "http://127.0.0.1:8082" {
		t.Fatalf("document server fallback was not used: %q", got)
	}
}

func TestOpenAPIDocumentationPreservesReferencedSchemasAndGroupDescriptions(t *testing.T) {
	var document map[string]any
	json.Unmarshal([]byte(`{"info":{"title":"Example","version":"1"},"tags":[{"name":"用户","description":"注册和登录"}],"security":[{"bearer":[]}],"components":{"schemas":{"Input":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}},"paths":{"/users/{id}":{"parameters":[{"name":"id","in":"path","schema":{"type":"integer"}}]}}}`), &document)
	operation := map[string]any{"tags": []any{"用户"}, "requestBody": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Input"}}}}}
	result := openAPIDocumentationSchema(document, "/users/{id}", operation)
	schema := result["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	if schema["type"] != "object" || schema["properties"] == nil || result["parameters"] == nil || result["security"] == nil {
		t.Fatal("import lost usable contract fields")
	}
	meta := result["x-pulse-document"].(map[string]any)
	if meta["title"] != "Example" || meta["tagDescriptions"].(map[string]any)["用户"] != "注册和登录" {
		t.Fatal("import lost document labels")
	}
	tagOrder, ok := meta["tagOrder"].([]any)
	if !ok || len(tagOrder) != 1 || tagOrder[0] != "用户" {
		t.Fatal("import lost the declared tag order")
	}
}

func TestOpenAPIReferenceCyclesStopAndExamplesRemainData(t *testing.T) {
	document := map[string]any{"components": map[string]any{"schemas": map[string]any{"Node": map[string]any{"type": "object", "properties": map[string]any{"next": map[string]any{"$ref": "#/components/schemas/Node"}}}}}}
	value := map[string]any{"$ref": "#/components/schemas/Node", "example": map[string]any{"$ref": "#/components/schemas/Node"}}
	result := expandOpenAPIReferences(value, document, map[string]bool{}, 0).(map[string]any)
	if result["properties"].(map[string]any)["next"].(map[string]any)["$ref"] != "#/components/schemas/Node" || result["example"].(map[string]any)["$ref"] != "#/components/schemas/Node" {
		t.Fatal("cycle or example was expanded incorrectly")
	}
}
