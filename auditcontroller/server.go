package auditcontroller

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/audit"
)

const (
	// maxIngestBodyBytes bounds a single ingest request.
	maxIngestBodyBytes = 8 << 20
	// maxIngestRecords bounds the number of records in a single ingest request.
	maxIngestRecords  = 1000
	defaultQueryLimit = 100
	maxQueryLimit     = 1000
)

// Server is the HTTP API of the audit controller.
type Server struct {
	recorder *Recorder
	token    string
	metrics  *Metrics
}

// NewServer returns the HTTP API. When token is empty, requests are not
// authenticated; this is only meant for local development.
func NewServer(recorder *Recorder, token string, metrics *Metrics) *Server {
	return &Server{recorder: recorder, token: token, metrics: metrics}
}

// Handler returns the HTTP handler serving the audit API and health checks.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(audit.IngestPath, s.handleRecords)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleRecords(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		s.metrics.ingestRejectedInc("unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.ingest(w, r)
	case http.MethodGet:
		s.query(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	got, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, maxIngestBodyBytes)
	var records []audit.Record
	if err := json.NewDecoder(body).Decode(&records); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.metrics.ingestRejectedInc("too_large")
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		s.metrics.ingestRejectedInc("malformed")
		http.Error(w, "malformed audit records: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(records) > maxIngestRecords {
		s.metrics.ingestRejectedInc("too_many_records")
		http.Error(w, "too many records in one request", http.StatusRequestEntityTooLarge)
		return
	}
	var failed int
	for _, rec := range records {
		if rec.Source == "" {
			rec.Source = audit.SourceAPIServer
		}
		if err := s.recorder.Record(rec); err != nil {
			failed++
			log.WithError(err).Error("Failed to write audit record")
		}
	}
	if failed > 0 {
		http.Error(w, strconv.Itoa(failed)+" audit records could not be written", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := Filter{
		User:         q.Get("user"),
		Action:       q.Get("action"),
		Verb:         q.Get("verb"),
		ResourceType: q.Get("resourceType"),
		ResourceName: q.Get("resourceName"),
		Project:      q.Get("project"),
		Result:       q.Get("result"),
	}
	if since := q.Get("since"); since != "" {
		t, err := parseSince(since, time.Now())
		if err != nil {
			http.Error(w, "invalid since: "+err.Error(), http.StatusBadRequest)
			return
		}
		filter.Since = t
	}
	limit := defaultQueryLimit
	if l := q.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = min(n, maxQueryLimit)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.recorder.Recent(filter, limit)); err != nil {
		log.WithError(err).Warn("Failed to write audit query response")
	}
}

// parseSince accepts an RFC 3339 timestamp or a duration relative to now ("15m", "2h").
func parseSince(v string, now time.Time) (time.Time, error) {
	if d, err := time.ParseDuration(v); err == nil {
		return now.Add(-d), nil
	}
	return time.Parse(time.RFC3339, v)
}
