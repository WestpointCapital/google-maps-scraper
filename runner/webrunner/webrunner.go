package webrunner

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/areapresets"
	"github.com/gosom/google-maps-scraper/deduper"
	"github.com/gosom/google-maps-scraper/exiter"
	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/google-maps-scraper/grid"
	"github.com/gosom/google-maps-scraper/runner"
	"github.com/gosom/google-maps-scraper/tlmt"
	"github.com/gosom/google-maps-scraper/web"
	"github.com/gosom/google-maps-scraper/web/sqlite"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/scrapemateapp"
	"golang.org/x/sync/errgroup"
)

// decodeJobResultEntry rebuilds a *gmaps.Entry from the raw_json column
// captured at scrape time, then merges in the latest emails column (which
// the async enricher updates after the row is first persisted).
func decodeJobResultEntry(r web.JobResult) (*gmaps.Entry, bool) {
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

type webrunner struct {
	srv      *web.Server
	svc      *web.Service
	cfg      *runner.Config
	enricher *web.EmailEnricher
}

func New(cfg *runner.Config) (runner.Runner, error) {
	if cfg.DataFolder == "" {
		return nil, fmt.Errorf("data folder is required")
	}

	if err := os.MkdirAll(cfg.DataFolder, os.ModePerm); err != nil {
		return nil, err
	}

	const dbfname = "jobs.db"

	store, err := openDatastore(cfg.DataFolder, dbfname)
	if err != nil {
		return nil, err
	}

	svc := web.NewService(store, cfg.DataFolder)

	srv, err := web.New(svc, cfg.Addr)
	if err != nil {
		return nil, err
	}

	enricher := web.NewEmailEnricher(svc, emailEnricherWorkers())

	ans := webrunner{
		srv:      srv,
		svc:      svc,
		cfg:      cfg,
		enricher: enricher,
	}

	return &ans, nil
}

func (w *webrunner) Run(ctx context.Context) error {
	w.enricher.Start(ctx)

	egroup, ctx := errgroup.WithContext(ctx)

	egroup.Go(func() error {
		return w.work(ctx)
	})

	egroup.Go(func() error {
		return w.srv.Start(ctx)
	})

	return egroup.Wait()
}

func (w *webrunner) Close(context.Context) error {
	if w.enricher != nil {
		w.enricher.Close()
	}

	return nil
}

// emailEnricherWorkers honours WEB_EMAIL_WORKERS for ops tuning, otherwise
// defaults to 16 — empirically a sweet spot between throughput and being a
// good HTTP citizen on shared egress.
func emailEnricherWorkers() int {
	const defaultWorkers = 16

	v := strings.TrimSpace(os.Getenv("WEB_EMAIL_WORKERS"))
	if v == "" {
		return defaultWorkers
	}

	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultWorkers
	}

	if n > 128 {
		n = 128
	}

	return n
}

func openDatastore(dataFolder, dbfname string) (web.Datastore, error) {
	tursoURL := strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL"))
	tursoToken := strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN"))

	if tursoURL != "" && tursoToken != "" {
		log.Println("web datastore: using Turso (libsql)")

		return sqlite.NewTurso(tursoURL, tursoToken)
	}

	dbpath := filepath.Join(dataFolder, dbfname)

	return sqlite.New(dbpath)
}

// mapsBiasCoords returns "lat,lon" for Google Maps URL centering, or empty if unset.
// "0"/"0" is treated as unset so the map is not anchored off the African coast by mistake.
func mapsBiasCoords(lat, lon string) string {
	la := strings.TrimSpace(lat)
	lo := strings.TrimSpace(lon)
	if la == "" || lo == "" {
		return ""
	}

	if la == "0" && lo == "0" {
		return ""
	}

	return la + "," + lo
}

func (w *webrunner) work(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			jobs, err := w.svc.SelectPending(ctx)
			if err != nil {
				return err
			}

			for i := range jobs {
				select {
				case <-ctx.Done():
					return nil
				default:
					t0 := time.Now().UTC()
					if err := w.scrapeJob(ctx, &jobs[i]); err != nil {
						params := map[string]any{
							"job_count": len(jobs[i].Data.Keywords),
							"duration":  time.Now().UTC().Sub(t0).String(),
							"error":     err.Error(),
						}

						evt := tlmt.NewEvent("web_runner", params)

						_ = runner.Telemetry().Send(ctx, evt)

						log.Printf("error scraping job %s: %v", jobs[i].ID, err)
					} else {
						params := map[string]any{
							"job_count": len(jobs[i].Data.Keywords),
							"duration":  time.Now().UTC().Sub(t0).String(),
						}

						_ = runner.Telemetry().Send(ctx, tlmt.NewEvent("web_runner", params))

						log.Printf("job %s scraped successfully", jobs[i].ID)
					}
				}
			}
		}
	}
}

