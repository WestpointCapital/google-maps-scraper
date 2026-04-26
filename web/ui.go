package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// uiJobDataDTO is JSON-friendly job form fields for the web UI (duplicate / preset / apply).
type uiJobDataDTO struct {
	Keywords     []string `json:"keywords"`
	Lang         string   `json:"lang"`
	Zoom         int      `json:"zoom"`
	Lat          string   `json:"lat"`
	Lon          string   `json:"lon"`
	FastMode     bool     `json:"fast_mode"`
	Radius       int      `json:"radius"`
	Depth        int      `json:"depth"`
	Email        bool     `json:"email"`
	ExtraReviews bool     `json:"extra_reviews"`
	MaxTime      string   `json:"max_time"`
	Proxies      []string `json:"proxies"`
	AreaPreset   string   `json:"area_preset,omitempty"`
	SearchRegion string   `json:"search_region,omitempty"`
	GridCellKm   float64  `json:"grid_cell_km,omitempty"`
	SimpleMode   bool     `json:"simple_mode,omitempty"`
	Intensity    int      `json:"intensity,omitempty"`
}

// uiJobFormResponse is returned by GET /ui/job-form-data for filling the sidebar form.
type uiJobFormResponse struct {
	NameHint string       `json:"name_hint"`
	Data     uiJobDataDTO `json:"data"`
}

type presetSaveRequest struct {
	PresetName string       `json:"preset_name"`
	Data       uiJobDataDTO `json:"data"`
}

func jobDataToDTO(d JobData) uiJobDataDTO {
	return uiJobDataDTO{
		Keywords:     d.Keywords,
		Lang:         d.Lang,
		Zoom:         d.Zoom,
		Lat:          d.Lat,
		Lon:          d.Lon,
		FastMode:     d.FastMode,
		Radius:       d.Radius,
		Depth:        d.Depth,
		Email:        d.Email,
		ExtraReviews: d.ExtraReviews,
		MaxTime:      d.MaxTime.String(),
		Proxies:      d.Proxies,
		AreaPreset:   d.AreaPreset,
		SearchRegion: d.SearchRegion,
		GridCellKm:   d.GridCellKm,
		SimpleMode:   d.SimpleMode,
		Intensity:    d.Intensity,
	}
}

func jobDataFromDTO(d uiJobDataDTO) (JobData, error) {
	var mt time.Duration

	switch strings.TrimSpace(d.MaxTime) {
	case "":
		mt = 10 * time.Minute
	default:
		var err error

		mt, err = time.ParseDuration(d.MaxTime)
		if err != nil {
			return JobData{}, fmt.Errorf("max time: %w", err)
		}
	}

	lang := strings.TrimSpace(d.Lang)
	if lang == "" {
		lang = "en"
	}

	zoom := d.Zoom
	if zoom == 0 {
		zoom = 14
	}

	depth := d.Depth
	if depth == 0 {
		depth = 10
	}

	radius := d.Radius
	if radius == 0 {
		radius = 10000
	}

	return JobData{
		Keywords:     d.Keywords,
		Lang:         lang,
		Zoom:         zoom,
		Lat:          d.Lat,
		Lon:          d.Lon,
		FastMode:     d.FastMode,
		Radius:       radius,
		Depth:        depth,
		Email:        d.Email,
		ExtraReviews: d.ExtraReviews,
		MaxTime:      mt,
		Proxies:      d.Proxies,
		AreaPreset:   d.AreaPreset,
		SearchRegion: d.SearchRegion,
		GridCellKm:   d.GridCellKm,
		SimpleMode:   d.SimpleMode,
		Intensity:    d.Intensity,
	}, nil
}

