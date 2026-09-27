package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

const (
	defaultQueueSize     = 10000
	defaultBatchSize     = 100
	defaultFlushInterval = time.Second
	defaultMaxAttempts   = 5
	defaultRetryBackoff  = 200 * time.Millisecond
	shutdownFlushTimeout = 5 * time.Second
)

// Shipper asynchronously delivers audit records to the argocd-audit-controller.
//
// Enqueue never blocks the caller: records are buffered in memory and sent in
// batches. When the audit controller cannot be reached after several attempts,
// or the in-memory queue is full, records are written to this process' log
// instead so that they are never silently lost.
type Shipper struct {
	endpoint      string
	token         string
	client        *http.Client
	queue         chan Record
	batchSize     int
	flushInterval time.Duration
	maxAttempts   int
	retryBackoff  time.Duration
	dropped       atomic.Int64
	delivered     atomic.Int64
	log           *log.Entry
}

// ShipperOption customizes a Shipper.
type ShipperOption func(*Shipper)

// WithQueueSize sets the number of records buffered in memory.
func WithQueueSize(n int) ShipperOption {
	return func(s *Shipper) { s.queue = make(chan Record, n) }
}

// WithBatchSize sets the maximum number of records sent in one request.
func WithBatchSize(n int) ShipperOption {
	return func(s *Shipper) { s.batchSize = n }
}

// WithFlushInterval sets how often a partial batch is sent.
func WithFlushInterval(d time.Duration) ShipperOption {
	return func(s *Shipper) { s.flushInterval = d }
}

// WithRetry sets the number of delivery attempts and the initial backoff between them.
func WithRetry(attempts int, backoff time.Duration) ShipperOption {
	return func(s *Shipper) {
		s.maxAttempts = attempts
		s.retryBackoff = backoff
	}
}

// WithHTTPClient sets the HTTP client used to reach the audit controller.
func WithHTTPClient(c *http.Client) ShipperOption {
	return func(s *Shipper) { s.client = c }
}

// NewShipper returns a Shipper that sends records to the audit controller at
// address ("host:port", or a full http(s):// URL), authenticating with token.
func NewShipper(address, token string, opts ...ShipperOption) *Shipper {
	s := &Shipper{
		endpoint:      IngestURL(address),
		token:         token,
		client:        &http.Client{Timeout: 10 * time.Second},
		queue:         make(chan Record, defaultQueueSize),
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		maxAttempts:   defaultMaxAttempts,
		retryBackoff:  defaultRetryBackoff,
		log:           log.WithField("component", "audit-shipper"),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// IngestURL builds the ingest endpoint URL from an audit controller address.
func IngestURL(address string) string {
	address = strings.TrimSuffix(address, "/")
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return address + IngestPath
}

// Enqueue schedules a record for delivery. It never blocks.
func (s *Shipper) Enqueue(r Record) {
	select {
	case s.queue <- r:
	default:
		s.dropped.Add(1)
		s.logUndelivered(r, "audit queue is full")
	}
}

// Dropped returns the number of records that could not be delivered to the audit controller.
func (s *Shipper) Dropped() int64 { return s.dropped.Load() }

// Delivered returns the number of records successfully delivered to the audit controller.
func (s *Shipper) Delivered() int64 { return s.delivered.Load() }

// Run delivers queued records until ctx is cancelled, then makes a best effort
// to flush what is left.
func (s *Shipper) Run(ctx context.Context) {
	s.log.Infof("Shipping audit records to %s", s.endpoint)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	batch := make([]Record, 0, s.batchSize)
	for {
		select {
		case <-ctx.Done():
			s.drain(batch)
			return
		case r := <-s.queue:
			batch = append(batch, r)
			if len(batch) >= s.batchSize {
				s.send(ctx, batch)
				batch = make([]Record, 0, s.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.send(ctx, batch)
				batch = make([]Record, 0, s.batchSize)
			}
		}
	}
}

// drain sends the pending batch and everything still queued, using a fresh,
// bounded context because the Run context is already cancelled.
func (s *Shipper) drain(batch []Record) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownFlushTimeout)
	defer cancel()
	for {
		select {
		case r := <-s.queue:
			batch = append(batch, r)
			if len(batch) >= s.batchSize {
				s.send(ctx, batch)
				batch = make([]Record, 0, s.batchSize)
			}
		default:
			if len(batch) > 0 {
				s.send(ctx, batch)
			}
			return
		}
	}
}

func (s *Shipper) send(ctx context.Context, batch []Record) {
	body, err := json.Marshal(batch)
	if err != nil {
		for _, r := range batch {
			s.dropped.Add(1)
			s.logUndelivered(r, fmt.Sprintf("failed to marshal audit records: %v", err))
		}
		return
	}

	backoff := s.retryBackoff
	var lastErr error
	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		if lastErr = s.post(ctx, body); lastErr == nil {
			s.delivered.Add(int64(len(batch)))
			return
		}
		if attempt == s.maxAttempts || ctx.Err() != nil {
			break
		}
		s.log.Debugf("Audit delivery attempt %d/%d failed: %v", attempt, s.maxAttempts, lastErr)
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	for _, r := range batch {
		s.dropped.Add(1)
		s.logUndelivered(r, fmt.Sprintf("failed to deliver audit record to audit controller: %v", lastErr))
	}
}

func (s *Shipper) post(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer utilio.Close(resp.Body)
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("audit controller responded with %s", resp.Status)
	}
	return nil
}

// logUndelivered writes the record to this process' log, so it is kept in the
// log pipeline even when the audit controller is unavailable.
func (s *Shipper) logUndelivered(r Record, reason string) {
	data, err := json.Marshal(r)
	if err != nil {
		s.log.Warnf("%s: %s %s by %s", reason, r.Action, r.Resource.Name, r.Actor.Username)
		return
	}
	s.log.WithField("auditRecord", string(data)).Warn(reason)
}
