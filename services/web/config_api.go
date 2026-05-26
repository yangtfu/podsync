package web

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/yangtfu/podsync/pkg/feed"
)

func sortedFeedConfigs(feeds map[string]*feed.Config) []*feed.Config {
	ids := make([]string, 0, len(feeds))
	for id := range feeds {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]*feed.Config, 0, len(ids))
	for _, id := range ids {
		out = append(out, feeds[id])
	}
	return out
}

func (s *Server) feedDisableHandler(w http.ResponseWriter, r *http.Request, feedID string) {
	s.feedEnabledHandler(w, r, feedID, false)
}

func (s *Server) feedEnableHandler(w http.ResponseWriter, r *http.Request, feedID string) {
	s.feedEnabledHandler(w, r, feedID, true)
}

func (s *Server) feedConfigHandler(w http.ResponseWriter, r *http.Request, feedID string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	block, err := readFeedConfigBlock(s.options.ConfigPath, feedID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"feed_id": feedID,
		"config":  block,
	})
}

func (s *Server) feedEnabledHandler(w http.ResponseWriter, r *http.Request, feedID string, enabled bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.options.SetFeedEnabled == nil {
		writeAPIError(w, http.StatusInternalServerError, errors.New("feed state controller is not configured"))
		return
	}

	if err := s.options.SetFeedEnabled(r.Context(), feedID, enabled); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"feed_id":          feedID,
		"requires_restart": false,
	})
}

func (s *Server) feedLogsHandler(w http.ResponseWriter, r *http.Request, feedID string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeAPIError(w, http.StatusBadRequest, errors.New("limit must be a non-negative integer"))
			return
		}
		limit = parsed
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"feed_id": feedID,
		"logs":    DefaultFeedLogStore.List(feedID, limit),
	})
}

func readFeedConfigBlock(configPath, feedID string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read config")
	}

	start, end, err := findFeedConfigBlock(data, feedID)
	if err != nil {
		return "", err
	}
	return string(data[start:end]), nil
}

func findFeedConfigBlock(data []byte, feedID string) (int, int, error) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	offset := 0
	start := -1
	end := len(data)

	for _, line := range lines {
		currentOffset := offset
		offset += len(line)

		trimmed := strings.TrimSpace(string(line))
		// Check if this is any section header (starts with [)
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "#") {
			sectionFeedID, isFeedSection := feedSectionID(string(line))

			if start == -1 {
				if isFeedSection && sectionFeedID == feedID {
					start = currentOffset
				}
				continue
			}

			// If we already found the feed section, stop at any new section
			if isFeedSection {
				if sectionFeedID != feedID {
					end = currentOffset
					break
				}
			} else {
				// Hit a non-feed section (like [downloader] or [log])
				end = currentOffset
				break
			}
		}
	}

	if start == -1 {
		return 0, 0, fmt.Errorf("feed %q was not found in config", feedID)
	}
	return start, end, nil
}

func feedSectionID(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "[") {
		return "", false
	}

	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimSpace(trimmed)

	if !strings.HasPrefix(trimmed, "feeds.") {
		return "", false
	}

	rest := strings.TrimPrefix(trimmed, "feeds.")
	if rest == "" {
		return "", false
	}
	if strings.HasPrefix(rest, "\"") {
		return "", false
	}

	parts := strings.SplitN(rest, ".", 2)
	return parts[0], parts[0] != ""
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
