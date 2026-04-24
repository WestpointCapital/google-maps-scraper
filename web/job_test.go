package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJobDataValidate_gridCalifornia(t *testing.T) {
	t.Parallel()

	j := Job{
		ID:     "x",
		Name:   "n",
		Status: StatusPending,
		Date:   time.Now().UTC(),
		Data: JobData{
			Keywords:   []string{"cosmetic clinic"},
			Lang:       "en",
			Zoom:       14,
			Depth:      5,
			MaxTime:    10 * time.Minute,
			AreaPreset: "us-ca",
			GridCellKm: 50,
		},
	}

	require.NoError(t, j.Validate())
}

func TestJobDataValidate_fastModeWithGridRejected(t *testing.T) {
	t.Parallel()

	j := Job{
		ID:     "x",
		Name:   "n",
		Status: StatusPending,
		Date:   time.Now().UTC(),
		Data: JobData{
			Keywords:   []string{"q"},
			Lang:       "en",
			Zoom:       14,
			Depth:      3,
			MaxTime:    10 * time.Minute,
			FastMode:   true,
			Lat:        "36",
			Lon:        "-119",
			AreaPreset: "us-ca",
		},
	}

	require.Error(t, j.Validate())
}