func (s *Server) uiJobFormData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)

		return
	}

	job, err := s.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	if err := enc.Encode(jobToFormResponse(job)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

func jobToFormResponse(j Job) uiJobFormResponse {
	return uiJobFormResponse{
		NameHint: j.Name,
		Data:     jobDataToDTO(j.Data),
	}
}

func (s *Server) uiJobDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)

		return
	}

	job, err := s.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)

		return
	}

	tmpl, ok := s.tmpl["static/templates/job_detail.html"]
	if !ok {
		http.Error(w, "missing tpl", http.StatusInternalServerError)

		return
	}

	nResults, _ := s.svc.CountJobResults(r.Context(), id)

	exportPath := "/api/v1/jobs/" + id + "/results?since=0&limit=50000"

	item := s.jobListItemForJob(&job)

	vm := jobDetailVM{
		Job:              job,
		StatusClass:      item.StatusClass(),
		StatusLabel:      item.StatusLabel(),
		KeywordsDisplay:  strings.Join(job.Data.Keywords, "\n"),
		ProxiesDisplay:   strings.Join(job.Data.Proxies, "\n"),
		ResultCount:      nResults,
		ExportResultsURL: exportPath,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tmpl.Execute(w, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

// jobDetailVM is passed to job_detail.html.
type jobDetailVM struct {
	Job              Job
	StatusClass      string
	StatusLabel      string
	KeywordsDisplay  string
	ProxiesDisplay   string
	ResultCount      int64
	ExportResultsURL string
}

func (s *Server) writePresetPanelHTML(ctx context.Context, w io.Writer) error {
	presets, err := s.svc.ListPresets(ctx)
	if err != nil {
		return err
	}

	tmpl, ok := s.tmpl["static/templates/preset_panel.html"]
	if !ok {
		return fmt.Errorf("missing preset panel template")
	}

	return tmpl.Execute(w, presets)
}

func (s *Server) uiPresetsPanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := s.writePresetPanelHTML(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

type presetSaveSimpleRequest struct {
	PresetName   string `json:"preset_name"`
	BusinessType string `json:"business_type"`
	Location     string `json:"location"`
	Intensity    int    `json:"intensity"`
	Email        bool   `json:"email"`
	Lang         string `json:"lang"`
}

func (s *Server) uiPresetSaveSimple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	const maxBody = 1 << 20

	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req presetSaveSimpleRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)

		return
	}

	name := strings.TrimSpace(req.PresetName)
	if name == "" {
		http.Error(w, "preset name is required", http.StatusBadRequest)

		return
	}

	if len(name) > 128 {
		http.Error(w, "preset name is too long", http.StatusBadRequest)

		return
	}

	lang := strings.TrimSpace(req.Lang)
	if lang == "" {
		lang = "en"
	}

	data, err := BuildJobDataFromSimpleRequest(
		req.BusinessType,
		req.Location,
		req.Intensity,
		req.Email,
		false,
		lang,
		nil,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	if err := data.ValidateAsTemplate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)

		return
	}

	p := &JobPreset{
		ID:        uuid.New().String(),
		Name:      name,
		Data:      data,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.svc.CreatePreset(r.Context(), p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": p.ID})
}

func (s *Server) uiPresetSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	const maxBody = 1 << 20

	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req presetSaveRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)

		return
	}

	name := strings.TrimSpace(req.PresetName)
	if name == "" {
		http.Error(w, "preset name is required", http.StatusBadRequest)

		return
	}

	if len(name) > 128 {
		http.Error(w, "preset name is too long", http.StatusBadRequest)

		return
	}

	data, err := jobDataFromDTO(req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	if err := data.ValidateAsTemplate(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)

		return
	}

	p := &JobPreset{
		ID:        uuid.New().String(),
		Name:      name,
		Data:      data,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.svc.CreatePreset(r.Context(), p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": p.ID})
}

func (s *Server) uiPresetDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)

		return
	}

	if err := s.svc.DeletePreset(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer

	if err := s.writePresetPanelHTML(r.Context(), &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(buf.Bytes())
}

func (s *Server) uiPresetFormData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)

		return
	}

	p, err := s.svc.GetPreset(r.Context(), id)
	if err != nil {
		http.Error(w, "preset not found", http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	resp := uiJobFormResponse{
		NameHint: p.Name,
		Data:     jobDataToDTO(p.Data),
	}

	if err := enc.Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}
}

