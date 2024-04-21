package builder

import (
	"context"
	"fmt"

	// "fmt"
	"sort"
	"strconv"
	"time"

	"github.com/CuteReimu/bilibili"
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
	client *bilibili.Client
}

func (b *BilibiliBuilder) getVideoInfo(bvid string) (*model.Episode, error) {
	videoParam := bilibili.VideoParam{Bvid: bvid}
	videoInfo, err := b.client.GetVideoInfo(videoParam)
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
		PubDate:     time.Unix(int64(videoInfo.Pubdate), 0),
		Thumbnail:   videoInfo.Pic,
		Status:      model.EpisodeNew,
	}
	// fmt.Println(e)
	return &e, nil
}

func (b *BilibiliBuilder) queryFeed(feed *model.Feed, info *model.Info) error {

	switch info.LinkType {
	case model.TypeChannel:
		// mid, err := strconv.Atoi(strings.Split(info.ItemID, ":")[0])
		// if err != nil {
		// 	return err
		// }
		// // 查询usercard
		// getUserCardParam := bilibili.GetUserCardParam{Mid: mid, Photo: false}
		// userCard, err := b.client.GetUserCard(getUserCardParam)
		// if err != nil {
		// 	return err
		// }

		// feed.Author = userCard.Card.Name
		// feed.CoverArt = userCard.Card.Face
		// feed.Title = userCard.Card.Name
		// feed.Description = userCard.Card.Sign

		// // 查询合集动态
		// var videoCollectionInfo *bilibili.VideoCollectionInfo

		// sid, err := strconv.Atoi(strings.Split(info.ItemID, ":")[1])
		// getVideoCollectionInfoParam := bilibili.GetVideoCollectionInfoParam{
		// 	Mid:      mid,
		// 	SeriesId: sid,
		// 	Pn:       1,
		// 	Ps:       feed.PageSize,
		// }
		// if err == nil {
		// 	videoCollectionInfo, err = b.client.GetVideoCollectionInfo(getVideoCollectionInfoParam)
		// 	if err != nil || len(videoCollectionInfo.Archives) == 0 {
		// 		return err
		// 	}
		// }
		// feed.PubDate = time.Unix(int64(videoCollectionInfo.Archives[0].Pubdate), 0)
		// // feed.Description = fmt.Sprintf("%s:%s", userCard.Card.Sign, archiveList.Meta.Description)
		// added := 0
		// for _, item := range videoCollectionInfo.Archives {
		// 	e, err := b.getVideoInfo(item.Bvid)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	feed.Episodes = append(feed.Episodes, e)
		// 	added++

		// 	if added >= feed.PageSize || added >= videoCollectionInfo.Page.Total {
		// 		return nil
		// 	}
		// }
		return nil
	case model.TypeUser:
		// 查询usercard
		mid, err := strconv.Atoi(info.ItemID)
		var userCard *bilibili.UserCard
		userCardParam := bilibili.GetUserCardParam{Mid: mid, Photo: false}
		if err == nil {
			userCard, err = b.client.GetUserCard(userCardParam)
			if err != nil {
				return err
			}
		}

		feed.Author = userCard.Card.Name
		feed.CoverArt = userCard.Card.Face
		feed.Title = userCard.Card.Name
		feed.Description = userCard.Card.Sign
		// 查询用户动态
		var dynamicInfo *bilibili.DynamicInfo
		getUserSpaceDynamicParam := bilibili.GetUserSpaceDynamicParam{HostMid: info.ItemID}
		if err == nil {
			dynamicInfo, err = b.client.GetUserSpaceDynamic(getUserSpaceDynamicParam)
			if err != nil {
				return err
			}
		}

		feed.PubDate = time.Unix(int64(dynamicInfo.Items[0].Modules.ModuleAuthor.PubTs), 0)

		added := 0
		for dynamicInfo.Offset != "" {
			for _, item := range dynamicInfo.Items {
				if item.Basic.CommentType == 1 {
					e, err := b.getVideoInfo(item.Modules.ModuleDynamic.Major.Archive.Bvid)
					if err != nil {
						return err
					}

					feed.Episodes = append(feed.Episodes, e)

					added++
				}
				if added >= feed.PageSize {
					return nil
				} else if item.IdStr.(string) == dynamicInfo.Offset {
					getUserSpaceDynamicParam.Offset = item.IdStr.(string)
					dynamicInfo, err = b.client.GetUserSpaceDynamic(getUserSpaceDynamicParam)
					if err != nil {
						return err
					}
				}
				fmt.Println("IdStr", item.IdStr.(string))
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
	sc := bilibili.New()
	sc.SetCookiesString(cookie)

	return &BilibiliBuilder{client: sc}, nil
}
