package gmaps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeAndDedupeEmails(t *testing.T) {
	t.Parallel()

	in := []string{
		"ok@business.com",
		"OK@business.com",
		"user@domain.com",
		"you@example.com",
		"  ok@business.com  ",
	}
	got := SanitizeAndDedupeEmails(in)
	require.Len(t, got, 1)
	require.Equal(t, "ok@business.com", got[0])
}

func TestEmailsCsvSlotsWidth(t *testing.T) {
	t.Parallel()

	slots := emailsCsvSlots([]string{"a1@real.com", "a2@real.com", "a3@real.com"})
	require.Len(t, slots, CsvEmailColumnCount)
	require.Equal(t, "a1@real.com", slots[0])
	require.Equal(t, "a2@real.com", slots[1])
	require.Equal(t, "a3@real.com", slots[2])
	require.Equal(t, "", slots[3])
}

func TestEntry_CsvRowMatchesCsvHeaders(t *testing.T) {
	t.Parallel()

	e := Entry{
		ID:    "id1",
		Title: "T",
		Emails: []string{
			"first@example.com",
			"second@shop.test",
			"user@domain.com",
		},
	}
	h := e.CsvHeaders()
	r := e.CsvRow()
	require.Equal(t, len(h), len(r), "CSV header and row length must match")
	require.Equal(t, "email_1", h[len(h)-CsvEmailColumnCount])
}
