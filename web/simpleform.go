package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SimpleFormSubmitted reports whether the request used the simplified scrape form.
func SimpleFormSubmitted(r *http.Request) bool {
	return strings.TrimSpace(r.Form.Get("simple_mode")) == "1"
}

// ParseSimpleScrapeForm builds ParsedScrapeForm from simple_mode fields.
// Caller must have called r.ParseForm().
func ParseSimpleScrapeForm(r *http.Request) (*ParsedScrapeForm, error) {
	business := strings.TrimSpace(r.Form.Get("business_type"))
	if business == "" {
		return nil, fmt.Errorf("business type is required")
	}

	locRaw := strings.TrimSpace(r.Form.Get("location"))
	if locRaw == "" {
		return nil, fmt.Errorf("location is required")
	}

	intensity, err := strconv.Atoi(strings.TrimSpace(r.Form.Get("intensity")))
	if err != nil {
		return nil, fmt.Errorf("invalid intensity: %w", err)
	}

	maxTimeStr := strings.TrimSpace(r.Form.Get("maxtime"))
	if maxTimeStr == "" {
		maxTimeStr = DefaultMaxTimeForIntensity(intensity)
	}

	maxTime, err := time.ParseDuration(maxTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid max time: %w", err)
	}

	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		name = fmt.Sprintf("%s — %s", business, locRaw)
	}

	lang := strings.TrimSpace(r.Form.Get("lang"))
	if lang == "" {
		lang = "en"
	}

	var proxies []string

	for _, p := range strings.Split(r.Form.Get("proxies"), "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		proxies = append(proxies, p)
	}

	data, err := BuildJobDataFromSimpleRequest(
		business,
		locRaw,
		intensity,
		r.Form.Get("email") == "on",
		r.Form.Get("extra_reviews") == "on",
		lang,
		proxies,
	)
	if err != nil {
		return nil, err
	}

	return &ParsedScrapeForm{
		JobName: name,
		Data:    data,
		MaxTime: maxTime,
	}, nil
}
