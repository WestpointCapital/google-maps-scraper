package gmaps

import (
	"context"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/gosom/scrapemate"
	"github.com/mcnijman/go-emailaddress"

	"github.com/gosom/google-maps-scraper/exiter"
)

const maxEmailSubpages = 3

const emailFetchMaxRetries = 2

// contactPagePatterns match URL path or link text (lowercased) for likely contact pages.
var contactPagePatterns = []string{
	"contact", "about", "kontakt", "impressum",
	"team", "info", "reach", "connect",
}

type EmailExtractJobOptions func(*EmailExtractJob)

type EmailExtractJob struct {
	scrapemate.Job

	Entry                   *Entry
	ExitMonitor             exiter.Exiter
	WriterManagedCompletion bool
	// emitToCSV is set in Process to true only when returning the final Entry to CSV writers
	// (never when chaining to EmailSubpageJob with a nil payload).
	emitToCSV bool
}

func NewEmailJob(parentID string, entry *Entry, opts ...EmailExtractJobOptions) *EmailExtractJob {
	const defaultPrio = scrapemate.PriorityHigh

	job := EmailExtractJob{
		Job: scrapemate.Job{
			ID:         uuid.New().String(),
			ParentID:   parentID,
			Method:     "GET",
			URL:        normalizeGoogleURL(entry.WebSite),
			MaxRetries: emailFetchMaxRetries,
			Priority:   defaultPrio,
		},
	}

	job.Entry = entry

	for _, opt := range opts {
		opt(&job)
	}

	return &job
}

func WithEmailJobExitMonitor(exitMonitor exiter.Exiter) EmailExtractJobOptions {
	return func(j *EmailExtractJob) {
		j.ExitMonitor = exitMonitor
	}
}

func WithEmailJobWriterManagedCompletion() EmailExtractJobOptions {
	return func(j *EmailExtractJob) {
		j.WriterManagedCompletion = true
	}
}

func (j *EmailExtractJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
	}()

	log := scrapemate.GetLoggerFromContext(ctx)

	log.Info("Processing email job", "url", j.URL)

	if resp.Error != nil {
		j.emitToCSV = true
		return finalizeEmailPlace(j.ExitMonitor, j.WriterManagedCompletion, j.Entry)
	}

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		j.emitToCSV = true
		return finalizeEmailPlace(j.ExitMonitor, j.WriterManagedCompletion, j.Entry)
	}

	mergeExtractedEmailsIntoEntry(j.Entry, doc, resp.Body)

	base, err := url.Parse(j.URL)
	if err != nil {
		j.emitToCSV = true
		return finalizeEmailPlace(j.ExitMonitor, j.WriterManagedCompletion, j.Entry)
	}

	nextURLs := discoverContactPageURLs(doc, base)
	if len(nextURLs) == 0 {
		j.emitToCSV = true
		return finalizeEmailPlace(j.ExitMonitor, j.WriterManagedCompletion, j.Entry)
	}

	first := nextURLs[0]
	var remaining []string
	if len(nextURLs) > 1 {
		remaining = append(remaining, nextURLs[1:]...)
	}

	sub := newEmailSubpageJob(j.ParentID, j.Entry, first, remaining, j.ExitMonitor, j.WriterManagedCompletion)

	return nil, []scrapemate.IJob{sub}, nil
}

func (j *EmailExtractJob) ProcessOnFetchError() bool {
	return true
}

// BrowserActions renders the page with Playwright so JS-rendered emails are visible.
func (j *EmailExtractJob) BrowserActions(_ context.Context, page scrapemate.BrowserPage) scrapemate.Response {
	return fetchEmailPage(page, j.URL)
}

// UseInResults is true only when Process sets emitToCSV (final row with merged emails).
func (j *EmailExtractJob) UseInResults() bool {
	return j.emitToCSV
}

// --- Email subpage chain (contact / about / …) ---

type EmailSubpageJob struct {
	scrapemate.Job

	Entry                   *Entry
	RemainingURLs           []string
	ExitMonitor             exiter.Exiter
	WriterManagedCompletion bool
	emitToCSV               bool
}

