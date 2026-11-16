package projecttest

import "time"

type TaskInput struct {
	Field    string `json:"field"`
	Question string `json:"question"`
	Secret   bool   `json:"secret,omitempty"`
}
type MissingTaskInformation struct{ Fields []TaskInput }

func (e *MissingTaskInformation) Error() string { return "需要补充测试信息" }

type Task struct {
	ID               int64             `json:"id"`
	ProjectID        int               `json:"projectId"`
	Name             string            `json:"name"`
	BaseURL          string            `json:"baseUrl"`
	OpenAPIURL       string            `json:"openApiUrl"`
	Goal             string            `json:"goal"`
	Template         string            `json:"template"`
	AccountMode      string            `json:"accountMode"`
	Username         string            `json:"username"`
	Password         string            `json:"password,omitempty"`
	HasPassword      bool              `json:"hasPassword"`
	FileMode         string            `json:"fileMode"`
	FileName         string            `json:"fileName"`
	FileData         string            `json:"fileData,omitempty"`
	FileSize         int               `json:"fileSize"`
	Permission       string            `json:"permission"`
	Features         []string          `json:"features"`
	InitialVariables map[string]string `json:"initialVariables,omitempty"`
	Steps            []TaskStep        `json:"steps"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}
type TaskStep struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	Query      map[string]string `json:"query"`
	Body       string            `json:"body"`
	Upload     bool              `json:"upload"`
	FileField  string            `json:"fileField,omitempty"`
	DependsOn  []string          `json:"dependsOn"`
	Assertions []TaskAssertion   `json:"assertions"`
	Extract    []TaskExtract     `json:"extract"`
	Guard      string            `json:"guard,omitempty"`
}
type TaskAssertion struct {
	Type       string `json:"type"`
	Path       string `json:"path,omitempty"`
	Expected   string `json:"expected"`
	Field      string `json:"field,omitempty"`
	MatchField string `json:"matchField,omitempty"`
	MatchValue string `json:"matchValue,omitempty"`
}
type TaskExtract struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	MatchField string `json:"matchField,omitempty"`
	MatchValue string `json:"matchValue,omitempty"`
	ValueField string `json:"valueField,omitempty"`
	Owned      bool   `json:"owned,omitempty"`
}
type CheckResult struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}
type StepResult struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	StatusCode     int               `json:"statusCode"`
	DurationMs     int64             `json:"durationMs"`
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	RequestHeaders map[string]string `json:"requestHeaders"`
	RequestBody    string            `json:"requestBody"`
	ResponseBody   string            `json:"responseBody"`
	ResponseBytes  int               `json:"responseBytes"`
	Checks         []CheckResult     `json:"checks"`
	Extracted      map[string]string `json:"extracted"`
	Error          string            `json:"error,omitempty"`
	Reused         bool              `json:"reused,omitempty"`
}
type TaskRun struct {
	ID          int64             `json:"id"`
	ProjectID   int               `json:"projectId"`
	TaskID      int64             `json:"taskId"`
	SourceRunID int64             `json:"sourceRunId,omitempty"`
	Task        Task              `json:"task"`
	Status      string            `json:"status"`
	CurrentStep string            `json:"currentStep"`
	Results     []StepResult      `json:"results"`
	Summary     RunSummary        `json:"summary"`
	Variables   map[string]string `json:"variables"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	FinishedAt  *time.Time        `json:"finishedAt,omitempty"`
}
type RunSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Blocked int `json:"blocked"`
	NotRun  int `json:"notRun"`
}
type RunPrivate struct {
	FileData     string            `json:"fileData,omitempty"`
	Variables    map[string]string `json:"variables"`
	Owned        map[string]string `json:"owned"`
	CreatedNames map[string]bool   `json:"createdNames,omitempty"`
}
