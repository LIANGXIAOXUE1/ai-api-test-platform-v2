package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentUploadErrorsAreExplicit(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		data       []byte
		status     int
		code       string
	}{
		{"empty", "empty.txt", nil, 400, "document_empty"},
		{"unsupported", "archive.zip", []byte("test"), 400, "document_format_unsupported"},
		{"blank text", "blank.md", []byte(" \n\t"), 400, "document_empty_text"},
		{"encoding", "bad.txt", []byte{0xff, 0xfe, 0x00}, 400, "document_encoding_invalid"},
		{"binary", "binary.txt", []byte{'a', 0, 'b'}, 400, "document_binary_content"},
		{"corrupt docx", "broken.docx", []byte("not a zip"), 400, "document_parse_failed"},
		{"oversize", "large.txt", bytes.Repeat([]byte("a"), (12<<20)+1), 413, "document_too_large"},
		{"text limit", "long.md", []byte(strings.Repeat("文", 120001)), 413, "document_text_too_long"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			writer.WriteField("projectId", "1")
			part, _ := writer.CreateFormFile("file", tc.file)
			part.Write(tc.data)
			writer.Close()
			r := httptest.NewRequest("POST", "/api/documents", &body)
			r.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			(&server{}).documentsHandler(w, r)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
			var data map[string]any
			json.Unmarshal(w.Body.Bytes(), &data)
			if w.Code != tc.status || data["code"] != tc.code || data["error"] == "" || len(data["nextSteps"].([]any)) == 0 {
				t.Fatalf("unclear upload response: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGenerationOutcomeDistinguishesFailureAndDrafts(t *testing.T) {
	for _, tc := range []struct {
		result       businessCasesResult
		status, code string
	}{
		{businessCasesResult{}, "needs_information", "knowledge_missing_expectations"},
		{businessCasesResult{Cases: []BusinessTestCase{{Status: "needs_configuration"}}}, "drafts_only", "knowledge_no_executable_cases"},
		{businessCasesResult{Cases: []BusinessTestCase{{Status: "ready_for_review"}, {Status: "needs_configuration"}}}, "partial_generated", "knowledge_partial_generation"},
		{businessCasesResult{Cases: []BusinessTestCase{{Status: "ready_for_review"}}}, "generated", "knowledge_generated"},
	} {
		status, code, message, steps, _, _ := knowledgeGenerationFeedback(tc.result)
		if status != tc.status || code != tc.code || message == "" {
			t.Fatal("wrong generation result")
		}
		if status != "generated" && len(steps) == 0 {
			t.Fatal("missing recovery steps")
		}
	}
	code, message := assessmentProblem(documentAssessment{Status: "irrelevant"})
	if code != "knowledge_document_not_relevant" || !strings.Contains(message, "无关") {
		t.Fatal("irrelevant material lacks clear reason")
	}
}
