package main

import (
	"context"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"github.com/yangtfu/podsync/pkg/feed"
	"github.com/yangtfu/podsync/pkg/model"
	"github.com/yangtfu/podsync/services/update"
	"github.com/yangtfu/podsync/services/web"
	"golang.org/x/sync/errgroup"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/yangtfu/podsync/pkg/db"
	"github.com/yangtfu/podsync/pkg/fs"
	"github.com/yangtfu/podsync/pkg/ytdl"
)

type Opts struct {
	ConfigPath string `long:"config" short:"c" default:"config.toml" env:"PODSYNC_CONFIG_PATH"`
	Headless   bool   `long:"headless"`
	Debug      bool   `long:"debug"`
	NoBanner   bool   `long:"no-banner"`
}

const banner = `
 _______  _______  ______   _______           _        _______ 
(  ____ )(  ___  )(  __  \ (  ____ \|\     /|( (    /|(  ____ \
| (    )|| (   ) || (  \  )| (    \/( \   / )|  \  ( || (    \/
| (____)|| |   | || |   ) || (_____  \ (_) / |   \ | || |      
|  _____)| |   | || |   | |(_____  )  \   /  | (\ \) || |      
| (      | |   | || |   ) |      ) |   ) (   | | \   || |      
| )      | (___) || (__/  )/\____) |   | |   | )  \  || (____/\
|/       (_______)(______/ \_______)   \_/   |/    )_)(_______/
`

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	arch    = ""
)

type updateSpacer struct {
	rng *rand.Rand
}

