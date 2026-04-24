package web

import (
	"net/http"
	"strconv"
	"strings"
)

// apiJobResultsResponse is JSON for live polling and export.
type apiJobResultsResponse struct {
	JobStatus       string      `json:"job_status"`
	Results         []JobResult `json:"results"`
	LatestTimestamp int64       `json:"latest_timestamp"`
	Total           int64       `json:"total"`
	JobName         string      `json:"job_name,omitempty"`
	Email           EmailStats  `json:"email"`
}

// apiJobStatsResponse is JSON for lightweight status polling.
type apiJobStatsResponse struct {
	JobStatus   string     `json:"job_status"`
	ResultCount int64      `json:"result_count"`
	JobID       string     `json:"job_id"`
	JobName     string     `json:"job_name,omitempty"`
	Email       EmailStats `json:"email"`
}

func (s *Server) apiGetJobResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id, ok := getIDFromRequest(r)
	if !ok {
		renderJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Invalid ID",
		})

		return
	}

	since, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("since")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if limit <= 0 {
		limit = 500
	}

	job, err := s.svc.Get(r.Context(), id.String())
	if err != nil {
		renderJSON(w, http.StatusNotFound, apiError{
			Code:    http.StatusNotFound,
			Message: "job not found",
		})

		return
	}

	rows, err := s.svc.ListJobResultsSince(r.Context(), id.String(), since, limit)
	if err != nil {
		renderJSON(w, http.StatusInternalServerError, apiError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})

		return
	}

	stats, err := s.svc.CountJobResultsEmailStats(r.Context(), id.String())
	if err != nil {
		renderJSON(w, http.StatusInternalServerError, apiError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})

		return
	}

	latest := since
	for _, row := range rows {
		if row.UpdatedAt > latest {
			latest = row.UpdatedAt
		}
	}

	renderJSON(w, http.StatusOK, apiJobResultsResponse{
		JobStatus:       job.Status,
		Results:         rows,
		LatestTimestamp: latest,
		Total:           stats.Total,
		JobName:         job.Name,
		Email:           stats,
	})
}

func (s *Server) apiGetJobStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id, ok := getIDFromRequest(r)
	if !ok {
		renderJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Invalid ID",
		})

		return
	}

	job, err := s.svc.Get(r.Context(), id.String())
	if err != nil {
		renderJSON(w, http.StatusNotFound, apiError{
			Code:    http.StatusNotFound,
			Message: "job not found",
		})

		return
	}

	stats, err := s.svc.CountJobResultsEmailStats(r.Context(), id.String())
	if err != nil {
		renderJSON(w, http.StatusInternalServerError, apiError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})

		return
	}

	renderJSON(w, http.StatusOK, apiJobStatsResponse{
		JobStatus:   job.Status,
		ResultCount: stats.Total,
		JobID:       id.String(),
		JobName:     job.Name,
		Email:       stats,
	})
}
