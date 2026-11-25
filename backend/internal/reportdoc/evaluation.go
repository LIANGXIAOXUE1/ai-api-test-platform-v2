package reportdoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

func Evaluation(raw []byte) ([]byte, error) {
	var run EvaluationRun
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, err
	}
	analysis := summarizeEvaluation(run)
	d := &Document{Portrait: true}
	d.Title("模型测评总结与选型建议")
	d.Lead(analysis.Recommendation)
	d.Text(analysis.Reason)
	d.Heading("测试中发现了什么")
	for _, finding := range analysis.Findings {
		d.Text(finding)
	}
	d.Heading("关键测试结果对比")
	rows := [][]string{}
	for _, s := range analysis.Models {
		average := "暂无可比较耗时"
		if s.average() > 0 {
			average = fmt.Sprintf("%d ms", s.average())
		}
		advice := "需要处理不满足项"
		if s.qualified() {
			advice = "满足本次全部要求"
		}
		if s.Pending+s.Missing > 0 {
			advice = "补齐样本或人工复核"
		}
		if s.CallFailed > 0 {
			advice = "检查调用失败，再验证"
		}
		if s.ID == analysis.PreferredID {
			advice = "本次优先推荐"
		}
		name := s.Name
		if s.Deployment != "" && s.Deployment != s.Name {
			name += "\n部署名：" + s.Deployment
		}
		rows = append(rows, []string{name, fmt.Sprintf("满足 %d / 不满足 %d\n待复核 %d / 调用失败 %d\n未完成 %d", s.Passed, s.Failed, s.Pending, s.CallFailed, s.Missing), average, advice})
	}
	d.Table([]string{"候选模型", "样本检查结果", "平均成功请求耗时", "选型建议"}, rows)
	issues := 0
	for _, s := range analysis.Models {
		for _, r := range s.Results {
			for _, c := range r.Checks {
				if (c.Status != "not_satisfied" && c.Status != "unknown") || issues >= 4 {
					continue
				}
				if issues == 0 {
					d.Heading("问题依据节选")
				}
				issues++
				d.Text(fmt.Sprintf("%s · %s · %s：%s。%s", s.Name, r.Sample.Name, c.Label, status(c.Status), compact(c.Actual, 140)))
				if c.Evidence != "" {
					d.Text("对应原文：“" + compact(c.Evidence, 100) + "”")
				}
			}
		}
	}
	d.Heading("下一步怎么选择")
	if analysis.PreferredID > 0 {
		d.Text("优先对推荐候选继续验证实际业务样本；其他候选先处理报告指出的问题，再按同一标准比较。")
	} else {
		d.Text("先完成待复核项或解决不满足项，再进行选型。当前报告不强行指定一个最优模型。")
	}
	d.Text("本次推荐仅适用于该任务、样本和配置。语义判断来自模型辅助评价或人工复核；没有渠道价格依据，不根据Token估算费用。")
	d.Heading("本次测试的范围与记录")
	d.Text(fmt.Sprintf("任务：%s\n客户目标：%s\n已知资料：%s\n验收要求：%s", run.Suite.Name, run.Suite.Goal, compact(run.Suite.Facts, 300), compact(run.Suite.Requirements, 300)))
	d.Text(fmt.Sprintf("执行记录 %d　时间 %s\n状态：%s　完成 %d / %d 个模型样本", run.ID, timestamp(run.CreatedAt), status(run.Status), run.Completed, run.Total))
	if run.Error != "" {
		d.Text("执行说明：" + run.Error)
	}
	if len(run.Results) > 0 {
		usageLines := []string{}
		for _, s := range analysis.Models {
			if len(s.Results) > 0 {
				usageLines = append(usageLines, s.Name+"："+usage(s.Results[0]))
			}
		}
		d.Text("首个样本用量记录：" + strings.Join(usageLines, "；"))
	}
	d.Text("完整原文、逐项检查和复核依据可在平台的模型测评执行记录中查看。本报告聚焦推荐结论与关键发现。")
	return d.Bytes()
}
