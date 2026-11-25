package reportdoc

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inspect(t *testing.T, data []byte) string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	document := ""
	for _, file := range z.File {
		f, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(f)
		f.Close()
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, err = decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid %s: %v", file.Name, err)
			}
		}
		if file.Name == "word/document.xml" {
			document = string(raw)
		}
	}
	if !strings.Contains(document, "w:tblHeader") || !strings.Contains(document, "w:tblBorders") {
		t.Fatal("missing readable table structure")
	}
	return document
}
func TestEvaluationReportLeadsWithRecommendationAndKeepsKeyEvidence(t *testing.T) {
	run := EvaluationRun{ID: 5, Status: "completed", Completed: 2, Total: 2, CreatedAt: "2026-11-24T00:00:00Z", Conclusion: "第二个模型需复核"}
	run.Suite.Name = "通勤杯测评"
	run.Suite.ProfileIDs = []int{1, 2}
	run.Suite.Repeats = 1
	run.Suite.Cases = []struct{ ID, Name string }{{"s1", "推广脚本"}}
	for _, id := range []int{1, 2} {
		r := Result{Repeat: 1, Status: "completed", Answer: "原始输出 & <tag> 中文", UsageSource: "provider", DurationMs: 100}
		r.Profile.ID = id
		r.Profile.Name = fmt.Sprintf("候选模型%d", id)
		r.Profile.Deployment = "deployment"
		r.Sample.ID = "s1"
		r.Sample.Name = "推广脚本"
		r.Checks = []Check{{ID: "facts", Label: "事实检查", Status: "satisfied", Actual: "资料一致", Source: "模型辅助评价", Evidence: "原始输出 & <tag> 中文"}}
		if id == 2 {
			r.Checks[0].Status = "not_satisfied"
		}
		run.Results = append(run.Results, r)
	}
	raw, _ := json.Marshal(run)
	data, err := Evaluation(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc := inspect(t, data)
	for _, term := range []string{"本次优先推荐 候选模型1", "测试中发现了什么", "deployment", "原始输出 &amp; &lt;tag&gt; 中文", "辅助评审"} {
		if !strings.Contains(doc, term) {
			t.Fatalf("missing %s", term)
		}
	}
	if strings.Index(doc, "本次优先推荐") > strings.Index(doc, "<w:tbl>") {
		t.Fatal("recommendation buried after tables")
	}
	if directory := os.Getenv("PULSE_REPORT_SAMPLES_DIR"); directory != "" {
		os.MkdirAll(directory, 0755)
		if err := os.WriteFile(filepath.Join(directory, "模型测评分析报告.docx"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
func TestProjectReportIncludesBlockedSteps(t *testing.T) {
	raw := []byte(`{"id":7,"status":"completed","task":{"name":"云盘验收","baseUrl":"http://cloud-disk:8082"},"summary":{"total":2,"passed":1,"blocked":1},"results":[{"name":"登录","status":"passed","method":"POST","url":"/login","statusCode":200,"checks":[{"type":"json_nonempty","path":"token","expected":"非空","actual":"已脱敏","passed":true}]},{"name":"上传","status":"blocked","error":"依赖步骤失败"}]}`)
	data, err := Project(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc := inspect(t, data)
	if !strings.Contains(doc, "阻塞") || !strings.Contains(doc, "依赖步骤失败") {
		t.Fatal("lost blockers")
	}
	if directory := os.Getenv("PULSE_REPORT_SAMPLES_DIR"); directory != "" {
		os.MkdirAll(directory, 0755)
		os.WriteFile(filepath.Join(directory, "项目测试验收报告.docx"), data, 0644)
	}
}
