package testkit

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

type LoadConfig struct {
	Concurrency int
	Duration    time.Duration
	Method      string
	Path        string
	Payload     any
}

type LoadResult struct {
	TotalRequests int64
	Errors        int64
	ErrorRate     float64
	RPS           float64
	P50           time.Duration
	P95           time.Duration
	P99           time.Duration
}

type workerResult struct {
	durations []time.Duration
	errs      int64
	total     int64
}

// LoadTest executes in-process stress tests, properly propagating cancellation to in-flight requests.
func (s *Suite) LoadTest(t *testing.T, cfg LoadConfig) LoadResult {
	t.Helper()

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 8
	}

	resultsChan := make(chan workerResult, cfg.Concurrency)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var wg sync.WaitGroup
	start := time.Now()

	for range cfg.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localDurations := make([]time.Duration, 0, 1024)
			var localErrs int64
			var localTotal int64

			for {
				if ctx.Err() != nil {
					resultsChan <- workerResult{
						durations: localDurations,
						errs:      localErrs,
						total:     localTotal,
					}
					return
				}

				req := s.Request(cfg.Method, cfg.Path)
				if cfg.Payload != nil {
					req.WithJSON(cfg.Payload)
				}

				t0 := time.Now()
				res, err := req.DoContext(ctx)
				dur := time.Since(t0)

				if err != nil {
					// Clean shutdown: the load window ended while this request was in-flight.
					if ctx.Err() != nil {
						resultsChan <- workerResult{
							durations: localDurations,
							errs:      localErrs,
							total:     localTotal,
						}
						return
					}

					localTotal++
					localErrs++
					continue
				}

				localTotal++
				localDurations = append(localDurations, dur)
				if res.Status() >= 500 {
					localErrs++
				}
			}
		}()
	}

	wg.Wait()
	actualDuration := time.Since(start)
	close(resultsChan)

	var totalReqs, totalErrs int64
	var allDurations []time.Duration

	for wr := range resultsChan {
		totalErrs += wr.errs
		totalReqs += wr.total
		allDurations = append(allDurations, wr.durations...)
	}

	res := LoadResult{
		TotalRequests: totalReqs,
		Errors:        totalErrs,
	}

	if totalReqs > 0 {
		res.ErrorRate = float64(totalErrs) / float64(totalReqs)
		res.RPS = float64(totalReqs) / actualDuration.Seconds()
	}

	if len(allDurations) > 0 {
		slices.Sort(allDurations)
		res.P50 = allDurations[int(float64(len(allDurations))*0.50)]
		res.P95 = allDurations[int(float64(len(allDurations))*0.95)]
		res.P99 = allDurations[int(float64(len(allDurations))*0.99)]
	}

	return res
}
