package reportdoc

import (
	"strings"
	"testing"
)

func summaryFixture(statuses ...string) EvaluationRun {
	run := EvaluationRun{Status: "completed", Total: len(statuses), Completed: len(statuses)}
	run.Suite.Repeats = 1
	run.Suite.Cases = []struct{ ID, Name string }{{"s", "sample"}}
	for index, state := range statuses {
		id := index + 1
		run.Suite.ProfileIDs = append(run.Suite.ProfileIDs, id)
		r := Result{Status: "completed", Repeat: 1, DurationMs: int64(100 * (index + 1)), Checks: []Check{{ID: "facts", Label: "事实一致", Status: state, Source: "模型辅助评价"}}}
		r.Profile.ID = id
		r.Profile.Name = []string{"A", "B", "C"}[index]
		r.Sample.ID = "s"
		run.Results = append(run.Results, r)
	}
	return run
}
func TestSummaryQualityTakesPriorityOverSpeed(t *testing.T) {
	run := summaryFixture("satisfied", "not_satisfied")
	run.Results[0].DurationMs = 900
	run.Results[1].DurationMs = 100
	s := summarizeEvaluation(run)
	if s.PreferredID != 1 || !strings.Contains(s.Reason, "唯一") && !strings.Contains(s.Recommendation, "唯一") {
		t.Fatal("did not recommend qualified model")
	}
	if !strings.Contains(strings.Join(s.Findings, ""), "更快，但未满足") {
		t.Fatal("missing speed-quality tradeoff")
	}
	if !strings.Contains(s.Reason, "B 在") || !strings.Contains(s.Reason, "事实一致") {
		t.Fatal("opening reason lacks actual competing model failure")
	}
}
func TestSummaryUnknownAndIncompleteDoNotBecomeWinner(t *testing.T) {
	for _, run := range []EvaluationRun{summaryFixture("satisfied", "unknown"), summaryFixture("unknown", "unknown")} {
		s := summarizeEvaluation(run)
		if s.PreferredID != 0 || !strings.Contains(s.Recommendation, "暂不能") {
			t.Fatal("unknown got final winner")
		}
	}
	run := summaryFixture("satisfied", "satisfied")
	run.Results = run.Results[:1]
	run.Completed = 1
	if s := summarizeEvaluation(run); s.PreferredID != 0 || !strings.Contains(s.Recommendation, "未完成") {
		t.Fatal("partial execution got winner")
	}
}
func TestSummaryAllFailHasNoRecommendedModel(t *testing.T) {
	run := summaryFixture("not_satisfied", "not_satisfied")
	s := summarizeEvaluation(run)
	if s.PreferredID != 0 || !strings.Contains(s.Recommendation, "没有模型满足") {
		t.Fatal("selected a failing model")
	}
}
func TestSummaryQualifiedModelsUseExplicitSpeedCriterion(t *testing.T) {
	run := summaryFixture("satisfied", "satisfied")
	s := summarizeEvaluation(run)
	if s.PreferredID != 1 || !strings.Contains(s.Recommendation, "响应速度") {
		t.Fatal("unexplained ranking")
	}
	run.Results[1].DurationMs = run.Results[0].DurationMs
	s = summarizeEvaluation(run)
	if s.PreferredID != 0 || !strings.Contains(s.Recommendation, "不足以") {
		t.Fatal("forced winner for tied results")
	}
}
func TestSummaryRejectsInvalidCoverage(t *testing.T) {
	run := summaryFixture("satisfied", "satisfied")
	run.Results[1] = run.Results[0]
	if summarizeEvaluation(run).PreferredID != 0 {
		t.Fatal("duplicate results hid missing model")
	}
	run = summaryFixture("satisfied")
	run.Results[0].Status = "running"
	if summarizeEvaluation(run).PreferredID != 0 {
		t.Fatal("running result counted as pass")
	}
}

func TestSingleModelDoesNotClaimComparativeWinner(t *testing.T) {
	s := summarizeEvaluation(summaryFixture("satisfied"))
	if s.PreferredID != 0 || !strings.Contains(s.Recommendation, "仅测评") {
		t.Fatal("single candidate claimed best among others")
	}
}
