package gmaps

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func Test_mergeUniqueEmailSlices(t *testing.T) {
	t.Parallel()

	a := []string{"a@b.com", "c@d.com"}
	b := []string{"a@b.com", "e@f.com"}

	got := mergeUniqueEmailSlices(a, b)
	require.ElementsMatch(t, []string{"a@b.com", "c@d.com", "e@f.com"}, got)
}

func Test_mergeExtractedEmails_mailtoAndRegex(t *testing.T) {
	t.Parallel()

	html := `<html><body>
		<a href="mailto:hello@example.com">Mail us</a>
		<p>Also reach us at support@example.org for help.</p>
		</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)

	got := mergeExtractedEmails(doc, []byte(html))
	require.Contains(t, got, "hello@example.com")
	require.Contains(t, got, "support@example.org")
}

func Test_discoverContactPageURLs(t *testing.T) {
	t.Parallel()

	html := `<html><body>
		<a href="/">Home</a>
		<a href="/contact">Contact</a>
		<a href="https://evil.example/phish">Offsite</a>
		<a href="/about-us">About</a>
		<a href="/photo.jpg">Pic</a>
		</body></html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)

	base, err := url.Parse("https://shop.example/foo")
	require.NoError(t, err)

	got := discoverContactPageURLs(doc, base)
	require.Contains(t, got, "https://shop.example/contact")
	require.Contains(t, got, "https://shop.example/about-us")
	for _, u := range got {
		require.NotContains(t, u, "evil.example")
		require.NotContains(t, u, "photo.jpg")
	}
}

func Test_urlsRoughlyEqual(t *testing.T) {
	t.Parallel()

	require.True(t, urlsRoughlyEqual("https://x.com/a", "https://x.com/a/"))
	require.True(t, urlsRoughlyEqual("https://x.com/a", "https://x.com/a"))
}
