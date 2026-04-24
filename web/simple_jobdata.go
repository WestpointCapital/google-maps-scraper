package web

import (
	"fmt"
	"strings"

	"github.com/gosom/google-maps-scraper/areapresets"
)

// BuildJobDataFromSimpleRequest builds JobData from quick-search fields (shared by
// ParseSimpleScrapeForm and preset-from-simple JSON).
func BuildJobDataFromSimpleRequest(
	business, location string,
	intensity int,
	email, extraReviews bool,
	lang string,
	proxies []string,
) (JobData, error) {
	business = strings.TrimSpace(business)
	if business == "" {
		return JobData{}, fmt.Errorf("business type is required")
	}

	locRaw := strings.TrimSpace(location)
	if locRaw == "" {
		return JobData{}, fmt.Errorf("location is required")
	}

	loc, err := ParseLocationPlan(locRaw)
	if err != nil {
		return JobData{}, err
	}

	ip := BuildIntensityPlan(business, intensity)

	if lang == "" {
		lang = "en"
	}

	data := JobData{
		Keywords:     ip.KeywordSuffixes,
		Lang:         lang,
		Zoom:         ip.Zoom,
		Depth:        ip.Depth,
		Email:        email,
		Lat:          loc.Lat,
		Lon:          loc.Lon,
		SearchRegion: loc.SearchRegion,
		AreaPreset:   "",
		GridCellKm:   0,
		SimpleMode:   true,
		Intensity:    intensity,
		ExtraReviews: extraReviews,
		Proxies:      append([]string(nil), proxies...),
	}

	switch loc.Kind {
	case LocationKindStateGrid:
		if ip.TargetCellCount <= 1 {
			sr, ok := areapresets.USStateSearchRegion(loc.AreaPreset)
			if !ok {
				return JobData{}, fmt.Errorf("unknown state preset")
			}

			data.AreaPreset = ""
			data.SearchRegion = sr
			data.GridCellKm = 0
		} else {
			data.AreaPreset = loc.AreaPreset
			bbox, ok := areapresets.USStateBoundingBox(loc.AreaPreset)
			if !ok {
				return JobData{}, fmt.Errorf("unknown state preset")
			}

			data.GridCellKm = GridCellKmForBBox(bbox, ip.TargetCellCount)
		}

	case LocationKindCoords, LocationKindZIP, LocationKindRegion:
		// coords / region already applied
	}

	return data, nil
}
