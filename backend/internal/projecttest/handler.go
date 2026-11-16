package projecttest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input Input
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&input); err != nil || input.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请选择项目后再执行巡检"})
		return
	}
	output, err := h.service.Run(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, output)
}

func (h *Handler) Reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	reports, err := h.service.Reports(r.Context(), projectID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var validation *ValidationError
	if errors.As(err, &validation) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
