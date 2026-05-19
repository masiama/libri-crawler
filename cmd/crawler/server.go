package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"libri-crawler/internal/scraper"
)

var (
	ErrAllSourcesRunning    = errors.New("all sources are already running")
	ErrSourceAlreadyRunning = errors.New("source is already running")
	ErrInvalidSource        = errors.New("invalid source")
)

type crawlManager struct {
	mu     sync.Mutex
	active map[scraper.SourceName]struct{}
	runner *Runner
}

type crawlResponse struct {
	StartedSources []string `json:"startedSources"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewCrawlManager(runner *Runner) *crawlManager {
	return &crawlManager{
		active: make(map[scraper.SourceName]struct{}),
		runner: runner,
	}
}

func newServer(manager *crawlManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawl", manager.handleCrawl)
	return mux
}

func (m *crawlManager) handleCrawl(w http.ResponseWriter, r *http.Request) {
	started, err := m.start(r.URL.Query().Get("source"), r.URL.Query().Get("crawlId"))
	if err != nil {
		switch {
		case errors.Is(err, ErrAllSourcesRunning), errors.Is(err, ErrSourceAlreadyRunning):
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		case errors.Is(err, ErrInvalidSource):
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "invalid source, expected one of: " + scraper.GetSourcesString(),
			})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start crawl"})
		}
		return
	}

	response := crawlResponse{StartedSources: make([]string, 0, len(started))}
	for _, source := range started {
		response.StartedSources = append(response.StartedSources, string(source))
	}

	writeJSON(w, http.StatusAccepted, response)
}

func (m *crawlManager) start(rawSource, crawlId string) ([]scraper.SourceName, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	requestedSources, err := selectSources(strings.TrimSpace(rawSource), m.active)
	if err != nil {
		return nil, err
	}

	for _, source := range requestedSources {
		m.active[source] = struct{}{}
		go m.run(source, crawlId)
	}

	return requestedSources, nil
}

func (m *crawlManager) run(source scraper.SourceName, crawlId string) {
	defer func() {
		m.mu.Lock()
		delete(m.active, source)
		m.mu.Unlock()
	}()

	if err := m.runner.Run(context.Background(), source, crawlId); err != nil {
		slog.Error(string(LogEventCrawlFailed), "source", source, "crawlId", crawlId, "error", err)
	}
}

func selectSources(
	requested string,
	active map[scraper.SourceName]struct{},
) ([]scraper.SourceName, error) {
	if requested == "" || requested == "all" {
		available := make([]scraper.SourceName, 0, len(scraper.AllSources))
		for _, source := range scraper.AllSources {
			if _, ok := active[source]; ok {
				continue
			}
			available = append(available, source)
		}
		if len(available) == 0 {
			return nil, ErrAllSourcesRunning
		}
		return available, nil
	}

	source := scraper.SourceName(requested)
	if !scraper.IsValidSource(source) {
		return nil, ErrInvalidSource
	}
	if _, ok := active[source]; ok {
		return nil, ErrSourceAlreadyRunning
	}

	return []scraper.SourceName{source}, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error(string(LogEventHTTPResponseWriteFailed), "error", err)
	}
}
