package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func videoAnswer(count int, d float64) string {
	shots := []map[string]any{}
	for i := 0; i < count; i++ {
		shots = append(shots, map[string]any{"duration": d, "visual": "杯子", "voiceover": "450ml", "subtitle": "280g"})
	}
	b, _ := json.Marshal(map[string]any{"shots": shots})
	return string(b)
}
func TestStoryboardChecks(t *testing.T) {
	suite := Suite{Scenario: "short_video", ShotCount: 6, DurationSeconds: 30}
	for _, c := range ObjectiveChecks(suite, Sample{}, videoAnswer(6, 5)) {
		if c.Status != "satisfied" {
			t.Fatal(c)
		}
	}
	for _, test := range []struct{ answer, id string }{{videoAnswer(6, 7.5), "duration"}, {videoAnswer(5, 6), "shot_count"}, {`{"shots":[{"duration":30}]}`, "shot_fields"}, {"plain text", "json"}} {
		t.Run(test.id, func(t *testing.T) {
			bad := false
			for _, c := range ObjectiveChecks(suite, Sample{}, test.answer) {
				if c.ID == test.id && c.Status == "not_satisfied" {
					bad = true
				}
			}
			if !bad {
				t.Fatal(test)
			}
		})
	}
}
func TestUnknownExtractionAndReferenceIsolation(t *testing.T) {
	suite := Suite{Scenario: "extraction", OutputFields: []string{"name", "price"}}
	sample := Sample{Input: "张三", Expected: `{"name":"张三","price":null}`}
	for _, c := range ObjectiveChecks(suite, sample, `{"name":"张三","price":null}`) {
		if c.Status != "satisfied" {
			t.Fatal(c)
		}
	}
	_, user := GeneratePrompt(suite, sample)
	if strings.Contains(user, "price") || strings.Contains(user, "null") {
		t.Fatal("reference leaked to candidate")
	}
}
func TestJudgeEvidenceMustExist(t *testing.T) {
	for _, response := range []string{`{"checks":[{"id":"facts","status":"satisfied","evidence":"虚构证据"}]}`, `{"checks":[{"id":"facts","status":"satisfied"}]}`, "bad JSON"} {
		checks := ApplyJudgement(SemanticChecks(Suite{}), "实际原文", response)
		if checks[0].Status != "unknown" {
			t.Fatal(checks)
		}
	}
	good := ApplyJudgement(SemanticChecks(Suite{}), "实际原文", `{"checks":[{"id":"facts","status":"not_satisfied","actual":"错误","evidence":"实际原文"}]}`)
	if good[0].Status != "not_satisfied" {
		t.Fatal(good)
	}
}
func TestSeparateEvidenceFragmentsValidated(t *testing.T) {
	checks := ApplyJudgement(SemanticChecks(Suite{}), "容量450ml。重量280g。", `{"checks":[{"id":"facts","status":"satisfied","evidence":["容量450ml","重量280g"]}]}`)
	if checks[0].Status != "satisfied" {
		t.Fatal(checks)
	}
	checks = ApplyJudgement(SemanticChecks(Suite{}), "容量450ml。重量280g。", `{"checks":[{"id":"facts","status":"satisfied","evidence":["容量450ml","保温24小时"]}]}`)
	if checks[0].Status != "unknown" {
		t.Fatal(checks)
	}
}
func TestInvalidExtractionReferenceRejected(t *testing.T) {
	s := Suite{ProjectID: 1, Name: "test", Goal: "test", Scenario: "extraction", ProfileIDs: []int{1}, Repeats: 1, OutputFields: []string{"name"}, Cases: []Sample{{ID: "x", Input: "data", Expected: "bad json"}}}
	if Validate(s) == nil {
		t.Fatal("silently ignored invalid expected output")
	}
}

type fakeGateway struct{ failCandidate bool }

func (g fakeGateway) Candidate(_ context.Context, id int) (Candidate, error) {
	return Candidate{ID: id, Name: "真实模型"}, nil
}
func (g fakeGateway) DefaultJudge(context.Context) (int, error) { return 0, nil }
func (g fakeGateway) Complete(_ context.Context, id int, _, _ string) (Completion, error) {
	if g.failCandidate || id == 2 {
		return Completion{}, errors.New("upstream failed")
	}
	return Completion{Answer: "符合已知资料", InputTokens: 10, OutputTokens: 5, UsageSource: "provider"}, nil
}
func TestCallFailureNotScoredAsQuality(t *testing.T) {
	r := EvaluateSample(context.Background(), fakeGateway{true}, Suite{}, 1, Sample{}, 1)
	if r.Status != "call_failed" || len(r.Checks) != 0 {
		t.Fatal(r)
	}
}
func TestJudgeFailurePreservesCandidateAndUnknown(t *testing.T) {
	r := EvaluateSample(context.Background(), fakeGateway{}, Suite{Scenario: "knowledge", JudgeProfileID: 2}, 1, Sample{}, 1)
	if r.Status != "completed" || r.Answer == "" || r.Checks[0].Status != "unknown" {
		t.Fatal(r)
	}
}
func TestConclusionNeverForcesPendingWinner(t *testing.T) {
	run := Run{Status: "completed", Total: 1, Completed: 1, Suite: Suite{ProfileIDs: []int{1}, Repeats: 1, Cases: []Sample{{}}}, Results: []Result{{Profile: Candidate{ID: 1, Name: "A"}, Status: "completed", Checks: []Check{{Status: "unknown"}}}}}
	if !strings.Contains(Conclusion(run), "待复核") {
		t.Fatal(Conclusion(run))
	}
	run.Results[0].Checks[0].Status = "satisfied"
	if !strings.Contains(Conclusion(run), "全部达标") {
		t.Fatal(Conclusion(run))
	}
}