func newEmailSubpageJob(
	parentID string,
	entry *Entry,
	currentURL string,
	remaining []string,
	exit exiter.Exiter,
	writerManaged bool,
) *EmailSubpageJob {
	const defaultPrio = scrapemate.PriorityHigh

	u := normalizeGoogleURL(currentURL)

	j := &EmailSubpageJob{
		Job: scrapemate.Job{
			ID:         uuid.New().String(),
			ParentID:   parentID,
			Method:     "GET",
			URL:        u,
			MaxRetries: emailFetchMaxRetries,
			Priority:   defaultPrio,
		},
		Entry:                   entry,
		RemainingURLs:           remaining,
		ExitMonitor:             exit,
		WriterManagedCompletion: writerManaged,
	}

	return j
}

func (j *EmailSubpageJob) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	defer func() {
		resp.Document = nil
		resp.Body = nil
	}()

	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("Processing email subpage job", "url", j.URL)

	if resp.Error != nil {
		return j.advanceEmailChain()
	}

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return j.advanceEmailChain()
	}

	mergeExtractedEmailsIntoEntry(j.Entry, doc, resp.Body)

	return j.advanceEmailChain()
}

func (j *EmailSubpageJob) advanceEmailChain() (any, []scrapemate.IJob, error) {
	if len(j.RemainingURLs) > 0 {
		next := j.RemainingURLs[0]
		var rest []string
		if len(j.RemainingURLs) > 1 {
			rest = append(rest, j.RemainingURLs[1:]...)
		}

		sub := newEmailSubpageJob(j.ParentID, j.Entry, next, rest, j.ExitMonitor, j.WriterManagedCompletion)

		return nil, []scrapemate.IJob{sub}, nil
	}

	j.emitToCSV = true
	return finalizeEmailPlace(j.ExitMonitor, j.WriterManagedCompletion, j.Entry)
}

func (j *EmailSubpageJob) ProcessOnFetchError() bool {
	return true
}

// BrowserActions renders the page with Playwright so JS-rendered emails are visible.
func (j *EmailSubpageJob) BrowserActions(_ context.Context, page scrapemate.BrowserPage) scrapemate.Response {
	return fetchEmailPage(page, j.URL)
}

// UseInResults is true only when this subpage job finishes the chain (emitToCSV).
func (j *EmailSubpageJob) UseInResults() bool {
	return j.emitToCSV
}

// --- Shared helpers ---

// fetchEmailPage navigates via Playwright and returns body + document.
func fetchEmailPage(page scrapemate.BrowserPage, targetURL string) scrapemate.Response {
	var resp scrapemate.Response

	pageResp, err := page.Goto(targetURL, scrapemate.WaitUntilDOMContentLoaded)
	if err != nil {
		resp.Error = err
		return resp
	}

	const networkSettleTime = 2 * time.Second
	time.Sleep(networkSettleTime)

	resp.URL = pageResp.URL
	resp.StatusCode = pageResp.StatusCode
	resp.Headers = pageResp.Headers

	html, err := page.Content()
	if err != nil {
		resp.Error = err
		return resp
	}

	resp.Body = []byte(html)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		resp.Error = err
		return resp
	}

	resp.Document = doc

	return resp
}

func finalizeEmailPlace(exit exiter.Exiter, writerManaged bool, entry *Entry) (any, []scrapemate.IJob, error) {
	if exit != nil && !writerManaged {
		exit.IncrPlacesCompleted(1)
	}

	return entry, nil, nil
}

func mergeExtractedEmailsIntoEntry(entry *Entry, doc *goquery.Document, body []byte) {
	found := mergeExtractedEmails(doc, body)
	entry.Emails = SanitizeAndDedupeEmails(mergeUniqueEmailSlices(entry.Emails, found))
}

// mergeExtractedEmails combines mailto links and body regex matches (both are used).
func mergeExtractedEmails(doc *goquery.Document, body []byte) []string {
	mail := docEmailExtractor(doc)
	rawRegex := regexEmailExtractor(body)

	regex := make([]string, 0, len(rawRegex))
	for _, r := range rawRegex {
		if e, err := getValidEmail(r); err == nil {
			regex = append(regex, e)
		}
	}

	return mergeUniqueEmailSlices(mail, regex)
}

func mergeUniqueEmailSlices(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))

	for _, s := range a {
		if s == "" {
			continue
		}

		key := strings.ToLower(s)
		if seen[key] {
			continue
		}

		seen[key] = true
		out = append(out, s)
	}

	for _, s := range b {
		if s == "" {
			continue
		}

		key := strings.ToLower(s)
		if seen[key] {
			continue
		}

		seen[key] = true
		out = append(out, s)
	}

	return out
}

type scoredURL struct {
	u     string
	score int
}

