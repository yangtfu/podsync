package builder

import (
	"context"
	"fmt"
	"strings"

	"time"

	"github.com/yangtfu/podsync/pkg/feed"

	"github.com/yangtfu/podsync/pkg/model"
)

type BilibiliBuilder struct {
	client *APIClient
}

const (
	maxSeriesResults = 100
)

func (b *BilibiliBuilder) Build(_ context.Context, cfg *feed.Config) (*model.Feed, error) {
	info, err := ParseURL(cfg.URL)

	if err != nil {
		return nil, err
	}

	_feed := &model.Feed{
		ItemID:          info.ItemID,
		Provider:        info.Provider,
		LinkType:        info.LinkType,
		Format:          cfg.Format,
		Quality:         cfg.Quality,
		CoverArtQuality: cfg.Custom.CoverArtQuality,
		PageSize:        cfg.PageSize,
		PlaylistSort:    cfg.PlaylistSort,
		PrivateFeed:     cfg.PrivateFeed,
		UpdatedAt:       time.Now().UTC(),
		ItemURL:         cfg.URL,
	}

	var archives []Archive

	switch info.LinkType {
	case model.TypeSeason:
		mid, seasonId := strings.Split(info.ItemID, ":")[0], strings.Split(info.ItemID, ":")[1]
		userResponse, err := b.client.GetUserInfo(mid)
		if err != nil {
			return nil, err
		}
		_feed.Author = userResponse.Data.Card.Name
		seasonArchivesResponse, err := b.client.GetSeasonEpisodesByPage(mid, seasonId, 1, _feed.PageSize)
		if err != nil {
			return nil, err
		}
		_feed.CoverArt = seasonArchivesResponse.Data.Meta.Cover
		_feed.Title = seasonArchivesResponse.Data.Meta.Name
		_feed.Description = seasonArchivesResponse.Data.Meta.Description
		_feed.ItemURL = fmt.Sprintf("https://space.bilibili.com/%s/lists/%s?type=season", mid, seasonId)
		archives = seasonArchivesResponse.Data.Archives[:]
	case model.TypeSeries:
		mid, seriesId := strings.Split(info.ItemID, ":")[0], strings.Split(info.ItemID, ":")[1]
		seriesInfoResponse, err := b.client.GetSeriesInfo(seriesId)
		if err != nil {
			return nil, err
		}
		userResponse, err := b.client.GetUserInfo(mid)
		if err != nil {
			return nil, err
		}
		_feed.Author = userResponse.Data.Card.Name
		_feed.CoverArt = userResponse.Data.Card.Face
		_feed.Title = seriesInfoResponse.Data.Meta.Name
		_feed.Description = seriesInfoResponse.Data.Meta.Description
		_feed.ItemURL = fmt.Sprintf("https://space.bilibili.com/%s/lists/%s?type=series", mid, seriesId)

		seriesArchivesResponse, err := b.client.GetSeriesEpisodesByPage(mid, seriesId, 1, maxSeriesResults)
		if err != nil {
			return nil, err
		}
		archives = seriesArchivesResponse.Data.Archives[:]
	default:
		// case model.TypeUser:
		userResponse, err := b.client.GetUserInfo(info.ItemID)
		if err != nil {
			return nil, err
		}
		_feed.Author = userResponse.Data.Card.Name
		_feed.CoverArt = userResponse.Data.Card.Face
		_feed.Title = userResponse.Data.Card.Name
		_feed.Description = userResponse.Data.Card.Sign
		_feed.ItemURL = fmt.Sprintf("https://space.bilibili.com/%s", info.ItemID)

		// query video collection
		userEpisodesResponse, err := b.client.GetUserEpisodesByPage(info.ItemID, 1, _feed.PageSize)
		if err != nil {
			return nil, err
		}
		archives = userEpisodesResponse.Data.Archives[:]
	}
	var added = 0
	for _, videoInfo := range archives {
		episodeResponse, err := b.client.GetEpisodeInfo(videoInfo.Bvid)
		if err != nil {
			return nil, err
		}
		if episodeResponse.Data.Is_upower_exclusive {
			// skip uPower exclusive videos
			continue
		}
		_feed.Episodes = append(_feed.Episodes, &model.Episode{
			ID:          episodeResponse.Data.Bvid,
			Title:       episodeResponse.Data.Title,
			Description: episodeResponse.Data.Desc,
			Duration:    int64(episodeResponse.Data.Duration),
			Size:        int64(episodeResponse.Data.Duration * 15000), // very rough estimate
			VideoURL:    "https://www.bilibili.com/video/" + episodeResponse.Data.Bvid,
			PubDate:     time.Unix(videoInfo.PubDate, 0),
			Thumbnail:   episodeResponse.Data.Pic,
			Status:      model.EpisodeNew,
		})

		added++

		if added >= _feed.PageSize {
			return _feed, nil
		}
	}
	return _feed, nil
}

func NewBilibiliBuilder() (*BilibiliBuilder, error) {
	return &BilibiliBuilder{
		client: NewAPIClient(),
	}, nil
}
