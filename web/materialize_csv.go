package web

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gosom/google-maps-scraper/gmaps"
)

// entryFromJobResult rebuilds *gmaps.Entry from persisted job_results (raw_json
// plus emails_json, which the async email enricher updates after insert).
func entryFromJobResult(r JobResult) (*gmaps.Entry, bool) {
	if r.RawJSON == "" {
		return nil, false
	}

	entry := &gmaps.Entry{}
	if err := json.Unmarshal([]byte(r.RawJSON), entry); err != nil {
		return nil, false
	}

	if r.EmailsJSON != "" {
		var emails []string
		if err := json.Unmarshal([]byte(r.EmailsJSON), &emails); err == nil && len(emails) > 0 {
			entry.Emails = emails
		}
	}

	return entry, true
}

func writeJobResultsToCSV(rows []JobResult, w io.Writer) (bool, error) {
	if len(rows) == 0 {
		return false, nil
	}

	cw := csv.NewWriter(w)

	headerWritten := false

	for i := range rows {
		entry, ok := entryFromJobResult(rows[i])
		if !ok {
			continue
		}

		if !headerWritten {
			if err := cw.Write(entry.CsvHeaders()); err != nil {
				return headerWritten, err
			}

			headerWritten = true
		}

		if err := cw.Write(entry.CsvRow()); err != nil {
			return headerWritten, err
		}
	}

	if !headerWritten {
		return false, nil
	}

	cw.Flush()

	return true, cw.Error()
}

// WriteJobCSVFromDB writes a CSV snapshot of job_results to w, merging
// emails_json from the async enricher. Returns ok true when any CSV bytes
// were written (including header).
func (s *Service) WriteJobCSVFromDB(ctx context.Context, jobID string, w io.Writer) (ok bool, err error) {
	if strings.Contains(jobID, "/") || strings.Contains(jobID, "\\") || strings.Contains(jobID, "..") {
		return false, fmt.Errorf("invalid job id")
	}

	rows, err := s.ListJobResults(ctx, jobID, 0)
	if err != nil {
		return false, err
	}

	return writeJobResultsToCSV(rows, w)
}

// MaterializeJobCSV overwrites dataFolder/<jobID>.csv from job_results so the
// on-disk file matches the database (including emails written by the async
// enricher). If there are zero rows, the existing file is left unchanged.
func (s *Service) MaterializeJobCSV(ctx context.Context, jobID string) error {
	if strings.Contains(jobID, "/") || strings.Contains(jobID, "\\") || strings.Contains(jobID, "..") {
		return fmt.Errorf("invalid job id")
	}

	rows, err := s.ListJobResults(ctx, jobID, 0)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	outpath := filepath.Join(s.dataFolder, jobID+".csv")

	f, err := os.Create(outpath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	_, err = writeJobResultsToCSV(rows, f)

	return err
}
