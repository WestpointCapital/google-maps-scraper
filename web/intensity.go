package web

import (
	"fmt"
	"strings"

	"github.com/gosom/google-maps-scraper/grid"
)

// IntensityPlan maps the 0–100 UI slider to concrete scrape parameters.
type IntensityPlan struct {
	Intensity      int
	Depth          int
	Zoom           int
	GridCellKm     float64
	TargetCellCount int
	KeywordSuffixes []string
	Hint           string
}

const (
	intensityMin = 0
	intensityMax = 100
	maxGridCells = 900
)

// BuildIntensityPlan derives depth, zoom, grid density, and optional query suffixes
// from a business phrase and intensity 0–100.
func BuildIntensityPlan(business string, intensity int) IntensityPlan {
	if intensity < intensityMin {
		intensity = intensityMin
	}

	if intensity > intensityMax {
		intensity = intensityMax
	}

	business = strings.TrimSpace(business)

	p := IntensityPlan{
		Intensity: intensity,
		Zoom:      14,
		GridCellKm: 45,
		TargetCellCount: 1,
	}

	switch {
	case intensity <= 20:
		p.Depth = 10
		p.Zoom = 14
		p.TargetCellCount = 1
		p.GridCellKm = 0
		p.Hint = "Fast run: single-area search, moderate list scroll."
	case intensity <= 40:
		p.Depth = 25
		p.Zoom = 14
		p.TargetCellCount = 50
		p.Hint = "Regional coverage: ~50 grid cells when a state grid is used."
	case intensity <= 60:
		p.Depth = 40
		p.Zoom = 13
		p.TargetCellCount = 150
		p.Hint = "Strong coverage: ~150 grid cells when a state grid is used."
	case intensity <= 80:
		p.Depth = 60
		p.Zoom = 13
		p.TargetCellCount = 400
		p.Hint = "Very strong coverage: ~400 grid cells when a state grid is used."
	default:
		p.Depth = 100
		p.Zoom = 12
		p.TargetCellCount = 850
		p.Hint = "Maximum practical grid density (capped below 900 cells per validation)."
	}

	p.KeywordSuffixes = keywordVariationsForIntensity(business, intensity)

	return p
}

func keywordVariationsForIntensity(business string, intensity int) []string {
	if business == "" {
		return nil
	}

	base := []string{business}

	if intensity <= 20 {
		return base
	}

	if intensity <= 50 {
		return uniqueStrings(append(base,
			fmt.Sprintf("%s near me", business),
		))
	}

	if intensity <= 75 {
		return uniqueStrings(append(base,
			fmt.Sprintf("%s near me", business),
			fmt.Sprintf("best %s", business),
		))
	}

	return uniqueStrings(append(base,
		fmt.Sprintf("%s near me", business),
		fmt.Sprintf("best %s", business),
		fmt.Sprintf("%s services", business),
	))
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))

	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, s)
	}

	return out
}

// GridCellKmForBBox returns a cell size (km) so that estimated cell count is
// at most targetCells (and never above maxGridCells). Smaller km ⇒ more cells.
// If targetCells <= 1, returns 0 (caller should skip grid).
func GridCellKmForBBox(bbox grid.BoundingBox, targetCells int) float64 {
	if targetCells <= 1 {
		return 0
	}

	if targetCells > maxGridCells {
		targetCells = maxGridCells
	}

	lo, hi := 5.0, 500.0
	best := 45.0

	for range 60 {
		mid := (lo + hi) / 2
		c := grid.EstimateCellCount(bbox, mid)

		if c > maxGridCells {
			lo = mid

			continue
		}

		if c > targetCells {
			lo = mid
		} else {
			best = mid
			hi = mid
		}

		if hi-lo < 0.02 {
			break
		}
	}

	return best
}

// DefaultMaxTimeForIntensity scales job wall time with how heavy the run is.
func DefaultMaxTimeForIntensity(intensity int) string {
	switch {
	case intensity <= 20:
		return "45m"
	case intensity <= 40:
		return "3h"
	case intensity <= 60:
		return "8h"
	case intensity <= 80:
		return "16h"
	default:
		return "36h"
	}
}
