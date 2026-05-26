package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"

	"github.com/yangtfu/podsync/pkg/db"
	"github.com/yangtfu/podsync/pkg/feed"
	"github.com/yangtfu/podsync/pkg/model"
)

type feedRuntime struct {
	mu          sync.RWMutex
	feeds       map[string]*feed.Config
	db          db.Storage
	cron        *cron.Cron
	entries     map[string]cron.EntryID
	updates     chan<- *feed.Config
	rebuildOPML func(context.Context) error
}

func applyFeedEnabledOverrides(ctx context.Context, database db.Storage, feeds map[string]*feed.Config) error {
	for _, feedConfig := range feeds {
		enabled, err := database.GetFeedEnabled(ctx, feedConfig.ID)
		if err == model.ErrNotFound {
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to load feed enabled state for %q", feedConfig.ID)
		}
		setFeedEnabledValue(feedConfig, enabled)
	}
	return nil
}

func newFeedRuntime(
	database db.Storage,
	feeds map[string]*feed.Config,
	cronScheduler *cron.Cron,
	updates chan<- *feed.Config,
	rebuildOPML func(context.Context) error,
) *feedRuntime {
	return &feedRuntime{
		feeds:       feeds,
		db:          database,
		cron:        cronScheduler,
		entries:     make(map[string]cron.EntryID),
		updates:     updates,
		rebuildOPML: rebuildOPML,
	}
}

func (r *feedRuntime) RegisterEnabledFeeds(runOnStart bool) error {
	r.mu.Lock()

	initialUpdates := make([]*feed.Config, 0, len(r.feeds))
	for _, feedConfig := range sortedFeeds(r.feeds) {
		if !feedConfig.IsEnabled() {
			log.WithField("feed_id", feedConfig.ID).Info("skipping disabled feed")
			continue
		}

		if err := r.scheduleLocked(feedConfig); err != nil {
			r.mu.Unlock()
			return err
		}

		if runOnStart {
			initialUpdates = append(initialUpdates, feedConfig)
		}
	}
	r.mu.Unlock()

	for _, feedConfig := range initialUpdates {
		r.updates <- feedConfig
	}
	return nil
}

func (r *feedRuntime) SetEnabled(ctx context.Context, feedID string, enabled bool) error {
	r.mu.Lock()
	feedConfig, ok := r.feeds[feedID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("feed %q was not found", feedID)
	}

	if err := r.db.SetFeedEnabled(ctx, feedID, enabled); err != nil {
		r.mu.Unlock()
		return errors.Wrapf(err, "failed to persist feed enabled state for %q", feedID)
	}

	setFeedEnabledValue(feedConfig, enabled)

	var err error
	if enabled {
		err = r.scheduleLocked(feedConfig)
	} else {
		r.unscheduleLocked(feedID)
	}
	r.mu.Unlock()
	if err != nil {
		return err
	}

	if r.rebuildOPML != nil {
		if err := r.rebuildOPML(ctx); err != nil {
			log.WithError(err).WithField("feed_id", feedID).Warn("failed to rebuild OPML after feed state change")
		}
	}
	return nil
}

func (r *feedRuntime) IsEnabled(feedID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	feedConfig, ok := r.feeds[feedID]
	return ok && feedConfig.IsEnabled()
}

func (r *feedRuntime) NextUpdate(feedID string) time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entryID, ok := r.entries[feedID]
	if !ok {
		return time.Time{}
	}
	return r.cron.Entry(entryID).Next
}

func (r *feedRuntime) scheduleLocked(feedConfig *feed.Config) error {
	if _, ok := r.entries[feedConfig.ID]; ok {
		return nil
	}

	if feedConfig.CronSchedule == "" {
		feedConfig.CronSchedule = fmt.Sprintf("@every %s", feedConfig.UpdatePeriod.String())
	}

	cronFeed := feedConfig
	cronID, err := r.cron.AddFunc(cronFeed.CronSchedule, func() {
		if !r.IsEnabled(cronFeed.ID) {
			return
		}
		log.WithField("feed_id", cronFeed.ID).Debugf("adding %q to update queue", cronFeed.ID)
		r.updates <- cronFeed
	})
	if err != nil {
		return errors.Wrapf(err, "can't create cron task for feed: %s", cronFeed.ID)
	}

	r.entries[cronFeed.ID] = cronID
	log.WithField("feed_id", cronFeed.ID).Debugf("-> %s (update '%s')", cronFeed.ID, cronFeed.CronSchedule)
	return nil
}

func (r *feedRuntime) unscheduleLocked(feedID string) {
	entryID, ok := r.entries[feedID]
	if !ok {
		return
	}

	r.cron.Remove(entryID)
	delete(r.entries, feedID)
	log.WithField("feed_id", feedID).Info("removed feed from update schedule")
}

func setFeedEnabledValue(feedConfig *feed.Config, enabled bool) {
	feedConfig.Enable = &enabled
}