func (w *webrunner) scrapeJob(ctx context.Context, job *web.Job) error {
	job.Status = web.StatusWorking

	err := w.svc.Update(ctx, job)
	if err != nil {
		return err
	}

	if len(job.Data.Keywords) == 0 {
		job.Status = web.StatusFailed

		return w.svc.Update(ctx, job)
	}

	outpath := filepath.Join(w.cfg.DataFolder, job.ID+".csv")

	outfile, err := os.Create(outpath)
	if err != nil {
		return err
	}

	// outfileClosed lets us safely close-once: setupMate writes through this
	// file during the main scrape, but we then re-open it to overwrite the
	// CSV with enriched emails before reporting success.
	closeOutfile := func() {
		if outfile != nil {
			_ = outfile.Close()
			outfile = nil
		}
	}

	defer closeOutfile()

	mate, err := w.setupMate(ctx, outfile, job)
	if err != nil {
		job.Status = web.StatusFailed

		err2 := w.svc.Update(ctx, job)
		if err2 != nil {
			log.Printf("failed to update job status: %v", err2)
		}

		return err
	}

	defer mate.Close()

	coords := mapsBiasCoords(job.Data.Lat, job.Data.Lon)

	keywords := runner.ApplySearchRegion(job.Data.Keywords, job.Data.SearchRegion)

	preset := strings.ToLower(strings.TrimSpace(job.Data.AreaPreset))
	if preset != "" && preset != "none" {
		if stateRegion, ok := areapresets.USStateSearchRegion(job.Data.AreaPreset); ok {
			keywords = runner.ApplySearchRegion(keywords, stateRegion)
		}
	}

	queryReader := strings.NewReader(strings.Join(keywords, "\n"))

	dedup := deduper.New()
	exitMonitor := exiter.New()

	var seedJobs []scrapemate.IJob

	log.Printf("[job %s] area_preset=%q search_region=%q", job.ID, job.Data.AreaPreset, job.Data.SearchRegion)
	// Email enrichment is handled by an out-of-band HTTP-only worker pool
	// (see web.EmailEnricher) so we deliberately disable in-pipeline email
	// chaining inside scrapemate. This lets the main Google Maps scrape
	// run at full speed without waiting on slow website fetches.
	const seedEmailsInline = false

	switch {
	case preset != "" && preset != "none":
		bbox, ok := areapresets.USStateBoundingBox(job.Data.AreaPreset)
		if !ok {
			return fmt.Errorf("unknown area preset: %s", job.Data.AreaPreset)
		}

		cellKm := job.Data.GridCellKm
		if cellKm <= 0 {
			cellKm = 45
		}

		cellCount := grid.EstimateCellCount(bbox, cellKm)
		log.Printf("[job %s] using grid preset=%q bbox=%v cellKm=%.0f cells~%d", job.ID, preset, bbox, cellKm, cellCount)

		if cellCount > 900 {
			return fmt.Errorf("grid too large (~%d cells); increase grid cell size", cellCount)
		}

		seedJobs, err = runner.CreateGridSeedJobs(
			job.Data.Lang,
			queryReader,
			job.Data.Depth,
			seedEmailsInline,
			bbox,
			cellKm,
			job.Data.Zoom,
			dedup,
			exitMonitor,
			w.cfg.ExtraReviews || job.Data.ExtraReviews,
		)
	default:
		log.Printf("[job %s] no grid preset – using regular seed jobs", job.ID)
		seedJobs, err = runner.CreateSeedJobs(
			job.Data.FastMode,
			job.Data.Lang,
			queryReader,
			job.Data.Depth,
			seedEmailsInline,
			coords,
			job.Data.Zoom,
			func() float64 {
				if job.Data.Radius <= 0 {
					return 10000 // 10 km
				}

				return float64(job.Data.Radius)
			}(),
			dedup,
			exitMonitor,
			w.cfg.ExtraReviews || job.Data.ExtraReviews,
		)
	}
	if err != nil {
		err2 := w.svc.Update(ctx, job)
		if err2 != nil {
			log.Printf("failed to update job status: %v", err2)
		}

		return err
	}

	if len(seedJobs) > 0 {
		exitMonitor.SetSeedCount(len(seedJobs))

		allowedSeconds := max(60, len(seedJobs)*10*job.Data.Depth/50+120)

		if job.Data.MaxTime > 0 {
			if job.Data.MaxTime.Seconds() < 180 {
				allowedSeconds = 180
			} else {
				allowedSeconds = int(job.Data.MaxTime.Seconds())
			}
		}

		log.Printf("running job %s with %d seed jobs and %d allowed seconds", job.ID, len(seedJobs), allowedSeconds)

		mateCtx, cancel := context.WithTimeout(ctx, time.Duration(allowedSeconds)*time.Second)
		defer cancel()

		exitMonitor.SetCancelFunc(cancel)

		go exitMonitor.Run(mateCtx)

		err = mate.Start(mateCtx, seedJobs...)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			cancel()

			err2 := w.svc.Update(ctx, job)
			if err2 != nil {
				log.Printf("failed to update job status: %v", err2)
			}

			return err
		}

		cancel()
	}

	mate.Close()

	// Main scrape is done; let the async email enricher finish for this job
	// before reporting success and rewriting the CSV. We cap the wait so a
	// hung site can't keep the job in 'working' forever.
	if job.Data.Email && w.enricher != nil {
		drainBudget := emailDrainBudget(job.Data.MaxTime)
		log.Printf("[job %s] waiting up to %s for %d pending email enrichments",
			job.ID, drainBudget, w.enricher.PendingForJob(job.ID))

		w.enricher.WaitForJob(ctx, job.ID, drainBudget)
	}

	// Rewrite the CSV from the database so the downloadable file includes
	// emails the async enricher produced after the main scrape finished.
	closeOutfile()

	if err := w.regenerateCSV(ctx, job.ID, outpath); err != nil {
		log.Printf("[job %s] CSV regen failed: %v", job.ID, err)
	}

	job.Status = web.StatusOK

	return w.svc.Update(ctx, job)
}

