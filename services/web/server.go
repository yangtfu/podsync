package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/yangtfu/podsync/pkg/db"
	"github.com/yangtfu/podsync/pkg/feed"
	"github.com/yangtfu/podsync/pkg/model"
)

type Server struct {
	http.Server
	db      db.Storage
	cfg     Config
	options Options
}

type Options struct {
	ConfigPath     string
	Feeds          map[string]*feed.Config
	SetFeedEnabled func(ctx context.Context, feedID string, enabled bool) error
}

type Config struct {
	// Hostname to use for download links
	Hostname string `toml:"hostname"`
	// Port is a server port to listen to
	Port int `toml:"port"`
	// RunOnStart triggers a global feed update when Podsync starts
	RunOnStart bool `toml:"run_on_start"`
	// Bind a specific IP addresses for server
	// "*": bind all IP addresses which is default option
	// localhost or 127.0.0.1  bind a single IPv4 address
	BindAddress string `toml:"bind_address"`
	// Flag indicating if the server will use TLS
	TLS bool `toml:"tls"`
	// Path to a certificate file for TLS connections
	CertificatePath string `toml:"certificate_path"`
	// Path to a private key file for TLS connections
	KeyFilePath string `toml:"key_file_path"`
	// Specify path for reverse proxy and only [A-Za-z0-9]
	Path string `toml:"path"`
	// DataDir is a path to a directory to keep XML feeds and downloaded episodes,
	// that will be available to user via web server for download.
	DataDir string `toml:"data_dir"`
	// WebUIEnabled is a flag indicating if web UI is enabled
	WebUIEnabled bool `toml:"web_ui"`
}

func New(cfg Config, storage http.FileSystem, database db.Storage, options Options) *Server {
	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	bindAddress := cfg.BindAddress
	if bindAddress == "*" {
		bindAddress = ""
	}

	srv := Server{
		db:      database,
		cfg:     cfg,
		options: options,
	}

	srv.Addr = fmt.Sprintf("%s:%d", bindAddress, port)
	log.Debugf("using address: %s:%s", bindAddress, srv.Addr)

	mux := http.NewServeMux()
	fileServer := http.FileServer(storage)

	log.Debugf("handle path: /%s", cfg.Path)
	mux.Handle(fmt.Sprintf("/%s", cfg.Path), fileServer)

	// Add health check endpoint
	mux.HandleFunc("/health", srv.healthCheckHandler)

	if cfg.WebUIEnabled {
		InstallFeedLogHook(DefaultFeedLogStore)
		mux.HandleFunc("/api/feeds", srv.feedsHandler)
		mux.HandleFunc("/api/feeds/", srv.feedHandler)
	}

	srv.Handler = mux

	return &srv
}

type HealthStatus struct {
	Status         string    `json:"status"`
	Timestamp      time.Time `json:"timestamp"`
	FailedEpisodes int       `json:"failed_episodes,omitempty"`
	Message        string    `json:"message,omitempty"`
}

func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for recent download failures within the last 24 hours
	failedCount := 0
	cutoffTime := time.Now().Add(-24 * time.Hour)

	// Walk through all feeds to count recent failures
	err := s.db.WalkFeeds(ctx, func(feed *model.Feed) error {
		return s.db.WalkEpisodes(ctx, feed.ID, func(episode *model.Episode) error {
			if episode.Status == model.EpisodeError && episode.PubDate.After(cutoffTime) {
				failedCount++
			}
			return nil
		})
	})

	w.Header().Set("Content-Type", "application/json")

	status := HealthStatus{
		Timestamp: time.Now(),
	}

	if err != nil {
		log.WithError(err).Error("health check database error")
		status.Status = "unhealthy"
		status.Message = "database error during health check"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else if failedCount > 0 {
		status.Status = "unhealthy"
		status.FailedEpisodes = failedCount
		status.Message = fmt.Sprintf("found %d failed downloads in the last 24 hours", failedCount)
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		status.Status = "healthy"
		status.Message = "no recent download failures detected"
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(status)
}

func (s *Server) feedsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/feeds" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type feedSummary struct {
		ID          string `json:"id"`
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
		CoverArt    string `json:"cover_art"`
		Enabled     bool   `json:"enabled"`
		OPML        bool   `json:"opml"`
		Format      string `json:"format"`
		Quality     string `json:"quality"`
		XMLURL      string `json:"xml_url"`
	}

	feeds := make([]feedSummary, 0, len(s.options.Feeds))
	for _, cfg := range sortedFeedConfigs(s.options.Feeds) {
		savedFeed, err := s.db.GetFeed(r.Context(), cfg.ID)
		if err != nil && err != model.ErrNotFound {
			log.WithError(err).WithField("feed_id", cfg.ID).Warn("failed to load feed metadata")
		}

		title := cfg.Custom.Title
		description := cfg.Custom.Description
		coverArt := cfg.Custom.CoverArt
		if savedFeed != nil {
			if title == "" {
				title = savedFeed.Title
			}
			if description == "" {
				description = savedFeed.Description
			}
			if coverArt == "" {
				coverArt = savedFeed.CoverArt
			}
		}

		feeds = append(feeds, feedSummary{
			ID:          cfg.ID,
			URL:         cfg.URL,
			Title:       title,
			Description: description,
			CoverArt:    coverArt,
			Enabled:     cfg.IsEnabled(),
			OPML:        cfg.OPML,
			Format:      string(cfg.Format),
			Quality:     string(cfg.Quality),
			XMLURL:      strings.TrimRight(s.cfg.Hostname, "/") + "/" + cfg.ID + ".xml",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"feeds": feeds})
}

func (s *Server) feedHandler(w http.ResponseWriter, r *http.Request) {
	feedID, action, ok := parseFeedAPIPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if _, ok := s.options.Feeds[feedID]; !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "config":
		s.feedConfigHandler(w, r, feedID)
	case "enable":
		s.feedEnableHandler(w, r, feedID)
	case "disable":
		s.feedDisableHandler(w, r, feedID)
	case "logs":
		s.feedLogsHandler(w, r, feedID)
	default:
		http.NotFound(w, r)
	}
}

func parseFeedAPIPath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/feeds/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.WithError(err).Error("failed to write JSON response")
	}
}
