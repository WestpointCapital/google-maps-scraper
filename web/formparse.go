package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ParsedScrapeForm is the result of parsing the scrape / preset HTML form.
type ParsedScrapeForm struct {
	JobName string
	Data    JobData
	MaxTime time.Duration
}

// ParseScrapeForm reads the standard scrape form fields from r (ParseForm must not be called twice without rewind; caller should call r.ParseForm first).
func ParseScrapeForm(r *http.Request) (*ParsedScrapeForm, error) {
	var out ParsedScrapeForm

	maxTimeStr := r.Form.Get("maxtime")
	maxTime, err := time.ParseDuration(maxTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid max time: %w", err)
	}

	out.MaxTime = maxTime
	out.JobName = strings.TrimSpace(r.Form.Get("name"))

	keywordsStr, ok := r.Form["keywords"]
	if !ok {
		return nil, fmt.Errorf("missing keywords field")
	}

	keywords := strings.Split(keywordsStr[0], "\n")
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}

		out.Data.Keywords = append(out.Data.Keywords, k)
	}

	out.Data.Lang = r.Form.Get("lang")

	out.Data.Zoom, err = strconv.Atoi(r.Form.Get("zoom"))
	if err != nil {
		return nil, fmt.Errorf("invalid zoom: %w", err)
	}

	if r.Form.Get("fastmode") == "on" {
		out.Data.FastMode = true
	}

	out.Data.Radius, err = strconv.Atoi(r.Form.Get("radius"))
	if err != nil {
		return nil, fmt.Errorf("invalid radius: %w", err)
	}

	out.Data.Lat = strings.TrimSpace(r.Form.Get("latitude"))
	out.Data.Lon = strings.TrimSpace(r.Form.Get("longitude"))

	out.Data.SearchRegion = strings.TrimSpace(r.Form.Get("search_region"))
	out.Data.AreaPreset = strings.TrimSpace(r.Form.Get("area_preset"))

	gck := strings.TrimSpace(r.Form.Get("grid_cell_km"))
	if gck != "" {
		out.Data.GridCellKm, err = strconv.ParseFloat(gck, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid grid cell size: %w", err)
		}
	}

	out.Data.Depth, err = strconv.Atoi(r.Form.Get("depth"))
	if err != nil {
		return nil, fmt.Errorf("invalid depth: %w", err)
	}

	out.Data.Email = r.Form.Get("email") == "on"

	proxies := strings.Split(r.Form.Get("proxies"), "\n")
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		out.Data.Proxies = append(out.Data.Proxies, p)
	}

	return &out, nil
}
