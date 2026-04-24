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

// JobPreset is a saved form template (keywords, location, depth, etc.) without a scrape run.
type JobPreset struct {
	ID        string
	Name      string
	Data      JobData
	CreatedAt time.Time
}

// PresetRepository persists user-defined job templates.
type PresetRepository interface {
	CreatePreset(ctx context.Context, p *JobPreset) error
	DeletePreset(ctx context.Context, id string) error
	ListPresets(ctx context.Context) ([]JobPreset, error)
	GetPreset(ctx context.Context, id string) (JobPreset, error)
}

// Datastore is SQLite / Turso (or other) persistence for jobs, presets, and live results.
type Datastore interface {
	JobRepository
	PresetRepository
	ResultRepository
}

// ValidateAsTemplate checks fields before saving a preset (keywords may be empty).
func (d *JobData) ValidateAsTemplate() error {
	if d.Lang != "" && len(d.Lang) != 2 {
		return errors.New("invalid lang (use two letters, e.g. en)")
	}

	if d.Zoom != 0 && (d.Zoom < 1 || d.Zoom > 21) {
		return errors.New("invalid zoom (use 1–21 or leave at default)")
	}

	const maxSearchRegionLen = 240

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

		cellKm := d.GridCellKm
		if cellKm <= 0 {
			cellKm = 45
		}

		if c := grid.EstimateCellCount(bbox, cellKm); c > 900 {
			return fmt.Errorf(
				"area grid too large (~%d searches); increase grid cell size (currently %.0f km) or pick a smaller preset",
				c, cellKm,
			)
		}
	}

	return nil
}