func discoverContactPageURLs(doc *goquery.Document, base *url.URL) []string {
	if doc == nil || base == nil || base.Scheme == "" || base.Host == "" {
		return nil
	}

	baseHost := hostnameKey(base.Hostname())

	seen := make(map[string]bool)
	var scored []scoredURL

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return
		}

		abs := resolveMaybeRelativeURL(base, href)
		if abs == "" {
			return
		}

		pu, err := url.Parse(abs)
		if err != nil || pu.Scheme != "http" && pu.Scheme != "https" || pu.Host == "" {
			return
		}

		if hostnameKey(pu.Hostname()) != baseHost {
			return
		}

		lowPath := strings.ToLower(path.Clean(pu.Path))
		if skipEmailDiscoveryPath(lowPath) {
			return
		}

		text := strings.ToLower(strings.TrimSpace(s.Text()))
		score := contactPageScore(lowPath, text)
		if score <= 0 {
			return
		}

		norm := pu.String()
		if urlsRoughlyEqual(norm, base.String()) {
			return
		}

		if seen[norm] {
			return
		}

		seen[norm] = true
		scored = append(scored, scoredURL{u: norm, score: score})
	})

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}

		return scored[i].u < scored[j].u
	})

	out := make([]string, 0, maxEmailSubpages)
	for i := range scored {
		if len(out) >= maxEmailSubpages {
			break
		}

		out = append(out, scored[i].u)
	}

	return out
}

func skipEmailDiscoveryPath(lowPath string) bool {
	if lowPath == "" || lowPath == "/" {
		return true
	}

	ext := path.Ext(lowPath)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico", ".pdf", ".zip", ".css", ".js", ".mp4", ".mp3":
		return true
	default:
		return false
	}
}

func contactPageScore(lowPath, linkText string) int {
	score := 0

	for _, p := range contactPagePatterns {
		if strings.Contains(lowPath, p) {
			score += 3
		}

		if linkText != "" && strings.Contains(linkText, p) {
			score += 2
		}
	}

	return score
}

func urlsRoughlyEqual(a, b string) bool {
	return strings.EqualFold(
		strings.TrimSuffix(strings.TrimSpace(a), "/"),
		strings.TrimSuffix(strings.TrimSpace(b), "/"),
	)
}

func hostnameKey(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "www.")

	return h
}

func resolveMaybeRelativeURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}

	if strings.HasPrefix(href, "//") {
		href = base.Scheme + ":" + href
	}

	pu, err := url.Parse(href)
	if err != nil {
		return ""
	}

	if !pu.IsAbs() {
		pu = base.ResolveReference(pu)
	}

	if pu.Scheme != "http" && pu.Scheme != "https" {
		return ""
	}

	pu.Fragment = ""

	return pu.String()
}

func docEmailExtractor(doc *goquery.Document) []string {
	seen := map[string]bool{}

	var emails []string

	doc.Find("a[href^='mailto:']").Each(func(_ int, s *goquery.Selection) {
		mailto, exists := s.Attr("href")
		if exists {
			value := strings.TrimPrefix(mailto, "mailto:")
			if email, err := getValidEmail(value); err == nil {
				if !seen[email] {
					emails = append(emails, email)
					seen[email] = true
				}
			}
		}
	})

	return emails
}

func regexEmailExtractor(body []byte) []string {
	seen := map[string]bool{}

	var emails []string

	addresses := emailaddress.Find(body, false)
	for i := range addresses {
		if !seen[addresses[i].String()] {
			emails = append(emails, addresses[i].String())
			seen[addresses[i].String()] = true
		}
	}

	return emails
}

func getValidEmail(s string) (string, error) {
	email, err := emailaddress.Parse(strings.TrimSpace(s))
	if err != nil {
		return "", err
	}

	return email.String(), nil
}

// normalizeGoogleURL extracts the actual target URL from Google redirect URLs.
// Google Maps sometimes returns URLs like "/url?q=http://example.com/&opi=..."
// for external website links.
func normalizeGoogleURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	if strings.HasPrefix(rawURL, "/url?q=") {
		fullURL := "https://www.google.com" + rawURL

		parsed, err := url.Parse(fullURL)
		if err != nil {
			return rawURL
		}

		if target := parsed.Query().Get("q"); target != "" {
			return target
		}
	}

	if strings.HasPrefix(rawURL, "/") {
		return "https://www.google.com" + rawURL
	}

	return rawURL
}
