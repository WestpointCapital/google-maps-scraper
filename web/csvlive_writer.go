package web

import (
	"context"
	"encoding/csv"
	"fmt"
	"reflect"
	"sync"

	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/scrapemate"
)

var _ scrapemate.ResultWriter = (*CSVLiveWriter)(nil)

// CSVLiveWriter is the single scrapemate.ResultWriter for the web runner.
//
// Responsibilities:
//   - mirror scrapemate's built-in CSV writer (header + row per place);
//   - upsert each place into job_results so the live UI can poll it;
//   - if email enrichment is enabled, hand the entry to the async enricher
//     and mark the row 'pending' so the UI can show "X/Y emails fetched".
//
// The main scrape pipeline is NEVER blocked by email enrichment — submissions
// are non-blocking and the row is persisted with whatever Google Maps already
// returned (often nothing for emails). The enricher then UPDATEs the row in
// the background.
type CSVLiveWriter struct {
	w          *csv.Writer
	sink       JobResultSink
	enricher   *EmailEnricher
	jobID      string
	wantEmails bool
	once       sync.Once
}

// NewCSVLiveWriter wires the CSV writer + DB sink + (optional) async email
// enricher. enricher may be nil, in which case rows are persisted without
// any email status and no enrichment is triggered.
func NewCSVLiveWriter(w *csv.Writer, sink JobResultSink, enricher *EmailEnricher, jobID string, wantEmails bool) *CSVLiveWriter {
	return &CSVLiveWriter{
		w:          w,
		sink:       sink,
		enricher:   enricher,
		jobID:      jobID,
		wantEmails: wantEmails,
	}
}

// Run implements scrapemate.ResultWriter. It is the SOLE consumer of the
// Results channel: split that across multiple writers and you lose rows.
func (c *CSVLiveWriter) Run(ctx context.Context, in <-chan scrapemate.Result) error {
	for {
		if s, ok := c.sink.(*Service); ok {
			if err := s.WaitIfPaused(ctx, c.jobID); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result, ok := <-in:
			if !ok {
				return c.w.Error()
			}

			elements, err := csvElementsFromData(result.Data)
			if err != nil {
				return err
			}

			if len(elements) == 0 {
				continue
			}

			c.once.Do(func() {
				_ = c.w.Write(elements[0].CsvHeaders())
			})

			for _, element := range elements {
				if err := c.w.Write(element.CsvRow()); err != nil {
					return err
				}

				entry, entryOK := element.(*gmaps.Entry)
				if !entryOK || c.sink == nil {
					continue
				}

				initial := c.initialEmailStatus(entry)

				if err := c.sink.UpsertJobResult(ctx, c.jobID, entry, initial); err != nil {
					return err
				}

				// Hand off to the async enricher only when:
				//   - the user actually asked for emails,
				//   - we have a usable website,
				//   - the row is fresh (no emails already).
				if c.wantEmails && c.enricher != nil && initial == EmailStatusPending {
					placeKey := PlaceDedupeKey(entry)
					_ = c.enricher.Submit(c.jobID, placeKey, entry)
				}
			}

			c.w.Flush()
		}
	}
}

// initialEmailStatus decides what email_status to write on first INSERT:
//   - "" when emails are disabled (UI never shows the email progress line);
//   - "done" when Google Maps already returned at least one email;
//   - "skipped" when the website is missing or a known social profile;
//   - "pending" otherwise (the enricher will take it from here).
func (c *CSVLiveWriter) initialEmailStatus(e *gmaps.Entry) string {
	if !c.wantEmails {
		return ""
	}

	if len(e.Emails) > 0 {
		return EmailStatusDone
	}

	if !e.IsWebsiteValidForEmail() {
		return EmailStatusSkipped
	}

	return EmailStatusPending
}

func csvElementsFromData(data any) ([]scrapemate.CsvCapable, error) {
	var elements []scrapemate.CsvCapable

	if data == nil {
		return nil, nil
	}

	if interfaceIsSlice(data) {
		s := reflect.ValueOf(data)

		for i := 0; i < s.Len(); i++ {
			val := s.Index(i).Interface()
			if element, ok := val.(scrapemate.CsvCapable); ok {
				elements = append(elements, element)
			} else {
				return nil, fmt.Errorf("%w: unexpected data type: %T", scrapemate.ErrorNotCsvCapable, val)
			}
		}
	} else if element, ok := data.(scrapemate.CsvCapable); ok {
		elements = append(elements, element)
	} else {
		return nil, fmt.Errorf("%w: unexpected data type: %T", scrapemate.ErrorNotCsvCapable, data)
	}

	return elements, nil
}

func interfaceIsSlice(t any) bool {
	if t == nil {
		return false
	}

	switch reflect.TypeOf(t).Kind() { //nolint:exhaustive
	case reflect.Slice:
		return true
	default:
		return false
	}
}
