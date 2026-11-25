package reportdoc

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Check struct{ ID, Label, Status, Expected, Actual, Evidence, Source string }
type Result struct {
	Profile struct {
		ID               int
		Name, Deployment string
	}
	Sample                                        struct{ ID, Name, Input, Expected string }
	Repeat                                        int
	Status, Answer, Error, UsageSource, JudgeName string
	DurationMs                                    int64
	InputTokens, OutputTokens                     int
	Checks                                        []Check
}
type EvaluationRun struct {
	ID                                   int64
	Status, CreatedAt, Conclusion, Error string
	Total, Completed                     int
	Suite                                struct {
		Name, Goal, Facts, Requirements string
		ProfileIDs                      []int
		Repeats                         int
		Cases                           []struct{ ID, Name string }
	}
	Results []Result
}

func timestamp(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05 +08:00")
}
func status(value string) string {
	if text, ok := map[string]string{"satisfied": "满足", "not_satisfied": "不满足", "unknown": "待复核", "passed": "通过", "failed": "失败", "blocked": "阻塞", "not_run": "未执行", "call_failed": "调用失败", "completed": "已完成", "queued": "排队中", "running": "执行中", "cancelled": "已停止", "interrupted": "已中断"}[value]; ok {
		return text
	}
	return value
}
func verdict(r Result) string {
	if r.Status == "call_failed" {
		return "调用失败"
	}
	unknown := len(r.Checks) == 0
	for _, c := range r.Checks {
		if c.Status == "not_satisfied" {
			return "不满足"
		}
		if c.Status != "satisfied" {
			unknown = true
		}
	}
	if unknown {
		return "待复核"
	}
	return "满足"
}
func usage(r Result) string {
	source := map[string]string{"provider": "供应商用量", "estimated": "估算用量"}[r.UsageSource]
	if source == "" {
		return "用量未知"
	}
	return fmt.Sprintf("输入 %d / 输出 %d（%s）", r.InputTokens, r.OutputTokens, source)
}

func compact(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "……（完整内容见执行记录）"
}

func Project(raw []byte) ([]byte, error) {
	var run struct {
		ID                       int64
		Status, CreatedAt, Error string
		Task                     struct{ Name, BaseURL, Goal string }
		Summary                  struct {
			Total, Passed, Failed, Blocked, NotRun int
			Conclusion                             string
		}
		Results []struct {
			Name, Status, Method, URL, Error, ResponseBody string
			StatusCode                                     int
			DurationMs                                     int64
			Checks                                         []struct {
				Type, Path, Expected, Actual string
				Passed                       bool
			}
		}
	}
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, err
	}
	d := &Document{}
	d.Title("项目业务测试验收报告")
	d.Text(fmt.Sprintf("任务：%s\n目标：%s\n环境：%s\n记录：%d　时间：%s", run.Task.Name, run.Task.Goal, run.Task.BaseURL, run.ID, timestamp(run.CreatedAt)))
	d.Heading("执行结论")
	d.Text(fmt.Sprintf("状态：%s。共 %d 步，通过 %d，失败 %d，阻塞 %d，未执行 %d。", status(run.Status), run.Summary.Total, run.Summary.Passed, run.Summary.Failed, run.Summary.Blocked, run.Summary.NotRun))
	d.Text(run.Summary.Conclusion)
	if run.Error != "" {
		d.Text("执行说明：" + run.Error)
	}
	if run.Summary.Failed+run.Summary.Blocked+run.Summary.NotRun > 0 || run.Status != "completed" {
		d.Text("本次存在失败、阻塞、未执行或未完成步骤，请处理问题后再次验证。")
	} else {
		d.Text("本次已覆盖步骤按保存的断言通过。结果仅代表本次覆盖的业务范围。")
	}
	d.Heading("业务步骤结果")
	rows := [][]string{}
	for _, step := range run.Results {
		checks := []string{}
		for _, c := range step.Checks {
			state := "失败"
			if c.Passed {
				state = "通过"
			}
			checks = append(checks, fmt.Sprintf("%s %s：%s\n预期 %s；实际 %s", c.Type, c.Path, state, c.Expected, c.Actual))
		}
		rows = append(rows, []string{step.Name, step.Method + " " + step.URL, fmt.Sprintf("%s\nHTTP %d / %d ms", status(step.Status), step.StatusCode, step.DurationMs), strings.Join(checks, "\n") + "\n" + step.Error})
	}
	d.Table([]string{"业务步骤", "请求接口", "结果与耗时", "检查依据与问题"}, rows)
	d.Heading("问题定位与处理建议")
	issues := 0
	for _, step := range run.Results {
		if step.Status == "passed" {
			continue
		}
		issues++
		d.Text(step.Name + "：" + status(step.Status) + "\n" + step.Error)
	}
	if issues == 0 {
		d.Text("已记录步骤中未发现失败项。")
	}
	d.Text("按失败步骤的预期与实际响应定位；依赖阻塞先处理前置步骤。预期被拒绝的负向测试按断言判定，不因HTTP错误码自动视为失败。")
	d.Heading("响应证据")
	for _, step := range run.Results {
		d.Heading(step.Name)
		d.Text(step.ResponseBody)
	}
	d.Text("本报告不覆盖未选择的接口、页面交互和独立性能或安全专项。只读用户导出沿用只读脱敏结果。")
	return d.Bytes()
}
