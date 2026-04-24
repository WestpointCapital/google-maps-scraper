package areapresets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUSStateBoundingBox(t *testing.T) {
	t.Parallel()

	box, ok := USStateBoundingBox("us-ca")
	require.True(t, ok)
	require.Less(t, box.MinLat, box.MaxLat)
	require.Less(t, box.MinLon, box.MaxLon)

	_, ok2 := USStateBoundingBox("  US-CA  ")
	require.True(t, ok2)

	_, ok3 := USStateBoundingBox("not-a-state")
	require.False(t, ok3)
}

func TestUSStateSearchRegion(t *testing.T) {
	t.Parallel()

	s, ok := USStateSearchRegion("us-ca")
	require.True(t, ok)
	require.Equal(t, "California, United States", s)

	_, ok2 := USStateSearchRegion("not-real")
	require.False(t, ok2)
}

func TestUSSelectOptionsCoversAllBoxes(t *testing.T) {
	t.Parallel()

	opts := USSelectOptions()
	require.GreaterOrEqual(t, len(opts), 50)

	seen := make(map[string]bool)
	for _, o := range opts {
		require.NotEmpty(t, o.Code)
		require.NotEmpty(t, o.Label)
		require.False(t, seen[o.Code])
		seen[o.Code] = true
	}
}