func newUpdateSpacer() *updateSpacer {
	return &updateSpacer{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func sortedFeeds(feeds map[string]*feed.Config) []*feed.Config {
	ids := make([]string, 0, len(feeds))
	for id := range feeds {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	list := make([]*feed.Config, 0, len(feeds))
	for _, id := range ids {
		list = append(list, feeds[id])
	}

	return list
}

func (s *updateSpacer) Wait(ctx context.Context, feedConfig *feed.Config) error {
	if feedConfig.UpdateDelay <= 0 {
		return nil
	}
	delay := time.Duration(s.rng.Int63n(int64(feedConfig.UpdateDelay)))

	log.WithFields(log.Fields{
		"feed_id": feedConfig.ID,
		"delay":   delay,
	}).Info("waiting before next feed update")

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: time.RFC3339,
		FullTimestamp:   true,
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse args
	opts := Opts{}
	_, err := flags.Parse(&opts)
	if err != nil {
		log.WithError(err).Fatal("failed to parse command line arguments")
	}

	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}

	if !opts.NoBanner {
		log.Info(banner)
	}

	log.WithFields(log.Fields{
		"version": version,
		"commit":  commit,
		"date":    date,
		"arch":    arch,
	}).Info("running podsync")

	// Load TOML file
	log.Debugf("loading configuration %q", opts.ConfigPath)
	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		log.WithError(err).Fatal("failed to load configuration file")
	}

	if cfg.Log.Filename != "" {
		log.Infof("Using log file: %s", cfg.Log.Filename)

		log.SetOutput(&lumberjack.Logger{
			Filename:   cfg.Log.Filename,
			MaxSize:    cfg.Log.MaxSize,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAge:     cfg.Log.MaxAge,
			Compress:   cfg.Log.Compress,
		})

		// Optionally enable debug mode from config.toml
		if cfg.Log.Debug {
			log.SetLevel(log.DebugLevel)
		}
	}

	downloader, err := ytdl.New(ctx, cfg.Downloader)
	if err != nil {
		log.WithError(err).Fatal("youtube-dl error")
	}

	database, err := db.NewBadger(&cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("failed to open database")
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.WithError(err).Error("failed to close database")
		}
	}()

	var storage fs.Storage
	switch cfg.Storage.Type {
	case "local":
		storage, err = fs.NewLocal(cfg.Storage.Local.DataDir, cfg.Server.WebUIEnabled)
	case "s3":
		storage, err = fs.NewS3(cfg.Storage.S3) // serving files from S3 is not supported, so no WebUI either
	default:
		log.Fatalf("unknown storage type: %s", cfg.Storage.Type)
	}
	if err != nil {
		log.WithError(err).Fatal("failed to open storage")
	}

	// Run updater thread
	log.Debug("creating key providers")
	keys := map[model.Provider]feed.KeyProvider{}
	for name, list := range cfg.Tokens {
		provider, err := feed.NewKeyProvider(list)
		if err != nil {
			log.WithError(err).Fatalf("failed to create key provider for %q", name)
		}
		keys[name] = provider
	}

	log.Debug("creating update manager")
	if err := applyFeedEnabledOverrides(ctx, database, cfg.Feeds); err != nil {
		log.WithError(err).Fatal("failed to load feed runtime state")
	}

	manager, err := update.NewUpdater(cfg.Feeds, keys, cfg.Server.Hostname, downloader, database, storage)
	if err != nil {
		log.WithError(err).Fatal("failed to create updater")
	}

	// In Headless mode, do one round of feed updates and quit
	if opts.Headless {
		spacer := newUpdateSpacer()
		for _, _feed := range sortedFeeds(cfg.Feeds) {
			if !_feed.IsEnabled() {
				log.WithField("feed_id", _feed.ID).Info("skipping disabled feed")
				continue
			}

			if err := spacer.Wait(ctx, _feed); err != nil {
				log.WithError(err).Errorf("failed to wait before updating feed: %s", _feed.URL)
				continue
			}
			if err := manager.Update(ctx, _feed); err != nil {
				log.WithError(err).WithField("feed_id", _feed.ID).Errorf("failed to update feed: %s", _feed.URL)
			}
		}
		return
	}

	// Queue of feeds to update
	updates := make(chan *feed.Config, 16)
	defer close(updates)

	group, ctx := errgroup.WithContext(ctx)
	defer func() {
		if err := group.Wait(); err != nil && (err != context.Canceled && err != http.ErrServerClosed) {
			log.WithError(err).Error("wait error")
		}
		log.Info("gracefully stopped")
	}()

	// Create Cron
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)))
	feedRuntime := newFeedRuntime(database, cfg.Feeds, c, updates, manager.RebuildOPML)

	// Run updates listener
	group.Go(func() error {
		spacer := newUpdateSpacer()

		for {
			select {
			case _feed, ok := <-updates:
				if !ok {
					return nil
				}
				if !feedRuntime.IsEnabled(_feed.ID) {
					log.WithField("feed_id", _feed.ID).Info("skipping disabled feed")
					continue
				}
				if err := spacer.Wait(ctx, _feed); err != nil {
					return err
				}
				if !feedRuntime.IsEnabled(_feed.ID) {
					log.WithField("feed_id", _feed.ID).Info("skipping disabled feed")
					continue
				}
				if err := manager.Update(ctx, _feed); err != nil {
					log.WithError(err).WithField("feed_id", _feed.ID).Errorf("failed to update feed: %s", _feed.URL)
				} else {
					if next := feedRuntime.NextUpdate(_feed.ID); !next.IsZero() {
						log.WithField("feed_id", _feed.ID).Infof("next update of %s: %s", _feed.ID, next)
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	// Run cron scheduler
	group.Go(func() error {
		if err := feedRuntime.RegisterEnabledFeeds(cfg.Server.RunOnStart); err != nil {
			log.WithError(err).Fatal("can't create cron task")
		}

		c.Start()

		for {
			<-ctx.Done()

			log.Info("shutting down cron")
			c.Stop()

			return ctx.Err()
		}
	})

	if cfg.Storage.Type == "s3" {
		return // S3 content is hosted externally
	}

	// Run web server
	srv := web.New(cfg.Server, storage, database, web.Options{
		ConfigPath:     opts.ConfigPath,
		Feeds:          cfg.Feeds,
		SetFeedEnabled: feedRuntime.SetEnabled,
	})

	group.Go(func() error {
		log.Infof("running listener at %s", srv.Addr)
		if cfg.Server.TLS {
			return srv.ListenAndServeTLS(cfg.Server.CertificatePath, cfg.Server.KeyFilePath)
		}
		return srv.ListenAndServe()
	})

	group.Go(func() error {
		// Shutdown web server
		defer func() {
			ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer func() {
				cancel()
			}()
			log.Info("shutting down web server")
			if err := srv.Shutdown(ctxShutDown); err != nil {
				log.WithError(err).Error("server shutdown failed")
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-stop:
				cancel()
				return nil
			}
		}
	})
}
