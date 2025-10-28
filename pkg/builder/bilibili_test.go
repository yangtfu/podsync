package builder

import (
	"fmt"
	"testing"

	"github.com/mxpv/podsync/pkg/feed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBilibili_BuildFeed(t *testing.T) {
	builder, err := NewBilibiliBuilder()
	require.NoError(t, err)

	urls := []string{
		"https://space.bilibili.com/1302298364",
		"https://space.bilibili.com/7458285/lists/1067956?type=series",
		"https://space.bilibili.com/7380321/lists/678635?type=season",
	}

	for _, addr := range urls {
		t.Run(addr, func(t *testing.T) {
			fmt.Print(addr)

			_feed, err := builder.Build(testCtx, &feed.Config{URL: addr})
			require.NoError(t, err)

			assert.NotEmpty(t, _feed.Title)
			assert.NotNil(t, _feed.Description)
			assert.NotEmpty(t, _feed.Author)
			assert.NotEmpty(t, _feed.ItemURL)

			assert.NotZero(t, len(_feed.Episodes))

			for _, item := range _feed.Episodes {
				assert.NotEmpty(t, item.Title)
				assert.NotEmpty(t, item.VideoURL)
				assert.NotZero(t, item.Duration)
				assert.NotEmpty(t, item.Title)
				assert.NotEmpty(t, item.Thumbnail)
			}
		})
	}
}
