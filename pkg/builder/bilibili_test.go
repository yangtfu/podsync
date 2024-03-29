package builder

import (
	"os"
	"testing"

	"github.com/mxpv/podsync/pkg/feed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	biliKey = os.Getenv("BILIBILI_COOKIE")
)

func TestBilibili_BuildFeed(t *testing.T) {
	builder, err := NewBilibiliBuilder("biliKey")
	require.NoError(t, err)

	urls := []string{
		"https://space.bilibili.com/1302298364",
		"https://space.bilibili.com/397490386/channel/seriesdetail?sid=1203833",
	}

	for _, addr := range urls {
		t.Run(addr, func(t *testing.T) {
			feed, err := builder.Build(testCtx, &feed.Config{URL: addr})
			require.NoError(t, err)

			assert.NotEmpty(t, feed.Title)
			// assert.NotEmpty(t, feed.Description)
			assert.NotEmpty(t, feed.Author)
			assert.NotEmpty(t, feed.ItemURL)

			assert.NotZero(t, len(feed.Episodes))

			for _, item := range feed.Episodes {
				assert.NotEmpty(t, item.Title)
				assert.NotEmpty(t, item.VideoURL)
				assert.NotZero(t, item.Duration)
				assert.NotEmpty(t, item.Title)
				assert.NotEmpty(t, item.Thumbnail)
			}
		})
	}
}
