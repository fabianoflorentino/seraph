package collector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/state"
)

const (
	defaultConcurrency = 8
	defaultRetries     = 2
	defaultTimeout     = 8 * time.Second
	defaultUserAgent   = "seraph-statusboard"
)

// Collector observes services concurrently and normalizes their status.
type Collector struct {
	client      *http.Client
	concurrency int
	retries     int
	timeout     time.Duration
	now         func() time.Time
	backoff     func(attempt int) time.Duration
	userAgent   string
}

// Option customizes a Collector.
type Option func(*Collector)

// WithRetries sets how many times a failed request is retried.
func WithRetries(retries int) Option {
	return func(c *Collector) {
		if retries >= 0 {
			c.retries = retries
		}
	}
}

// WithConcurrency caps how many services are observed at once.
func WithConcurrency(concurrency int) Option {
	return func(c *Collector) {
		if concurrency > 0 {
			c.concurrency = concurrency
		}
	}
}

// WithTimeout sets the per-attempt request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Collector) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithClock overrides the clock used for observation timestamps.
func WithClock(now func() time.Time) Option {
	return func(c *Collector) {
		if now != nil {
			c.now = now
		}
	}
}

// WithBackoff overrides the delay before a retry.
func WithBackoff(backoff func(attempt int) time.Duration) Option {
	return func(c *Collector) {
		if backoff != nil {
			c.backoff = backoff
		}
	}
}

// WithUserAgent overrides the HTTP user agent.
func WithUserAgent(userAgent string) Option {
	return func(c *Collector) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

// New builds a Collector using client for every request.
func New(client *http.Client, opts ...Option) *Collector {
	if client == nil {
		client = http.DefaultClient
	}

	c := &Collector{
		client:      client,
		concurrency: defaultConcurrency,
		retries:     defaultRetries,
		timeout:     defaultTimeout,
		now:         time.Now,
		backoff:     func(attempt int) time.Duration { return time.Duration(attempt) * 100 * time.Millisecond },
		userAgent:   defaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Collect observes every service and returns results in catalog order.
func (c *Collector) Collect(ctx context.Context, services []catalog.Service) []state.Service {
	results := make([]state.Service, len(services))
	semaphore := make(chan struct{}, c.concurrency)

	var wg sync.WaitGroup

	for index, service := range services {
		wg.Add(1)

		go func(index int, service catalog.Service) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = c.failure(service, ctx.Err())

				return
			}

			results[index] = c.collect(ctx, service)
		}(index, service)
	}

	wg.Wait()

	return results
}

func (c *Collector) collect(ctx context.Context, service catalog.Service) state.Service {
	observedAt := c.now().UTC()

	status, latency, err := c.observe(ctx, service.Source)
	if err != nil {
		return failureAt(service, observedAt, err)
	}

	return state.Service{
		Slug:        service.Slug,
		Name:        service.Name,
		Category:    string(service.Category),
		OfficialURL: service.OfficialURL,
		StatusURL:   service.StatusURL,
		Regions:     service.RegionList(),
		Status:      status,
		ObservedAt:  &observedAt,
		LatencyMS:   latency.Milliseconds(),
		Points:      []state.Point{{At: observedAt, Status: status}},
	}
}

func (c *Collector) failure(service catalog.Service, err error) state.Service {
	return failureAt(service, c.now().UTC(), err)
}

func failureAt(service catalog.Service, observedAt time.Time, err error) state.Service {
	return state.Service{
		Slug:        service.Slug,
		Name:        service.Name,
		Category:    string(service.Category),
		OfficialURL: service.OfficialURL,
		StatusURL:   service.StatusURL,
		Regions:     service.RegionList(),
		Status:      state.StatusUnknown,
		ObservedAt:  &observedAt,
		Error:       err.Error(),
	}
}

func (c *Collector) observe(ctx context.Context, source catalog.Source) (state.Status, time.Duration, error) {
	switch source.Type {
	case catalog.SourceStatuspage:
		return c.retry(ctx, func(ctx context.Context) (state.Status, time.Duration, error) {
			return c.fetchStatuspage(ctx, source)
		})
	case catalog.SourceStatusIO:
		return c.retry(ctx, func(ctx context.Context) (state.Status, time.Duration, error) {
			return c.fetchStatusIO(ctx, source)
		})
	case catalog.SourceRSS:
		return c.retry(ctx, func(ctx context.Context) (state.Status, time.Duration, error) {
			return c.fetchRSS(ctx, source)
		})
	case catalog.SourceJSON:
		return c.retry(ctx, func(ctx context.Context) (state.Status, time.Duration, error) {
			return c.fetchJSON(ctx, source)
		})
	default:
		return state.StatusUnknown, 0, fmt.Errorf("source type %q is not implemented", source.Type)
	}
}

type observation func(context.Context) (state.Status, time.Duration, error)

func (c *Collector) retry(ctx context.Context, attempt observation) (state.Status, time.Duration, error) {
	var lastErr error

	for try := 0; try <= c.retries; try++ {
		if try > 0 {
			if err := wait(ctx, c.backoff(try)); err != nil {
				return state.StatusUnknown, 0, err
			}
		}

		status, latency, err := attempt(ctx)
		if err == nil {
			return status, latency, nil
		}

		lastErr = err

		if ctx.Err() != nil || !retryable(err) {
			break
		}
	}

	return state.StatusUnknown, 0, lastErr
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr.code == http.StatusTooManyRequests || httpErr.code >= 500
	}

	var netErr net.Error

	return errors.As(err, &netErr)
}
