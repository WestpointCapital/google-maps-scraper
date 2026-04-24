package runner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplySearchRegion(t *testing.T) {
	t.Parallel()

	kw := []string{"  cosmetic clinic  ", "dental office"}
	got := ApplySearchRegion(kw, "California, USA")
	require.Equal(t, []string{
		"cosmetic clinic in California, USA",
		"dental office in California, USA",
	}, got)

	// Already contains region: leave unchanged
	got2 := ApplySearchRegion([]string{"cafes in california"}, "California")
	require.Equal(t, []string{"cafes in california"}, got2)

	require.Equal(t, []string{"a"}, ApplySearchRegion([]string{"a"}, ""))
}
