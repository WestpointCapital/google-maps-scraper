package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLocationPlan_stateCalifornia(t *testing.T) {
	t.Parallel()

	p, err := ParseLocationPlan("California")
	require.NoError(t, err)
	require.Equal(t, LocationKindStateGrid, p.Kind)
	require.Equal(t, "us-ca", p.AreaPreset)
}

func TestParseLocationPlan_coords(t *testing.T) {
	t.Parallel()

	p, err := ParseLocationPlan(" 37.77 , -122.42 ")
	require.NoError(t, err)
	require.Equal(t, LocationKindCoords, p.Kind)
	require.Equal(t, "37.77", p.Lat)
	require.Equal(t, "-122.42", p.Lon)
}

func TestParseLocationPlan_zip(t *testing.T) {
	t.Parallel()

	p, err := ParseLocationPlan("90210")
	require.NoError(t, err)
	require.Equal(t, LocationKindZIP, p.Kind)
	require.Contains(t, p.SearchRegion, "90210")
}
