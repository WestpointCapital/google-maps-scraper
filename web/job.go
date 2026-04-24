package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/areapresets"
	"github.com/gosom/google-maps-scraper/grid"
)

var jobs []Job

const (
	StatusPending = "pending"
	StatusWorking = "working"
	StatusOK      = "ok"
	StatusFailed  = "failed"
)

type SelectParams struct {
	Status string
	Limit  int
}

type JobRepository interface {
	Get(context.Context, string) (Job, error)
	Create(context.Context, *Job) error
	Delete(context.Context, string) error
	Select(context.Context, SelectParams) ([]Job, error)
	Update(context.Context, *Job) error
}

type Job struct {
	ID     string
	Name   string
	Date   time.Time
	Status string
	Data   JobData
}

func (j *Job) Validate() error {
	if j.ID == "" {
		return errors.New("missing id")
	}

	if j.Name == "" {
		return errors.New("missing name")
	}

	if j.Status == "" {
		return errors.New("missing status")
	}

	if j.Date.IsZero() {
		return errors.New("missing date")
	}

	if err := j.Data.Validate(); err != nil {
		return err
	}

	return nil
}

type JobData struct {
	Keywords     []string      `json:"keywords"`
	Lang         string        `json:"lang"`
	Zoom         int           `json:"zoom"`
	Lat          string        `json:"lat"`
	Lon          string        `json:"lon"`
	FastMode     bool          `json:"fast_mode"`
	Radius       int           `json:"radius"`
	Depth        int           `json:"depth"`
	Email        bool          `json:"email"`
	ExtraReviews bool          `json:"extra_reviews"`
	MaxTime      time.Duration `json:"max_time"`
	Proxies      []string      `json:"proxies"`
	// AreaPreset selects a built-in region for grid scraping (e.g. "us-ca" = California).
	// Empty or "none" disables grid mode (keyword + optional map center only).
	AreaPreset string `json:"area_preset,omitempty"`
	// SearchRegion is optional free text appended to each keyword as " … in <region>"
	// (unless the keyword already contains the region). Example: "California, USA".
	SearchRegion string `json:"search_region,omitempty"`
	// GridCellKm is the approximate cell size for grid mode (default 45 when unset or zero).
	GridCellKm float64 `json:"grid_cell_km,omitempty"`
	// SimpleMode is true when the job was created from the simplified UI (slider + location).
	SimpleMode bool `json:"simple_mode,omitempty"`
	// Intensity is the 0–100 slider value when SimpleMode is set.
	Intensity int `json:"intensity,omitempty"`
}

func (d *JobData) Validate() error {
	if len(d.Keywords) == 0 {
		return errors.New("missing keywords")
	}

	if d.Lang == "" {
		return errors.New("missing lang")
	}

	if len(d.Lang) != 2 {
		return errors.New("invalid lang")
	}

	if d.Depth == 0 {
		return errors.New("missing depth")
	}

	if d.MaxTime == 0 {
		return errors.New("missing max time")
	}

	if d.FastMode && (d.Lat == "" || d.Lon == "") {
		return errors.New("missing geo coordinates")
	}

	const (
		maxSearchRegionLen   = 240
		maxGridCellsEstimate = 900
		defaultGridCellKm    = 45.0
	)

	if len(strings.TrimSpace(d.SearchRegion)) > maxSearchRegionLen {
		return errors.New("search region text is too long")
	}

	preset := strings.ToLower(strings.TrimSpace(d.AreaPreset))
	if preset != "" && preset != "none" {
		if d.FastMode {
			return errors.New("full-state grid cannot be used together with fast mode")
		}

		bbox, ok := areapresets.USStateBoundingBox(d.AreaPreset)
		if !ok {
			return fmt.Errorf("unknown area preset: %s", d.AreaPreset)
		}

		if d.Zoom < 1 || d.Zoom > 21 {
			return errors.New("invalid zoom level (use 1–21)")
		}

		cellKm := d.GridCellKm
		if cellKm <= 0 {
			cellKm = defaultGridCellKm
		}

		if c := grid.EstimateCellCount(bbox, cellKm); c > maxGridCellsEstimate {
			return fmt.Errorf(
				"area grid too large (~%d searches); increase grid cell size (currently %.0f km) or pick a smaller preset",
				c, cellKm,
			)
		}
	}

	return nil
}