// emailDrainBudget bounds how long we keep a job in 'working' while waiting
// for the async email enricher to finish. We never wait longer than the
// user's MaxTime budget for the whole job, and we always grant at least a
// small floor so quick scrapes still get their emails.
func emailDrainBudget(maxTime time.Duration) time.Duration {
	const (
		floor = 90 * time.Second
		ceil  = 30 * time.Minute
	)

	if maxTime <= 0 {
		return ceil
	}

	half := maxTime / 2
	switch {
	case half < floor:
		return floor
	case half > ceil:
		return ceil
	default:
		return half
	}
}

// regenerateCSV rebuilds the on-disk CSV from job_results so emails added
// by the async enricher (after the main scrape ended) are included in the
// download. If there are zero rows we leave the existing file untouched.
func (w *webrunner) regenerateCSV(ctx context.Context, jobID, outpath string) error {
	rows, err := w.svc.ListJobResults(ctx, jobID, 0)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	f, err := os.Create(outpath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	cw := csv.NewWriter(f)

	// Decode raw_json back into a *gmaps.Entry so we reuse the same CSV
	// layout (headers + columns) the live writer used during the scrape.
	headerWritten := false

	for i := range rows {
		entry, ok := decodeJobResultEntry(rows[i])
		if !ok {
			continue
		}

		if !headerWritten {
			if err := cw.Write(entry.CsvHeaders()); err != nil {
				return err
			}

			headerWritten = true
		}

		if err := cw.Write(entry.CsvRow()); err != nil {
			return err
		}
	}

	cw.Flush()

	return cw.Error()
}

func (w *webrunner) setupMate(_ context.Context, writer io.Writer, job *web.Job) (*scrapemateapp.ScrapemateApp, error) {
	opts := []func(*scrapemateapp.Config) error{
		scrapemateapp.WithConcurrency(w.cfg.Concurrency),
		// Do not use scrapemate's exit-on-inactivity here: scrapemate treats an
		// unset "last activity" timestamp as infinitely old, so the first stats
		// tick (~1m) can fire inactivity while the first Playwright/Maps job is
		// still running (common with high depth or large grids), yielding zero
		// CSV rows while the job still looks successful. Web jobs already have a
		// hard deadline in scrapeJob via context.WithTimeout(MaxTime).
		scrapemateapp.WithExitOnInactivity(0),
	}

	if !job.Data.FastMode {
		opts = append(opts,
			scrapemateapp.WithJS(scrapemateapp.DisableImages()),
		)
	} else {
		opts = append(opts,
			scrapemateapp.WithStealth("firefox"),
		)
	}

	hasProxy := false

	if len(w.cfg.Proxies) > 0 {
		opts = append(opts, scrapemateapp.WithProxies(w.cfg.Proxies))
		hasProxy = true
	} else if len(job.Data.Proxies) > 0 {
		opts = append(opts,
			scrapemateapp.WithProxies(job.Data.Proxies),
		)
		hasProxy = true
	}

	if !w.cfg.DisablePageReuse {
		opts = append(opts,
			scrapemateapp.WithPageReuseLimit(2),
			scrapemateapp.WithPageReuseLimit(200),
		)
	}

	log.Printf("job %s has proxy: %v", job.ID, hasProxy)

	live := web.NewCSVLiveWriter(csv.NewWriter(writer), w.svc, w.enricher, job.ID, job.Data.Email)

	writers := []scrapemate.ResultWriter{live}

	matecfg, err := scrapemateapp.NewConfig(
		writers,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return scrapemateapp.NewScrapeMateApp(matecfg)
}
