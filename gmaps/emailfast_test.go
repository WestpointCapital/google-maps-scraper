package gmaps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchEmailsForEntry_HappyPath(t *testing.T) {
	t.Parallel()

	const homeHTML = `<html><body>
		<a href="/contact">Contact</a>
		<a href="/about">About</a>
		<p>Reach hello@example-real.test for inquiries.</p>
	</body></html>`

	const contactHTML = `<html><body>
		<a href="mailto:sales@example-real.test">Sales</a>
		<p>Or write support@example-real.test</p>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch {
		case strings.HasPrefix(r.URL.Path, "/contact"):
			_, _ = w.Write([]byte(contactHTML))
		case strings.HasPrefix(r.URL.Path, "/about"):
			_, _ = w.Write([]byte(`<html><body>nothing here</body></html>`))
		default:
			_, _ = w.Write([]byte(homeHTML))
		}
	}))

	defer srv.Close()

	entry := &Entry{
		Title:   "Test",
		WebSite: srv.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := FetchEmailsForEntry(ctx, NewFastEmailHTTPClient(2*time.Second), entry); err != nil {
		t.Fatalf("FetchEmailsForEntry: %v", err)
	}

	// We expect the body regex to find hello@example-real.test on the home page,
	// the mailto: extractor to grab sales@example-real.test on /contact,
	// and the regex extractor to grab support@example-real.test on /contact.
	want := map[string]bool{
		"hello@example-real.test":   false,
		"sales@example-real.test":   false,
		"support@example-real.test": false,
	}

	for _, e := range entry.Emails {
		if _, ok := want[strings.ToLower(e)]; ok {
			want[strings.ToLower(e)] = true
		}
	}

	for addr, found := range want {
		if !found {
			t.Errorf("missing expected email %q in %v", addr, entry.Emails)
		}
	}
}

func TestFetchEmailsForEntry_NoWebsite(t *testing.T) {
	t.Parallel()

	entry := &Entry{Title: "Test"}

	if err := FetchEmailsForEntry(context.Background(), NewFastEmailHTTPClient(0), entry); err != nil {
		t.Fatalf("FetchEmailsForEntry returned error for missing website: %v", err)
	}

	if len(entry.Emails) != 0 {
		t.Fatalf("expected no emails, got %v", entry.Emails)
	}
}
