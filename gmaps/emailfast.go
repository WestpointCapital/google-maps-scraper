package gmaps

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Tunables for the fast (Playwright-free) email enrichment path.
// These intentionally cap work per site so a single slow website cannot
// stall the whole enrichment pool.
const (
	fastEmailMaxBodySize = 2 << 20 // 2 MiB — enough for typical landing/contact pages
	fastEmailMaxSubpages = 2       // visit at most this many extra contact-style pages
	fastEmailMaxRedirect = 5
)

// fastEmailUserAgent is a desktop UA so generic anti-bot rules accept the request.
const fastEmailUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// NewFastEmailHTTPClient returns an *http.Client tuned for the async email
// enricher. It does not use Playwright; all work is plain Go HTTP, so a
// single goroutine can process many sites per second.
func NewFastEmailHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}

	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= fastEmailMaxRedirect {
				return http.ErrUseLastResponse
			}

			return nil
		},
	}
}

// FetchEmailsForEntry visits entry.WebSite (and a small number of contact-style
// subpages) using plain HTTP and merges any discovered email addresses into
// entry.Emails. Returns nil and leaves the entry unchanged if the website is
// missing or known to be a social profile.
//
// Safe to call concurrently for different *Entry values from many goroutines.
func FetchEmailsForEntry(ctx context.Context, client *http.Client, entry *Entry) error {
	if entry == nil || !entry.IsWebsiteValidForEmail() {
		return nil
	}

	if client == nil {
		client = NewFastEmailHTTPClient(0)
	}

	target := normalizeGoogleURL(entry.WebSite)
	if target == "" {
		return nil
	}

	finalURL, body, doc, err := fetchPageHTML(ctx, client, target)
	if err != nil {
		return err
	}

	if doc != nil {
		mergeExtractedEmailsIntoEntry(entry, doc, body)
	}

	if finalURL == nil || doc == nil {
		return nil
	}

	nextURLs := discoverContactPageURLs(doc, finalURL)
	if len(nextURLs) > fastEmailMaxSubpages {
		nextURLs = nextURLs[:fastEmailMaxSubpages]
	}

	for _, sub := range nextURLs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, b2, d2, ferr := fetchPageHTML(ctx, client, sub)
		if ferr != nil {
			continue
		}

		if d2 != nil {
			mergeExtractedEmailsIntoEntry(entry, d2, b2)
		}
	}

	return nil
}

// fetchPageHTML returns (finalURL, body, parsedDoc, err). It only parses
// HTML/text content types and caps the body size so a malicious 1 GB page
// can't OOM the worker.
func fetchPageHTML(ctx context.Context, client *http.Client, target string) (*url.URL, []byte, *goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	req.Header.Set("User-Agent", fastEmailUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return resp.Request.URL, nil, nil, nil
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !contentTypeIsTextual(ct) {
		return resp.Request.URL, nil, nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, fastEmailMaxBodySize))
	if err != nil {
		return resp.Request.URL, nil, nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return resp.Request.URL, body, nil, nil
	}

	return resp.Request.URL, body, doc, nil
}

func contentTypeIsTextual(ct string) bool {
	return strings.Contains(ct, "html") ||
		strings.Contains(ct, "xml") ||
		strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "json")
}
