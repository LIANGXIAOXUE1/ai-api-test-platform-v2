package projecttest

import (
	"context"
	"time"
)

// ProjectCheck is a deliberately safe, project-wide connectivity sweep. It is
// not a business regression run: requests are stripped of credentials and
// payloads so that the check proves routing/reachability without creating or
// deleting user data.
type Input struct {
	ProjectID  int    `json:"projectId"`
	BaseURL    string `json:"baseUrl"`
	Task       string `json:"task"`
	Permission string `json:"permission"` // review | auto
	Confirmed  bool   `json:"confirmed"`
}

type Item struct {
	APIID        int    `json:"apiId"`
	Name         string `json:"name"`
	Method       string `json:"method"`
	URL          string `json:"url"`
	Risk         string `json:"risk"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason,omitempty"`
	Reachable    bool   `json:"reachable"`
	StatusCode   int    `json:"statusCode"`
	BusinessCode string `json:"businessCode,omitempty"`
	DurationMs   int64  `json:"durationMs"`
	Error        string `json:"error,omitempty"`
}

type Summary struct {
	Total       int `json:"total"`
	Reachable   int `json:"reachable"`
	Unavailable int `json:"unavailable"`
	Pending     int `json:"pending"`
}

type Report struct {
	ID         int       `json:"id"`
	ProjectID  int       `json:"projectId"`
	BaseURL    string    `json:"baseUrl"`
	Task       string    `json:"task"`
	Permission string    `json:"permission"`
	Executed   bool      `json:"executed"`
	CreatedAt  time.Time `json:"createdAt"`
	Summary    Summary   `json:"summary"`
	Items      []Item    `json:"items"`
	Markdown   string    `json:"markdown"`
}

// Endpoint intentionally excludes saved credentials and request payloads.
type Endpoint struct {
	ID     int
	Name   string
	Method string
	URL    string
}

type ProbeRequest struct {
	ProjectID int
	APIID     int
	Name      string
	Method    string
	URL       string
}

type ProbeResult struct {
	StatusCode   int
	DurationMs   int64
	ResponseBody string
	ErrorMessage string
}

type ProbeExecutor interface {
	Execute(context.Context, ProbeRequest) (ProbeResult, error)
}

type ProbeExecutorFunc func(context.Context, ProbeRequest) (ProbeResult, error)

func (f ProbeExecutorFunc) Execute(ctx context.Context, input ProbeRequest) (ProbeResult, error) {
	return f(ctx, input)
}

type Repository interface {
	Endpoints(context.Context, int) ([]Endpoint, error)
	SaveReport(context.Context, Report) (Report, error)
	Reports(context.Context, int) ([]Report, error)
}
