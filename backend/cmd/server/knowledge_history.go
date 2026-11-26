package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type KnowledgeGenerationHistorySummary struct {
	ID           int64     `json:"id"`
	ProjectID    int       `json:"projectId"`
	Prompt       string    `json:"prompt"`
	Mode         string    `json:"mode"`
	Status       string    `json:"status"`
	CaseCount    int       `json:"caseCount"`
	MissingCount int       `json:"missingCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type KnowledgeGenerationHistory struct {
	KnowledgeGenerationHistorySummary
	APIID       int                `json:"apiId"`
	ProfileID   int                `json:"profileId"`
	Supplement  string             `json:"supplement"`
	DocumentIDs []int              `json:"documentIds"`
	Documents   []string           `json:"documents"`
	Cases       []BusinessTestCase `json:"cases"`
	Missing     []string           `json:"missingInformation"`
}

type KnowledgeGenerationHistoryPage struct {
	Items  []KnowledgeGenerationHistorySummary `json:"items"`
	Total  int                                 `json:"total"`
	Limit  int                                 `json:"limit"`
	Offset int                                 `json:"offset"`
}

func knowledgeHistoryStatus(result businessCasesResult) string {
	if len(result.Cases) == 0 {
		return "needs_information"
	}
	if len(result.MissingInformation) > 0 {
		return "needs_attention"
	}
	for _, item := range result.Cases {
		if item.Status != "ready_for_review" {
			return "needs_attention"
		}
	}
	return "ready_for_review"
}

func (s *store) addKnowledgeGenerationHistory(ctx context.Context, input businessCasesInput, result businessCasesResult, documents []string, status string) (KnowledgeGenerationHistory, error) {
	item := KnowledgeGenerationHistory{
		KnowledgeGenerationHistorySummary: KnowledgeGenerationHistorySummary{
			ProjectID:    input.ProjectID,
			Prompt:       input.Prompt,
			Mode:         input.Mode,
			Status:       status,
			CaseCount:    len(result.Cases),
			MissingCount: len(result.MissingInformation),
		},
		APIID:       input.APIID,
		ProfileID:   input.ProfileID,
		Supplement:  input.Supplement,
		DocumentIDs: input.DocumentIDs,
		Documents:   documents,
		Cases:       result.Cases,
		Missing:     result.MissingInformation,
	}
	if item.DocumentIDs == nil {
		item.DocumentIDs = []int{}
	}
	if item.Documents == nil {
		item.Documents = []string{}
	}
	if item.Cases == nil {
		item.Cases = []BusinessTestCase{}
	}
	if item.Missing == nil {
		item.Missing = []string{}
	}
	err := s.db.QueryRowContext(ctx, `INSERT INTO knowledge_generation_history(project_id,api_id,profile_id,prompt,supplement,mode,document_ids,document_names,cases,missing_information,status) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb,$10::jsonb,$11) RETURNING id,created_at`, input.ProjectID, input.APIID, input.ProfileID, input.Prompt, input.Supplement, input.Mode, jsonBytes(item.DocumentIDs), jsonBytes(item.Documents), jsonBytes(item.Cases), jsonBytes(item.Missing), status).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func (s *store) knowledgeGenerationHistory(ctx context.Context, projectID, limit, offset int, query, status, mode string) (KnowledgeGenerationHistoryPage, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	filter := `project_id=$1 AND ($2='' OR strpos(lower(prompt),lower($2))>0) AND ($3='' OR status=$3) AND ($4='' OR mode=$4)`
	page := KnowledgeGenerationHistoryPage{Items: []KnowledgeGenerationHistorySummary{}, Limit: limit, Offset: offset}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_generation_history WHERE `+filter, projectID, query, status, mode).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,prompt,mode,status,jsonb_array_length(cases),jsonb_array_length(missing_information),created_at FROM knowledge_generation_history WHERE `+filter+` ORDER BY created_at DESC,id DESC LIMIT $5 OFFSET $6`, projectID, query, status, mode, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var item KnowledgeGenerationHistorySummary
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Prompt, &item.Mode, &item.Status, &item.CaseCount, &item.MissingCount, &item.CreatedAt); err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func (s *store) getKnowledgeGenerationHistory(ctx context.Context, projectID int, id int64) (KnowledgeGenerationHistory, bool) {
	var item KnowledgeGenerationHistory
	var documentIDs, documents, cases, missing []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,prompt,mode,status,jsonb_array_length(cases),jsonb_array_length(missing_information),created_at,api_id,profile_id,supplement,document_ids,document_names,cases,missing_information FROM knowledge_generation_history WHERE project_id=$1 AND id=$2`, projectID, id).Scan(&item.ID, &item.ProjectID, &item.Prompt, &item.Mode, &item.Status, &item.CaseCount, &item.MissingCount, &item.CreatedAt, &item.APIID, &item.ProfileID, &item.Supplement, &documentIDs, &documents, &cases, &missing)
	if err != nil {
		return item, false
	}
	_ = decodeJSON(documentIDs, &item.DocumentIDs)
	_ = decodeJSON(documents, &item.Documents)
	_ = decodeJSON(cases, &item.Cases)
	_ = decodeJSON(missing, &item.Missing)
	return item, true
}

func (s *server) knowledgeHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	projectID, err := strconv.Atoi(r.URL.Query().Get("projectId"))
	if err != nil || projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择有效项目后查看生成历史"})
		return
	}
	if rawID := r.URL.Query().Get("id"); rawID != "" {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "历史记录编号不正确"})
			return
		}
		item, ok := s.store.getKnowledgeGenerationHistory(r.Context(), projectID, id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "当前项目中没有这条生成历史"})
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 120 {
		query = string([]rune(query)[:120])
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "ready_for_review" && status != "needs_attention" && status != "needs_information" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "历史状态筛选项无效"})
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode != "" && mode != "design" && mode != "executable" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "历史类型筛选项无效"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset := 0
	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		offset, err = strconv.Atoi(rawOffset)
		if err != nil || offset < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "历史分页参数无效"})
			return
		}
	}
	page, err := s.store.knowledgeGenerationHistory(r.Context(), projectID, limit, offset, query, status, mode)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取知识库生成历史失败"})
		return
	}
	writeJSON(w, http.StatusOK, page)
}
