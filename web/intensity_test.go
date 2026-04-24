package web

import (
	"testing"

	"github.com/gosom/google-maps-scraper/areapresets"
	"github.com/gosom/google-maps-scraper/grid"
	"github.com/stretchr/testify/require"
)

func TestBuildIntensityPlan_keywords(t *testing.T) {
	t.Parallel()

	p := BuildIntensityPlan("café", 100)
	require.GreaterOrEqual(t, len(p.KeywordSuffixes), 3)
	require.Contains(t, p.KeywordSuffixes[0], "café")
}

func TestGridCellKmForBBoxCalifornia_underCap(t *testing.T) {
	t.Parallel()

	bbox, ok := areapresets.USStateBoundingBox("us-ca")
	require.True(t, ok)

	km := GridCellKmForBBox(bbox, 850)
	require.Greater(t, km, 0.0)

	c := grid.EstimateCellCount(bbox, km)
	require.LessOrEqual(t, c, 900)
}
