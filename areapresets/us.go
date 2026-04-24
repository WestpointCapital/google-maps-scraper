// Package areapresets maps human-friendly area codes to geographic bounding boxes
// used for grid-based scraping (see grid package).
package areapresets

import (
	"sort"
	"strings"

	"github.com/gosom/google-maps-scraper/grid"
)

// SelectOption is one entry for an HTML <select> (US state / DC grid presets).
type SelectOption struct {
	Code  string
	Label string
}

// usStateBoxes are approximate WGS84 bounding boxes (minLat, minLon, maxLat, maxLon).
// Sources: USGS / Census–style approximations, rounded for scraping use.
var usStateBoxes = map[string]grid.BoundingBox{
	"us-al": {30.19, -88.47, 35.01, -84.89},
	"us-ak": {51.20, -179.15, 71.54, -129.99},
	"us-az": {31.33, -114.82, 37.00, -109.05},
	"us-ar": {33.00, -94.62, 36.50, -89.62},
	"us-ca": {32.51, -124.48, 42.01, -114.13},
	"us-co": {36.99, -109.06, 41.00, -102.04},
	"us-ct": {40.98, -73.73, 42.05, -71.79},
	"us-de": {38.45, -75.79, 39.84, -74.85},
	"us-dc": {38.79, -77.12, 38.99, -76.91},
	"us-fl": {24.52, -87.63, 31.00, -80.03},
	"us-ga": {30.36, -85.61, 34.99, -80.84},
	"us-hi": {18.91, -160.25, 22.24, -154.81},
	"us-id": {41.99, -117.22, 49.00, -111.05},
	"us-il": {36.97, -91.51, 42.51, -87.02},
	"us-in": {37.77, -88.10, 41.76, -84.78},
	"us-ia": {40.38, -96.64, 43.50, -90.14},
	"us-ks": {36.99, -102.05, 40.00, -94.59},
	"us-ky": {36.50, -89.57, 39.15, -81.96},
	"us-la": {28.93, -94.04, 33.02, -88.82},
	"us-me": {42.98, -71.08, 47.46, -66.95},
	"us-md": {37.91, -79.49, 39.72, -75.05},
	"us-ma": {41.24, -73.50, 42.89, -69.93},
	"us-mi": {41.70, -90.42, 48.31, -82.12},
	"us-mn": {43.50, -97.24, 49.38, -89.49},
	"us-ms": {30.17, -91.66, 35.00, -88.09},
	"us-mo": {35.99, -95.77, 40.61, -89.10},
	"us-mt": {44.36, -116.05, 49.00, -104.04},
	"us-ne": {39.99, -104.05, 43.00, -95.31},
	"us-nv": {35.00, -120.01, 42.00, -114.04},
	"us-nh": {42.70, -72.56, 45.31, -70.70},
	"us-nj": {38.93, -75.56, 41.36, -73.89},
	"us-nm": {31.33, -109.05, 37.00, -103.00},
	"us-ny": {40.50, -79.76, 45.02, -71.86},
	"us-nc": {33.84, -84.32, 36.59, -75.46},
	"us-nd": {45.94, -104.05, 49.00, -96.56},
	"us-oh": {38.40, -84.82, 42.32, -80.52},
	"us-ok": {33.62, -103.00, 37.00, -94.43},
	"us-or": {41.99, -124.57, 46.29, -116.46},
	"us-pa": {39.72, -80.52, 42.54, -74.69},
	"us-ri": {41.15, -71.91, 42.02, -71.05},
	"us-sc": {32.03, -83.35, 35.21, -78.54},
	"us-sd": {42.48, -104.06, 45.94, -96.44},
	"us-tn": {34.98, -90.31, 36.68, -81.65},
	"us-tx": {25.84, -106.65, 36.50, -93.51},
	"us-ut": {36.99, -114.05, 42.00, -109.04},
	"us-vt": {42.73, -73.09, 45.01, -71.51},
	"us-va": {36.54, -83.67, 39.47, -75.24},
	"us-wa": {45.54, -124.79, 49.00, -116.92},
	"us-wv": {37.20, -82.64, 40.64, -77.72},
	"us-wi": {42.49, -92.90, 47.31, -86.25},
	"us-wy": {40.99, -111.05, 45.01, -104.05},
}

// usStateLabels maps preset codes to a short English label.
var usStateLabels = map[string]string{
	"us-al": "Alabama", "us-ak": "Alaska", "us-az": "Arizona", "us-ar": "Arkansas",
	"us-ca": "California", "us-co": "Colorado", "us-ct": "Connecticut", "us-de": "Delaware",
	"us-dc": "District of Columbia", "us-fl": "Florida", "us-ga": "Georgia", "us-hi": "Hawaii",
	"us-id": "Idaho", "us-il": "Illinois", "us-in": "Indiana", "us-ia": "Iowa",
	"us-ks": "Kansas", "us-ky": "Kentucky", "us-la": "Louisiana", "us-me": "Maine",
	"us-md": "Maryland", "us-ma": "Massachusetts", "us-mi": "Michigan", "us-mn": "Minnesota",
	"us-ms": "Mississippi", "us-mo": "Missouri", "us-mt": "Montana", "us-ne": "Nebraska",
	"us-nv": "Nevada", "us-nh": "New Hampshire", "us-nj": "New Jersey", "us-nm": "New Mexico",
	"us-ny": "New York", "us-nc": "North Carolina", "us-nd": "North Dakota", "us-oh": "Ohio",
	"us-ok": "Oklahoma", "us-or": "Oregon", "us-pa": "Pennsylvania", "us-ri": "Rhode Island",
	"us-sc": "South Carolina", "us-sd": "South Dakota", "us-tn": "Tennessee", "us-tx": "Texas",
	"us-ut": "Utah", "us-vt": "Vermont", "us-va": "Virginia", "us-wa": "Washington",
	"us-wv": "West Virginia", "us-wi": "Wisconsin", "us-wy": "Wyoming",
}

// USStateBoundingBox returns the bounding box for a preset code such as "us-ca".
// Codes are case-insensitive; surrounding space is trimmed.
func USStateBoundingBox(preset string) (grid.BoundingBox, bool) {
	key := strings.ToLower(strings.TrimSpace(preset))
	box, ok := usStateBoxes[key]

	return box, ok
}

// USStateSearchRegion returns text for biasing Maps keyword search to a US state
// (e.g. "California, United States"). ok is false if preset is unknown.
func USStateSearchRegion(preset string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(preset))
	label, ok := usStateLabels[key]
	if !ok {
		return "", false
	}

	return label + ", United States", true
}

// USSelectOptions returns US state / DC grid presets sorted by label for HTML forms.
func USSelectOptions() []SelectOption {
	codes := make([]string, 0, len(usStateBoxes))
	for k := range usStateBoxes {
		codes = append(codes, k)
	}

	sort.Strings(codes)

	out := make([]SelectOption, 0, len(codes))
	for _, code := range codes {
		label := usStateLabels[code]
		if label == "" {
			label = code
		}

		out = append(out, SelectOption{Code: code, Label: label})
	}

	return out
}
