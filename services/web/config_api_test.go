package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFeedConfigBlock(t *testing.T) {
	path := writeTestConfig(t, `
[server]
data_dir = "/data"

[feeds]
  [feeds.A]
  url = "https://example.com/a"

  [feeds.B]
  url = "https://example.com/b"
`)

	block, err := readFeedConfigBlock(path, "A")
	require.NoError(t, err)

	assert.Contains(t, block, "[feeds.A]")
	assert.Contains(t, block, `url = "https://example.com/a"`)
	assert.NotContains(t, block, "[feeds.B]")
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0600))
	return path
}
