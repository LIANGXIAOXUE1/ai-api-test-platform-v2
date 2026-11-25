package reportdoc

import (
	"fmt"
	"sort"
	"strings"
)

type modelSummary struct {
	ID                                                            int
	Name, Deployment                                              string
	Expected, Passed, Failed, Pending, CallFailed, Missing, Calls int
	Duration                                                      int64
	Results                                                       []Result
}

func (s modelSummary) average() int64 {
	if s.Calls == 0 {
		return 0
	}
	return s.Duration / int64(s.Calls)
}
func (s modelSummary) qualified() bool {
	return s.Expected > 0 && s.Passed == s.Expected && s.Missing == 0 && s.Pending == 0 && s.Failed == 0 && s.CallFailed == 0
}

type evaluationSummary struct {
	Recommendation, Reason string
	Findings               []string
	Models                 []modelSummary
	PreferredID            int
}

// Recommendation uses recorded checks first. Speed only breaks ties among
// fully qualified candidates; unknown and missing samples never count as pass.
func summarizeEvaluation(run EvaluationRun) evaluationSummary {
	output := evaluationSummary{}
	samples := map[string]bool{}
	for _, sample := range run.Suite.Cases {
		if sample.ID != "" {
			samples[sample.ID] = true
		}
	}
	for _, id := range run.Suite.ProfileIDs {
		s := modelSummary{ID: id, Name: fmt.Sprintf("模型 %d", id), Expected: len(samples) * run.Suite.Repeats}
		seen := map[string]bool{}
		for _, r := range run.Results {
			if r.Profile.ID != id || !samples[r.Sample.ID] || r.Repeat < 1 || r.Repeat > run.Suite.Repeats {
				continue
			}
			key := fmt.Sprintf("%s/%d", r.Sample.ID, r.Repeat)
			if seen[key] {
				continue
			}
			seen[key] = true
			s.Name = firstText(r.Profile.Name, s.Name)
			s.Deployment = r.Profile.Deployment
			s.Results = append(s.Results, r)
			if r.Status == "call_failed" {
				s.CallFailed++
				continue
			}
			if r.Status != "completed" {
				s.Pending++
				continue
			}
			s.Calls++
			s.Duration += r.DurationMs
			switch verdict(r) {
			case "满足":
				s.Passed++
			case "不满足":
				s.Failed++
			default:
				s.Pending++
			}
		}
		s.Missing = max(0, s.Expected-len(seen))
		output.Models = append(output.Models, s)
	}
	qualified := []modelSummary{}
	unresolved := run.Status != "completed" || run.Completed < run.Total || len(output.Models) == 0 || len(samples) != len(run.Suite.Cases) || run.Total != len(output.Models)*len(samples)*run.Suite.Repeats
	for _, s := range output.Models {
		if s.qualified() {
			qualified = append(qualified, s)
		}
		if s.Pending+s.Missing > 0 || s.Expected == 0 {
			unresolved = true
		}
	}
	switch {
	case unresolved:
		output.Recommendation = "暂不能确定本次最优模型，仍有待复核、未完成或缺少判断依据的结果。"
		if len(qualified) > 0 {
			names := []string{}
			for _, s := range qualified {
				names = append(names, s.Name)
			}
			output.Reason = "目前已确认满足全部要求的候选是 " + strings.Join(names, "、") + "；其余结果完成复核后再作最终比较。"
		} else {
			output.Reason = "先补齐未完成样本或复核不确定项，再判断哪个模型满足本次业务要求。"
		}
	case len(qualified) == 0:
		output.Recommendation = "本次没有模型满足全部验收要求，暂不推荐直接采用其中任何一个。"
		output.Reason = "优先查看不满足项和调用失败原因，调整配置或任务要求后再验证。响应快不能抵消关键业务检查未通过。"
	case len(output.Models) == 1:
		output.Recommendation = fmt.Sprintf("本次仅测评 %s，其结果满足全部检查；尚不能判断它是否优于其他模型。", qualified[0].Name)
		output.Reason = "该模型可作为本次任务的达标候选。比较其他候选后，再形成相对最优的选型结论。"
	case len(qualified) == 1:
		winner := qualified[0]
		output.PreferredID = winner.ID
		output.Recommendation = fmt.Sprintf("本次优先推荐 %s。它是本次唯一满足全部验收要求的候选模型。", winner.Name)
		reasons := []string{fmt.Sprintf("%s 的 %d/%d 个样本均满足所有已记录检查。", winner.Name, winner.Passed, winner.Expected)}
		for _, other := range output.Models {
			if other.ID == winner.ID {
				continue
			}
			labels := []string{}
			seen := map[string]bool{}
			detail, source := "", ""
			for _, r := range other.Results {
				for _, c := range r.Checks {
					if c.Status == "not_satisfied" {
						if !seen[c.Label] {
							labels = append(labels, c.Label)
							seen[c.Label] = true
						}
						if detail == "" {
							detail = compact(c.Actual, 110)
							source = c.Source
						}
					}
				}
			}
			if len(labels) > 0 {
				reasons = append(reasons, fmt.Sprintf("%s 在“%s”检查中未满足要求（%s）：%s。", other.Name, strings.Join(labels, "、"), firstText(source, "保存的检查"), strings.TrimRight(detail, "。.!！")))
			}
			if other.CallFailed > 0 {
				reasons = append(reasons, fmt.Sprintf("%s 有 %d 个调用失败，尚未完成本次验收。", other.Name, other.CallFailed))
			}
			if len(reasons) >= 4 {
				break
			}
		}
		output.Reason = strings.Join(reasons, "\n")
	default:
		allTimed := true
		for _, s := range qualified {
			if s.average() <= 0 {
				allTimed = false
			}
		}
		sort.SliceStable(qualified, func(i, j int) bool { return qualified[i].average() < qualified[j].average() })
		if allTimed && qualified[0].average() < qualified[1].average() {
			winner := qualified[0]
			output.PreferredID = winner.ID
			output.Recommendation = fmt.Sprintf("若在满足全部验收要求后优先考虑响应速度，本次优先选择 %s。", winner.Name)
			output.Reason = fmt.Sprintf("多个候选均满足本次全部检查；%s 的平均成功请求耗时为 %d ms，是达标候选中最低的。业务质量检查未区分出唯一优胜者，这里的选择依据是本次实测速度。", winner.Name, winner.average())
		} else {
			output.Recommendation = "多个模型均满足本次要求，现有结果不足以选出唯一最优模型。"
			output.Reason = "业务检查结果相同，耗时相同或缺少可比较耗时；可结合更多实际业务样本作选择。"
		}
	}
	// Common objective success is a finding, not a proxy for semantic quality.
	labels := []string{}
	order := []string{}
	counts := map[string]int{}
	names := map[string]string{}
	totalCalls := 0
	for _, s := range output.Models {
		totalCalls += s.Calls
		for _, r := range s.Results {
			if r.Status != "completed" {
				continue
			}
			seen := map[string]bool{}
			for _, c := range r.Checks {
				if c.Source != "程序校验" || seen[c.ID] {
					continue
				}
				seen[c.ID] = true
				if _, ok := names[c.ID]; !ok {
					order = append(order, c.ID)
					names[c.ID] = c.Label
				}
				if c.Status == "satisfied" {
					counts[c.ID]++
				}
			}
		}
	}
	for _, id := range order {
		if totalCalls > 0 && counts[id] == totalCalls {
			labels = append(labels, names[id])
		}
	}
	if len(labels) > 0 {
		output.Findings = append(output.Findings, "已完成调用均通过程序检查中的“"+strings.Join(labels, "、")+"”。这些结果说明输出符合已检查的结构要求，内容是否满足业务仍需结合后续评价。")
	}
	for _, s := range output.Models {
		problemLabels := []string{}
		seen := map[string]bool{}
		sources := map[string]bool{}
		for _, r := range s.Results {
			for _, c := range r.Checks {
				if c.Status == "not_satisfied" {
					if !seen[c.Label] {
						problemLabels = append(problemLabels, c.Label)
						seen[c.Label] = true
					}
					sources[c.Source] = true
				}
			}
		}
		if len(problemLabels) > 0 {
			source := "保存的检查结果"
			if sources["模型辅助评价"] && !sources["程序校验"] && !sources["人工复核"] {
				source = "辅助评审"
			}
			output.Findings = append(output.Findings, fmt.Sprintf("%s 被%s标记为“%s”不满足。该结论来自本次检查记录，可对照原文依据复核。", s.Name, source, strings.Join(problemLabels, "、")))
		}
		if s.CallFailed > 0 {
			output.Findings = append(output.Findings, fmt.Sprintf("%s 有 %d 个调用失败；这些结果反映请求未完成，不能据此判断其回答质量。", s.Name, s.CallFailed))
		}
		if s.Pending+s.Missing > 0 {
			output.Findings = append(output.Findings, fmt.Sprintf("%s 有 %d 个待复核、%d 个未完成样本，结论仍需补充依据。", s.Name, s.Pending, s.Missing))
		}
	}
	if output.PreferredID > 0 {
		var winner modelSummary
		for _, s := range output.Models {
			if s.ID == output.PreferredID {
				winner = s
			}
		}
		for _, s := range output.Models {
			if s.ID != winner.ID && winner.average() > 0 && s.average() > 0 && s.average() < winner.average() && !s.qualified() {
				output.Findings = append(output.Findings, fmt.Sprintf("%s 平均耗时 %d ms，比推荐候选 %s 的 %d ms 更快，但未满足全部业务要求。本次按业务达标优先选择。", s.Name, s.average(), winner.Name, winner.average()))
			}
		}
	}
	if len(output.Findings) == 0 {
		output.Findings = append(output.Findings, "当前记录没有足够的可比较检查结果。")
	}
	return output
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
