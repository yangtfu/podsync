package builder

import (
	"context"
	// "fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	bilibiliapi "github.com/CuteReimu/bilibili"
	"github.com/mxpv/podsync/pkg/feed"
	"github.com/pkg/errors"

	"github.com/mxpv/podsync/pkg/model"
)

// const (
// maxBilibiliResults = 50
// hdBytesPerSecond        = 350000
// ldBytesPerSecond        = 100000
// lowAudioBytesPerSecond  = 48000 / 8
// highAudioBytesPerSecond = 128000 / 8
// )

type BilibiliBuilder struct {
	client *bilibiliapi.Client
}

func (b *BilibiliBuilder) getVideoInfo(bvid string) (*model.Episode, error) {
	videoInfo, err := bilibiliapi.GetVideoInfoByBvid(bvid)
	if err != nil {
		return nil, err
	}
	e := model.Episode{
		ID:          bvid,
		Title:       videoInfo.Title,
		Description: videoInfo.Desc,
		Duration:    int64(videoInfo.Duration),
		Size:        int64(videoInfo.Duration * 15000),
		VideoURL:    "https://www.bilibili.com/" + bvid,
		PubDate:     time.Unix(videoInfo.Pubdate, 0),
		Thumbnail:   videoInfo.Pic,
		Status:      model.EpisodeNew,
	}
	// fmt.Println(e)
	return &e, nil
}

func (b *BilibiliBuilder) queryFeed(feed *model.Feed, info *model.Info) error {

	mid, err := strconv.Atoi(strings.Split(info.ItemID, ":")[0])
	if err != nil {
		return err
	}

	switch info.LinkType {
	case model.TypeChannel:
		// 查询usercard
		userCard, err := b.client.GetUserCard(mid, false)
		if err != nil {
			return err
		}

		feed.Author = userCard.Card.Name
		feed.CoverArt = userCard.Card.Face
		feed.Title = userCard.Card.Name
		feed.Description = userCard.Card.Sign

		// 查询合集动态
		var archiveList *bilibiliapi.ArchivesList

		sid, err := strconv.Atoi(strings.Split(info.ItemID, ":")[1])
		if err == nil {
			archiveList, err = b.client.GetArchivesList(mid, sid, 1, feed.PageSize, false)
			if err != nil || len(archiveList.Archives) == 0 {
				return err
			}
		}
		feed.PubDate = time.Unix(int64(archiveList.Archives[0].Pubdate), 0)
		// feed.Description = fmt.Sprintf("%s:%s", userCard.Card.Sign, archiveList.Meta.Description)
		added := 0
		for _, item := range archiveList.Archives {
			e, err := b.getVideoInfo(item.Bvid)
			if err != nil {
				return err
			}
			feed.Episodes = append(feed.Episodes, e)
			added++

			if added >= feed.PageSize || added >= archiveList.Page.Total {
				return nil
			}
		}
		return nil
	case model.TypeUser:
		// 查询usercard
		mid, err := strconv.Atoi(info.ItemID)
		var userCard *bilibiliapi.UserCardResult
		if err == nil {
			userCard, err = b.client.GetUserCard(mid, false)
			if err != nil {
				return err
			}
		}

		feed.Author = userCard.Card.Name
		feed.CoverArt = userCard.Card.Face
		feed.Title = userCard.Card.Name
		feed.Description = userCard.Card.Sign
		// 查询用户动态
		var dynamicInfo *bilibiliapi.DynamicInfo
		if err == nil {
			dynamicInfo, err = b.client.GetUserSpaceDynamic(mid, "")
			if err != nil {
				return err
			}
		}

		feed.PubDate = time.Unix(int64(dynamicInfo.Items[0].Modules.ModuleAuthor.PubTs), 0)

		added := 0
		for _, item := range dynamicInfo.Items {
			if item.Basic.CommentType == 1 {
				e, err := b.getVideoInfo(item.Modules.ModuleDynamic.Major.Archive.Bvid)
				if err != nil {
					return err
				}

				feed.Episodes = append(feed.Episodes, e)

				added++
			}
			if added >= feed.PageSize || dynamicInfo.Offset == "" {
				return nil
			} else if item.IdStr == dynamicInfo.Offset {
				dynamicInfo, err = b.client.GetUserSpaceDynamic(mid, dynamicInfo.Offset)
				if err != nil {
					return err
				}
			}
		}

		return nil
	default:
		return errors.New("unsupported link format")
	}
}

func (b *BilibiliBuilder) Build(ctx context.Context, cfg *feed.Config) (*model.Feed, error) {
	info, err := ParseURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	feed := &model.Feed{
		ItemID:    info.ItemID,
		Provider:  info.Provider,
		LinkType:  info.LinkType,
		Format:    cfg.Format,
		Quality:   cfg.Quality,
		PageSize:  cfg.PageSize,
		UpdatedAt: time.Now().UTC(),
		ItemURL:   cfg.URL,
	}

	if feed.PageSize == 0 {
		feed.PageSize = maxYoutubeResults
	}

	// Query general information about feed (title, description, lang, etc)
	if err := b.queryFeed(feed, &info); err != nil {
		return nil, err
	}
	// Round up to page size.
	// if len(feed.Episodes) > feed.PageSize {
	// 	feed.Episodes = feed.Episodes[:feed.PageSize]
	// }

	sort.Slice(feed.Episodes, func(i, j int) bool {
		item1, _ := strconv.Atoi(feed.Episodes[i].Order)
		item2, _ := strconv.Atoi(feed.Episodes[j].Order)
		return item1 < item2
	})

	return feed, nil
}

func NewBilibiliBuilder(cookie string) (*BilibiliBuilder, error) {
	sc := bilibiliapi.New()
	sc.SetCookiesString(cookie)
	sc.SetTimeout(time.Second * 30)

	return &BilibiliBuilder{client: sc}, nil
}
