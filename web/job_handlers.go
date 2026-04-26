package web

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) jobListItemForJob(j *Job) JobListItem {
	if j == nil {
		return JobListItem{}
	}

	return JobListItem{
		Job:      *j,
		IsPaused: s.svc.IsJobPaused(j.ID),
	}
}

func (s *Server) writeJobRowHTML(w http.ResponseWriter, j Job) {
	tmpl, ok := s.tmpl["static/templates/job_row.html"]
	if !ok {
		http.Error(w, "missing tpl", http.StatusInternalServerError)

		return
	}

	_ = tmpl.Execute(w, s.jobListItemForJob(&j))
}

// waitForJobSettled polls until the job is no longer working or timeout.
func (s *Server) waitForJobSettled(ctx context.Context, jobID string, d time.Duration) (Job, error) {
	deadline := time.Now().Add(d)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	for {
		j, err := s.svc.Get(ctx, jobID)
		if err != nil {
			return j, err
		}

		if j.Status != StatusWorking {
			return j, nil
		}

		if time.Now().After(deadline) {
			return j, nil
		}

		select {
		case <-ctx.Done():
			return j, ctx.Err()
		case <-tick.C:
		}
	}
}

func (s *Server) requireJobController(w http.ResponseWriter) bool {
	if s.jobCtl == nil {
		http.Error(w, "job control is not available", http.StatusServiceUnavailable)

		return false
	}

	return true
}

func (s *Server) jobStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if !s.requireJobController(w) {
		return
	}

	r = requestWithID(r)

	id, ok := getIDFromRequest(r)
	if !ok {
		http.Error(w, "Invalid ID", http.StatusUnprocessableEntity)

		return
	}

	jobID := id.String()

	if err := s.jobCtl.RequestStop(r.Context(), jobID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	j, err := s.waitForJobSettled(r.Context(), jobID, 2*time.Minute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.writeJobRowHTML(w, j)
}

func (s *Server) jobPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if !s.requireJobController(w) {
		return
	}

	r = requestWithID(r)

	id, ok := getIDFromRequest(r)
	if !ok {
		http.Error(w, "Invalid ID", http.StatusUnprocessableEntity)

		return
	}

	if err := s.jobCtl.RequestPause(r.Context(), id.String()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	j, err := s.svc.Get(r.Context(), id.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.writeJobRowHTML(w, j)
}

func (s *Server) jobResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if !s.requireJobController(w) {
		return
	}

	r = requestWithID(r)

	id, ok := getIDFromRequest(r)
	if !ok {
		http.Error(w, "Invalid ID", http.StatusUnprocessableEntity)

		return
	}

	if err := s.jobCtl.RequestResume(r.Context(), id.String()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	j, err := s.svc.Get(r.Context(), id.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.writeJobRowHTML(w, j)
}

type apiOKMessage struct {
	OK    bool   `json:"ok"`
	JobID string `json:"job_id"`
}

func (s *Server) apiJobStop(w http.ResponseWriter, r *http.Request) {
	if s.jobCtl == nil {
		ans := apiError{
			Code:    http.StatusServiceUnavailable,
			Message: "job control is not available",
		}

		renderJSON(w, http.StatusServiceUnavailable, ans)

		return
	}

	id, ok := getIDFromRequest(r)
	if !ok {
		ans := apiError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Invalid job id",
		}

		renderJSON(w, http.StatusUnprocessableEntity, ans)

		return
	}

	jobID := id.String()

	if err := s.jobCtl.RequestStop(r.Context(), jobID); err != nil {
		ans := apiError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}

		renderJSON(w, http.StatusBadRequest, ans)

		return
	}

	renderJSON(w, http.StatusOK, apiOKMessage{OK: true, JobID: jobID})
}

func (s *Server) apiJobPause(w http.ResponseWriter, r *http.Request) {
	if s.jobCtl == nil {
		ans := apiError{
			Code:    http.StatusServiceUnavailable,
			Message: "job control is not available",
		}

		renderJSON(w, http.StatusServiceUnavailable, ans)

		return
	}

	id, ok := getIDFromRequest(r)
	if !ok {
		ans := apiError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Invalid job id",
		}

		renderJSON(w, http.StatusUnprocessableEntity, ans)

		return
	}

	jobID := id.String()

	if err := s.jobCtl.RequestPause(r.Context(), jobID); err != nil {
		ans := apiError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}

		renderJSON(w, http.StatusBadRequest, ans)

		return
	}

	renderJSON(w, http.StatusOK, apiOKMessage{OK: true, JobID: jobID})
}

func (s *Server) apiJobResume(w http.ResponseWriter, r *http.Request) {
	if s.jobCtl == nil {
		ans := apiError{
			Code:    http.StatusServiceUnavailable,
			Message: "job control is not available",
		}

		renderJSON(w, http.StatusServiceUnavailable, ans)

		return
	}

	id, ok := getIDFromRequest(r)
	if !ok {
		ans := apiError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Invalid job id",
		}

		renderJSON(w, http.StatusUnprocessableEntity, ans)

		return
	}

	jobID := id.String()

	if err := s.jobCtl.RequestResume(r.Context(), jobID); err != nil {
		ans := apiError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}

		renderJSON(w, http.StatusBadRequest, ans)

		return
	}

	renderJSON(w, http.StatusOK, apiOKMessage{OK: true, JobID: jobID})
}
