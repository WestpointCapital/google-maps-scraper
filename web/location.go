package web

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gosom/google-maps-scraper/areapresets"
)

// LocationKind describes how the free-text location field was interpreted.
type LocationKind string

const (
	LocationKindStateGrid LocationKind = "state_grid"
	LocationKindCoords    LocationKind = "coordinates"
	LocationKindZIP       LocationKind = "zip"
	LocationKindRegion    LocationKind = "region"
)

// LocationPlan is the outcome of parsing the "where" field for simple mode.
type LocationPlan struct {
	Kind LocationKind

	// AreaPreset is a grid preset code (e.g. us-ca) when Kind==LocationKindStateGrid.
	AreaPreset string

	// SearchRegion biases keyword text ("… in <region>") when not using grid or as extra context.
	SearchRegion string

	Lat string
	Lon string
}

var (
	zipUS     = regexp.MustCompile(`^\s*(\d{5})(-\d{4})?\s*$`)
	coordPair = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)
)

// ParseLocationPlan interprets a user-entered location string.
func ParseLocationPlan(raw string) (LocationPlan, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return LocationPlan{}, fmt.Errorf("location is required")
	}

	if m := coordPair.FindStringSubmatch(s); len(m) == 3 {
		return LocationPlan{
			Kind: LocationKindCoords,
			Lat:  strings.TrimSpace(m[1]),
			Lon:  strings.TrimSpace(m[2]),
		}, nil
	}

	if zipUS.MatchString(s) {
		zip := strings.TrimSpace(s)

		return LocationPlan{
			Kind:         LocationKindZIP,
			SearchRegion: zip + ", USA",
		}, nil
	}

	if code, ok := matchUSStatePreset(s); ok {
		return LocationPlan{
			Kind:       LocationKindStateGrid,
			AreaPreset: code,
		}, nil
	}

	region := s
	if !strings.Contains(strings.ToLower(region), "usa") &&
		!strings.Contains(strings.ToLower(region), "united states") {
		region = region + ", USA"
	}

	return LocationPlan{
		Kind:         LocationKindRegion,
		SearchRegion: region,
	}, nil
}

func matchUSStatePreset(s string) (code string, ok bool) {
	key := strings.TrimSpace(strings.ToLower(s))
	if i := strings.Index(key, ","); i >= 0 {
		key = strings.TrimSpace(key[:i])
	}

	// Common abbreviations → preset codes
	abbr := map[string]string{
		"al": "us-al", "ak": "us-ak", "az": "us-az", "ar": "us-ar", "ca": "us-ca",
		"co": "us-co", "ct": "us-ct", "de": "us-de", "dc": "us-dc", "fl": "us-fl",
		"ga": "us-ga", "hi": "us-hi", "id": "us-id", "il": "us-il", "in": "us-in",
		"ia": "us-ia", "ks": "us-ks", "ky": "us-ky", "la": "us-la", "me": "us-me",
		"md": "us-md", "ma": "us-ma", "mi": "us-mi", "mn": "us-mn", "ms": "us-ms",
		"mo": "us-mo", "mt": "us-mt", "ne": "us-ne", "nv": "us-nv", "nh": "us-nh",
		"nj": "us-nj", "nm": "us-nm", "ny": "us-ny", "nc": "us-nc", "nd": "us-nd",
		"oh": "us-oh", "ok": "us-ok", "or": "us-or", "pa": "us-pa", "ri": "us-ri",
		"sc": "us-sc", "sd": "us-sd", "tn": "us-tn", "tx": "us-tx", "ut": "us-ut",
		"vt": "us-vt", "va": "us-va", "wa": "us-wa", "wv": "us-wv", "wi": "us-wi",
		"wy": "us-wy",
	}

	if c, ok2 := abbr[key]; ok2 {
		if _, ok3 := areapresets.USStateBoundingBox(c); ok3 {
			return c, true
		}
	}

	for _, opt := range areapresets.USSelectOptions() {
		lbl := strings.ToLower(opt.Label)
		if key == lbl || key == strings.ToLower(opt.Code) {
			return opt.Code, true
		}
	}

	return "", false
}
