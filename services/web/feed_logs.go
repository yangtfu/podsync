package web

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const defaultFeedLogLimit = 500

var (
	DefaultFeedLogStore = NewFeedLogStore(defaultFeedLogLimit)
	feedLogHookOnce     sync.Once
)

type FeedLogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type FeedLogStore struct {
	mu      sync.RWMutex
	limit   int
	entries map[string][]FeedLogEntry
}

func NewFeedLogStore(limit int) *FeedLogStore {
	if limit <= 0 {
		limit = defaultFeedLogLimit
	}

	return &FeedLogStore{
		limit:   limit,
		entries: make(map[string][]FeedLogEntry),
	}
}

func (s *FeedLogStore) Add(feedID string, entry FeedLogEntry) {
	if feedID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries := append(s.entries[feedID], entry)
	if len(entries) > s.limit {
		entries = entries[len(entries)-s.limit:]
	}
	s.entries[feedID] = entries
}

func (s *FeedLogStore) List(feedID string, limit int) []FeedLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.entries[feedID]
	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}
	if limit == 0 {
		return nil
	}

	out := make([]FeedLogEntry, limit)
	copy(out, entries[len(entries)-limit:])
	return out
}

type feedLogHook struct {
	store *FeedLogStore
}

func InstallFeedLogHook(store *FeedLogStore) {
	feedLogHookOnce.Do(func() {
		log.AddHook(feedLogHook{store: store})
	})
}

func (h feedLogHook) Levels() []log.Level {
	return log.AllLevels
}

func (h feedLogHook) Fire(entry *log.Entry) error {
	feedID, ok := entry.Data["feed_id"]
	if !ok {
		return nil
	}

	feedIDString := fmt.Sprint(feedID)
	if feedIDString == "" {
		return nil
	}

	fields := make(map[string]string, len(entry.Data))
	for key, value := range entry.Data {
		if key == "feed_id" {
			continue
		}
		fields[key] = fmt.Sprint(value)
	}

	if len(fields) == 0 {
		fields = nil
	}

	h.store.Add(feedIDString, FeedLogEntry{
		Timestamp: entry.Time,
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Fields:    fields,
	})
	return nil
}
