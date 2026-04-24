package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSimpleScrapeForm_californiaGrid(t *testing.T) {
	t.Parallel()

	v := url.Values{}
	v.Set("simple_mode", "1")
	v.Set("business_type", "cosmetic clinic")
	v.Set("location", "California")
	v.Set("intensity", "80")
	v.Set("maxtime", "8h")
	v.Set("email", "on")
	v.Set("lang", "en")

	r := httptest.NewRequest("POST", "/scrape", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	require.NoError(t, r.ParseForm())

	out, err := ParseSimpleScrapeForm(r)
	require.NoError(t, err)
	require.Equal(t, "us-ca", out.Data.AreaPreset)
	require.True(t, out.Data.Email)
	require.Greater(t, out.Data.Depth, 40)
}
