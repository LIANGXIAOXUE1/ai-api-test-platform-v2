package evaluation

import (
	"context"
	"time"
)

type Suite struct {
	ID              int64    `json:"id"`
	ProjectID       int      `json:"projectId"`
	Name            string   `json:"name"`
	Scenario        string   `json:"scenario"`
	Goal            string   `json:"goal"`
	Facts           string   `json:"facts"`
	Requirements    string   `json:"requirements"`
	ShotCount       int      `json:"shotCount"`
	DurationSeconds int      `json:"durationSeconds"`
	OutputFields    []string `json:"outputFields"`
	Cases           []Sample `json:"cases"`
	ProfileIDs      []int    `json:"profileIds"`
	JudgeProfileID  int      `json:"judgeProfileId"`
	Repeats         int      `json:"repeats"`
}
type Sample struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected string `json:"expected,omitempty"`
}
type Candidate struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Deployment string `json:"deployment"`
}
type Check struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Evidence string `json:"evidence"`
	Source   string `json:"source"`
}
type Result struct {
	Profile      Candidate `json:"profile"`
	Sample       Sample    `json:"sample"`
	Repeat       int       `json:"repeat"`
	Status       string    `json:"status"`
	Answer       string    `json:"answer"`
	Error        string    `json:"error,omitempty"`
	Checks       []Check   `json:"checks"`
	DurationMs   int64     `json:"durationMs"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	UsageSource  string    `json:"usageSource"`
	JudgeName    string    `json:"judgeName,omitempty"`
}
type Run struct {
	ID          int64      `json:"id"`
	SourceRunID int64      `json:"sourceRunId,omitempty"`
	ProjectID   int        `json:"projectId"`
	Suite       Suite      `json:"suite"`
	Status      string     `json:"status"`
	Total       int        `json:"total"`
	Completed   int        `json:"completed"`
	Results     []Result   `json:"results"`
	Conclusion  string     `json:"conclusion"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}
type Completion struct {
	Answer       string
	InputTokens  int
	OutputTokens int
	UsageSource  string
}
type Gateway interface {
	Candidate(context.Context, int) (Candidate, error)
	Complete(context.Context, int, string, string) (Completion, error)
	DefaultJudge(context.Context) (int, error)
}
