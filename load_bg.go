package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync/atomic"
)

// StartBackgroundLoad generates background load, correctly checking for URL/request errors.
func (s *Suite) StartBackgroundLoad(cfg LoadConfig) func() {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}

	ctx, cancel := context.WithCancel(context.Background())
	var reqs, errs atomic.Int64

	var payloadBytes []byte
	if cfg.Payload != nil {
		payloadBytes, _ = json.Marshal(cfg.Payload)
	}

	for range cfg.Concurrency {
		go func() {
			for {
				if ctx.Err() != nil {
					return
				}

				var body io.Reader
				if len(payloadBytes) > 0 {
					body = bytes.NewReader(payloadBytes)
				}

				req, err := http.NewRequestWithContext(ctx, cfg.Method, s.baseURL+cfg.Path, body)
				if err != nil {
					errs.Add(1)
					continue
				}

				if len(payloadBytes) > 0 {
					req.Header.Set("Content-Type", "application/json")
				}

				s.mu.RLock()
				for k, v := range s.headers {
					req.Header.Set(k, v)
				}
				for _, c := range s.cookies {
					req.AddCookie(c)
				}
				s.mu.RUnlock()

				resp, err := s.client.Do(req)
				if err == nil {
					if resp.StatusCode >= 500 {
						errs.Add(1)
					}
					_ = resp.Body.Close()
				} else {
					errs.Add(1)
				}
				reqs.Add(1)
			}
		}()
	}

	return func() {
		cancel()
		log.Printf("🛑 Background Load Stopped: [%s] %s | Total: %d | Errors: %d\n",
			cfg.Method, cfg.Path, reqs.Load(), errs.Load())
	}
}
